// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"path"
	"regexp"
	"strings"

	"github.com/aireilly/nasc-cli/internal/gitmeta"
	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// shouldSet reports whether a derivable field should be written. A field is set
// when it is absent, when the source always refreshes it (a value nasc owns
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

// fieldsDerivedBy returns the schema's field names whose derive is one of
// kinds, in declaration order.
func fieldsDerivedBy(s *schema.Schema, kinds ...string) []string {
	if s == nil {
		return nil
	}
	names := s.FieldOrder
	if len(names) == 0 {
		for n := range s.Fields {
			names = append(names, n)
		}
	}
	var out []string
	for _, n := range names {
		f := s.Fields[n]
		for _, k := range kinds {
			if f.Derive == k {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// FileSource derives fields from the path and headings: `slug`, `h1`, and
// `parent-dir`. Only fields the schema declares with one of those derives are
// produced, keyed by field name. It fills absent values and leaves existing
// ones alone unless force is set. Values come from the raw inputs (path, H1,
// directory), not from d.Title/d.Type, which already prefer the frontmatter
// value and so could never be overwritten under force.
//
// The second return value is the document's directory when a parent-dir field
// could not be placed, so the caller can suggest a map entry; otherwise "".
func FileSource(d model.Doc, s *schema.Schema, force bool) (map[string]model.Value, string) {
	up := map[string]model.Value{}
	unmapped := ""
	for _, name := range fieldsDerivedBy(s, "slug", "h1", "parent-dir") {
		f := s.Fields[name]
		if !shouldSet(d, s.Key(name), force, false) {
			continue
		}
		switch f.Derive {
		case "slug":
			if sl := slug(d.Path); sl != "" {
				up[name] = model.Value{Kind: model.KindString, Str: sl}
			}
		case "h1":
			if h1 := firstH1(d); h1 != "" {
				up[name] = model.Value{Kind: model.KindString, Str: h1}
			}
		case "parent-dir":
			if v, ok := dirValue(d.Path, f); ok {
				up[name] = model.Value{Kind: model.KindString, Str: v}
			} else {
				unmapped = path.Dir(d.Path)
			}
		}
	}
	return up, unmapped
}

// dirValue resolves a parent-dir field for a document path. The field's map is
// checked from the deepest directory up, by full path and then by directory
// name; "." in the map covers top-level files. Without a map hit the immediate
// parent directory name is used, but only when the field has no enum or the
// name is in it.
func dirValue(p string, f schema.Field) (string, bool) {
	if path.Dir(p) == "." {
		if v, ok := f.Map["."]; ok {
			return v, true
		}
	}
	for dir := path.Dir(p); dir != "." && dir != "/" && dir != ""; dir = path.Dir(dir) {
		if v, ok := f.Map[dir]; ok {
			return v, true
		}
		if v, ok := f.Map[path.Base(dir)]; ok {
			return v, true
		}
	}
	raw := parentDirType(p)
	if raw == "" {
		return "", false
	}
	if len(f.Enum) == 0 {
		return raw, true
	}
	for _, e := range f.Enum {
		if e == raw {
			return raw, true
		}
	}
	return "", false
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

// GitSource derives `git-last-change` and `codeowners` fields from git.
// git-last-change always refreshes: it is git's to own and ignores commits
// that only touched frontmatter, so it settles once a body change is dated.
// codeowners fills only when absent, since a project may set the owner by
// hand. force overwrites either. ownerBlame enables the git blame fallback
// when no CODEOWNERS rule matches.
func GitSource(d model.Doc, s *schema.Schema, root string, force, ownerBlame bool) map[string]model.Value {
	up := map[string]model.Value{}
	if !gitmeta.Available(root) {
		return up
	}
	for _, name := range fieldsDerivedBy(s, "git-last-change", "codeowners") {
		key := s.Key(name)
		switch s.Fields[name].Derive {
		case "git-last-change":
			if !shouldSet(d, key, force, true) {
				continue
			}
			if iso, err := gitmeta.LastUpdated(root, d.Path); err == nil && iso != "" {
				up[name] = model.Value{Kind: model.KindDate, Str: iso}
			}
		case "codeowners":
			if !shouldSet(d, key, force, false) {
				continue
			}
			if o, err := gitmeta.Owner(root, d.Path, ownerBlame); err == nil && o != "" {
				up[name] = model.Value{Kind: model.KindString, Str: o}
			}
		}
	}
	return up
}

// slug turns a repo-relative path into an identifier that is unique across
// the corpus: "deploy/environments/dev/README.md" becomes
// "deploy-environments-dev-readme".
func slug(p string) string {
	s := strings.TrimSuffix(p, path.Ext(p))
	s = nonSlug.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}
