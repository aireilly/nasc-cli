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

// shouldSet reports whether a derivable field should be written. A field is set
// when it is absent, when the tier always refreshes it (a value nasc owns
// outright, such as the git commit date), or when force overrides a value that
// is already present.
func shouldSet(d model.Doc, key string, force, alwaysRefresh bool) bool {
	if _, present := d.Field(key); !present {
		return true
	}
	if alwaysRefresh {
		return true
	}
	return force
}

// FileTier derives id, title, and type from the path and headings. It fills
// absent keys and leaves existing values alone unless force is set. Values come
// from the raw sources (path slug, H1 heading, parent dir), not from
// d.Title/d.Type, which already prefer the frontmatter value and so could never
// be overwritten under force.
func FileTier(d model.Doc, force bool) map[string]model.Value {
	up := map[string]model.Value{}
	if shouldSet(d, "id", force, false) {
		up["id"] = model.Value{Kind: model.KindString, Str: slug(d.Path)}
	}
	if h1 := firstH1(d); h1 != "" && shouldSet(d, "title", force, false) {
		up["title"] = model.Value{Kind: model.KindString, Str: h1}
	}
	if pt := parentDirType(d.Path); pt != "" && shouldSet(d, "type", force, false) {
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

// GitTier derives lastUpdated and owner from git history. lastUpdated always
// refreshes: it is git's to own and moves on every commit, so a stale value is
// never what you want. owner is filled only when absent, since a project may set
// it by hand. force overwrites either.
func GitTier(d model.Doc, root string, force bool) map[string]model.Value {
	up := map[string]model.Value{}
	if !gitmeta.Available(root) {
		return up
	}
	if shouldSet(d, "lastUpdated", force, true) {
		if iso, err := gitmeta.LastUpdated(root, d.Path); err == nil && iso != "" {
			up["lastUpdated"] = model.Value{Kind: model.KindDate, Str: iso}
		}
	}
	if shouldSet(d, "owner", force, false) {
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
