// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

func schemaFor(t *testing.T) *schema.Schema {
	t.Helper()
	s, _ := schema.Parse([]byte(`
version: 1
fields:
  id: {type: string, derive: slug}
  title: {type: string, derive: h1}
  type: {type: string, derive: parent-dir}
`))
	return s
}

func TestFileSourceFillsAbsentOnly(t *testing.T) {
	d := model.Doc{
		Path:     "docs/auth.md",
		Title:    "Auth flow",
		Type:     "docs",
		Headings: []model.Heading{{Level: 1, Text: "Auth flow"}},
		Fields:   map[string]model.Value{"title": {Kind: model.KindString, Str: "Human title"}},
	}
	up := FileSource(d, false)
	if _, ok := up["title"]; ok {
		t.Fatalf("title is human-set and must not be overwritten")
	}
	if up["id"].Str != "docs-auth" && up["id"].Str != "auth" {
		t.Fatalf("id = %q", up["id"].Str)
	}
	if up["type"].Str != "docs" {
		t.Fatalf("type = %q", up["type"].Str)
	}
}

// Any present field is left alone without force, even when the derived value has
// moved on: here the H1 changed but the existing title is preserved.
func TestPlanPreservesHumanFields(t *testing.T) {
	d := model.Doc{
		Path: "docs/auth.md", Type: "docs",
		Headings: []model.Heading{{Level: 1, Text: "New title"}},
		Fields:   map[string]model.Value{"title": {Kind: model.KindString, Str: "Human title"}},
	}
	results := Plan([]model.Doc{d}, schemaFor(t), ".", []string{"file"}, false)
	for _, r := range results {
		if _, ok := r.Updates["title"]; ok {
			t.Fatalf("human-set title must be preserved, got update %q", r.Updates["title"].Str)
		}
	}
}

// force overwrites even a human-set field.
func TestPlanForceOverwritesHumanFields(t *testing.T) {
	d := model.Doc{
		Path: "docs/auth.md", Type: "docs",
		Headings: []model.Heading{{Level: 1, Text: "New title"}},
		Fields:   map[string]model.Value{"title": {Kind: model.KindString, Str: "Human title"}},
	}
	results := Plan([]model.Doc{d}, schemaFor(t), ".", []string{"file"}, true)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0].Updates["title"].Str != "New title" {
		t.Fatalf("force should overwrite title, got %q", results[0].Updates["title"].Str)
	}
}

func TestPlanSkipsDocsWithNothingToDo(t *testing.T) {
	d := model.Doc{
		Path:   "docs/a.md",
		Fields: map[string]model.Value{"id": {Kind: model.KindString, Str: "a"}, "title": {Kind: model.KindString, Str: "A"}, "type": {Kind: model.KindString, Str: "docs"}},
		Title:  "A", Type: "docs",
	}
	results := Plan([]model.Doc{d}, schemaFor(t), ".", []string{"file"}, false)
	if len(results) != 0 {
		t.Fatalf("expected no results, got %v", results)
	}
}

// Plan must derive into a copy, never the caller's Fields map. This locks in the
// cloneFields guard: a doc whose fields are all absent gains updates in the
// Result, while the caller's own Fields map stays untouched.
func TestPlanDoesNotMutateCallerFields(t *testing.T) {
	fields := map[string]model.Value{}
	d := model.Doc{Path: "docs/auth.md", Title: "Auth flow", Type: "docs", Fields: fields}

	results := Plan([]model.Doc{d}, schemaFor(t), ".", []string{"file"}, false)
	if len(results) == 0 {
		t.Fatal("expected the doc to gain fields")
	}
	if len(fields) != 0 {
		t.Fatalf("caller Fields map was mutated: %v", fields)
	}
	if len(d.Fields) != 0 {
		t.Fatalf("caller doc Fields was mutated: %v", d.Fields)
	}
}

// A doc with a nil Fields map must not panic: cloneFields handles nil and the
// merged view is written into the freshly cloned map.
func TestPlanHandlesNilFields(t *testing.T) {
	d := model.Doc{Path: "docs/auth.md", Title: "Auth flow", Type: "docs", Fields: nil}
	results := Plan([]model.Doc{d}, schemaFor(t), ".", []string{"file"}, false)
	if len(results) == 0 {
		t.Fatal("expected results for a doc with nil Fields")
	}
}
