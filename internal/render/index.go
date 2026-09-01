// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package render

import (
	_ "embed"
	"io"
	"sort"
	"text/template"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

//go:embed templates/agents-index.tmpl
var defaultIndexTmpl string

// Group is one type bucket of documents.
type Group struct {
	Type string
	Docs []model.Doc
}

// IndexData is the template input.
type IndexData struct {
	Groups   []Group
	Unmarked []model.Doc
}

// BuildIndex groups marked docs by type and collects the unmarked ones.
func BuildIndex(docs []model.Doc, s *schema.Schema) IndexData {
	byType := map[string][]model.Doc{}
	var unmarked []model.Doc
	for _, d := range docs {
		if s != nil && missingRequired(d, s) {
			unmarked = append(unmarked, d)
			continue
		}
		t := d.Type
		if t == "" {
			t = "other"
		}
		byType[t] = append(byType[t], d)
	}
	var groups []Group
	for t, ds := range byType {
		sort.Slice(ds, func(i, j int) bool { return ds[i].Title < ds[j].Title })
		groups = append(groups, Group{Type: t, Docs: ds})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Type < groups[j].Type })
	sort.Slice(unmarked, func(i, j int) bool { return unmarked[i].Path < unmarked[j].Path })
	return IndexData{Groups: groups, Unmarked: unmarked}
}

func missingRequired(d model.Doc, s *schema.Schema) bool {
	for name, f := range s.Fields {
		if !f.Required {
			continue
		}
		if v, ok := d.Field(name); !ok || v.IsZero() {
			return true
		}
	}
	return false
}

// Index renders the index. An empty tmpl uses the embedded default.
func Index(w io.Writer, data IndexData, tmpl string) error {
	text := tmpl
	if text == "" {
		text = defaultIndexTmpl
	}
	funcs := template.FuncMap{
		"fieldOr": func(d model.Doc, key, fallback string) string {
			if v, ok := d.Field(key); ok && !v.IsZero() {
				return v.String()
			}
			return fallback
		},
	}
	t, err := template.New("index").Funcs(funcs).Parse(text)
	if err != nil {
		return err
	}
	return t.Execute(w, data)
}
