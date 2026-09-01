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

// FileTier derives id, title, and type from the path and headings. It returns
// only keys the doc does not already have.
func FileTier(d model.Doc) map[string]model.Value {
	up := map[string]model.Value{}
	if _, ok := d.Field("id"); !ok {
		up["id"] = model.Value{Kind: model.KindString, Str: slug(d.Path)}
	}
	if _, ok := d.Field("title"); !ok && d.Title != "" {
		up["title"] = model.Value{Kind: model.KindString, Str: d.Title}
	}
	if _, ok := d.Field("type"); !ok && d.Type != "" {
		up["type"] = model.Value{Kind: model.KindString, Str: d.Type}
	}
	return up
}

// GitTier derives lastUpdated and owner from git history.
func GitTier(d model.Doc, root string) map[string]model.Value {
	up := map[string]model.Value{}
	if !gitmeta.Available(root) {
		return up
	}
	if _, ok := d.Field("lastUpdated"); !ok {
		if iso, err := gitmeta.LastUpdated(root, d.Path); err == nil && iso != "" {
			up["lastUpdated"] = model.Value{Kind: model.KindDate, Str: iso}
		}
	}
	if _, ok := d.Field("owner"); !ok {
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
