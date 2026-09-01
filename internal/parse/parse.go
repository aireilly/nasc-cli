// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package parse

import (
	"bytes"
	"path"
	"strings"

	"github.com/aireilly/nasc-cli/internal/model"
)

// File parses one markdown file into a Doc. It never fails; malformed input
// becomes a warning on the returned Doc.
func File(p string, data []byte) model.Doc {
	d := model.Doc{Path: p, Fields: map[string]model.Value{}}
	d.CRLF = bytes.Contains(data, []byte("\r\n"))

	fm, bodyStart, fmStart, fmEnd, ok, warn := SplitFrontmatter(data)
	if warn != "" {
		d.Warnings = append(d.Warnings, warn)
	}
	if ok {
		fields, _, err := decodeFields(fm)
		if err != nil {
			d.Warnings = append(d.Warnings, "frontmatter parse error: "+err.Error())
		} else {
			d.Fields = fields
		}
		d.FMStart, d.FMEnd = fmStart, fmEnd
		d.Body = string(data[bodyStart:])
	} else {
		d.Body = string(data)
	}

	hs, inlineTags := scanBody(d.Body)
	d.Headings = hs

	d.Title = deriveTitle(d)
	d.Type = deriveType(d, p)
	d.Tags = deriveTags(d, inlineTags)
	d.Paths = listField(d, "paths")
	return d
}

func deriveTitle(d model.Doc) string {
	if v, ok := d.Field("title"); ok && !v.IsZero() {
		return v.String()
	}
	for _, h := range d.Headings {
		if h.Level == 1 {
			return h.Text
		}
	}
	return ""
}

func deriveType(d model.Doc, p string) string {
	if v, ok := d.Field("type"); ok && !v.IsZero() {
		return v.String()
	}
	dir := path.Dir(p)
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	return path.Base(dir)
}

func deriveTags(d model.Doc, inline []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range listField(d, "tags") {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, t := range inline {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func listField(d model.Doc, key string) []string {
	v, ok := d.Field(key)
	if !ok {
		return nil
	}
	if v.Kind == model.KindList {
		var parts []string
		for _, p := range strings.Split(v.String(), ", ") {
			if p != "" {
				parts = append(parts, p)
			}
		}
		return parts
	}
	if v.Kind == model.KindString && v.Str != "" {
		return []string{v.Str}
	}
	return nil
}
