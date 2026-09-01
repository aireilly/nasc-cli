// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"path"
	"regexp"
	"strings"

	"github.com/aireilly/nasc-cli/internal/gitmeta"
	"github.com/aireilly/nasc-cli/internal/model"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// shouldSet reports whether a derivable field should be (re)written. A field is
// set when it is absent, when nasc owns it and the tier refreshes its own
// values (deterministic tiers), or when force overrides a human-set value.
// owned is the set of keys nasc previously wrote, from x-nasc-generated.
func shouldSet(d model.Doc, key string, owned map[string]bool, force, refreshOwned bool) bool {
	if _, present := d.Field(key); !present {
		return true
	}
	if refreshOwned && owned[key] {
		return true
	}
	return force
}

// FileTier derives id, title, and type from the path and headings. Absent keys
// are filled; keys nasc owns are refreshed; human-set keys are left alone unless
// force is set. Values come from the raw sources (path slug, H1 heading, parent
// dir), not from d.Title/d.Type, which already prefer the frontmatter value and
// so could never refresh or be overwritten.
func FileTier(d model.Doc, owned map[string]bool, force bool) map[string]model.Value {
	up := map[string]model.Value{}
	if shouldSet(d, "id", owned, force, true) {
		up["id"] = model.Value{Kind: model.KindString, Str: slug(d.Path)}
	}
	if h1 := firstH1(d); h1 != "" && shouldSet(d, "title", owned, force, true) {
		up["title"] = model.Value{Kind: model.KindString, Str: h1}
	}
	if pt := parentDirType(d.Path); pt != "" && shouldSet(d, "type", owned, force, true) {
		up["type"] = model.Value{Kind: model.KindString, Str: pt}
	}
	return up
}

// firstH1 returns the text of the document's first level-1 heading, or "".
func firstH1(d model.Doc) string {
	for _, h := range d.Headings {
		if h.Level == 1 {
			return h.Text
		}
	}
	return ""
}

// parentDirType returns the immediate parent directory name, or "" for a
// top-level file.
func parentDirType(p string) string {
	dir := path.Dir(p)
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	return path.Base(dir)
}

// GitTier derives lastUpdated and owner from git history, refreshing keys nasc
// owns and honouring force for human-set ones.
func GitTier(d model.Doc, root string, owned map[string]bool, force bool) map[string]model.Value {
	up := map[string]model.Value{}
	if !gitmeta.Available(root) {
		return up
	}
	if shouldSet(d, "lastUpdated", owned, force, true) {
		if iso, err := gitmeta.LastUpdated(root, d.Path); err == nil && iso != "" {
			up["lastUpdated"] = model.Value{Kind: model.KindDate, Str: iso}
		}
	}
	if shouldSet(d, "owner", owned, force, true) {
		if o, err := gitmeta.Owner(root, d.Path); err == nil && o != "" {
			up["owner"] = model.Value{Kind: model.KindString, Str: o}
		}
	}
	return up
}

func slug(p string) string {
	base := strings.TrimSuffix(path.Base(p), path.Ext(p))
	s := nonSlug.ReplaceAllString(strings.ToLower(base), "-")
	return strings.Trim(s, "-")
}
