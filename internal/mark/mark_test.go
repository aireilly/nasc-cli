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

func fileOpts() Options { return Options{Root: ".", Sources: []string{"file"}} }

func TestFileSourceFillsAbsentOnly(t *testing.T) {
	d := model.Doc{
		Path:     "docs/auth.md",
		Title:    "Auth flow",
		Type:     "docs",
		Headings: []model.Heading{{Level: 1, Text: "Auth flow"}},
		Fields:   map[string]model.Value{"title": {Kind: model.KindString, Str: "Human title"}},
	}
	up, _ := FileSource(d, schemaFor(t), false)
	if _, ok := up["title"]; ok {
		t.Fatalf("title is human-set and must not be overwritten")
	}
	if up["id"].Str != "docs-auth" {
		t.Fatalf("id = %q, want the full-path slug docs-auth", up["id"].Str)
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
	results := Plan([]model.Doc{d}, schemaFor(t), fileOpts()).Results
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
	opts := fileOpts()
	opts.Force = true
	results := Plan([]model.Doc{d}, schemaFor(t), opts).Results
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
	results := Plan([]model.Doc{d}, schemaFor(t), fileOpts()).Results
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

	results := Plan([]model.Doc{d}, schemaFor(t), fileOpts()).Results
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
	results := Plan([]model.Doc{d}, schemaFor(t), fileOpts()).Results
	if len(results) == 0 {
		t.Fatal("expected results for a doc with nil Fields")
	}
}

func mustSchema(t *testing.T, y string) *schema.Schema {
	t.Helper()
	s, err := schema.Parse([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A parent-dir field with an enum uses the map, then the raw directory name if
// it is allowed, and otherwise leaves the field unset and reports the
// directory, so mark never writes a value validate would reject.
func TestFileSourceParentDirHonoursMapAndEnum(t *testing.T) {
	s := mustSchema(t, `
version: 1
fields:
  type:
    type: string
    derive: parent-dir
    enum: [procedure, reference, runbook]
    map:
      guides: procedure
      deploy/environments: runbook
      ".": reference
`)
	cases := map[string]string{
		"docs/guides/a.md":             "procedure",
		"deploy/environments/dev/x.md": "runbook",
		"README.md":                    "reference",
		"docs/reference/api.md":        "reference",
	}
	for p, want := range cases {
		up, unmapped := FileSource(model.Doc{Path: p}, s, false)
		if up["type"].Str != want || unmapped != "" {
			t.Errorf("%s: type = %q unmapped = %q, want %q", p, up["type"].Str, unmapped, want)
		}
	}
	up, unmapped := FileSource(model.Doc{Path: "hack/dev/notes.md"}, s, false)
	if _, ok := up["type"]; ok || unmapped != "hack/dev" {
		t.Fatalf("unmapped dir: got %v, unmapped %q", up, unmapped)
	}
}

// Only fields the schema declares with a file derive are written.
func TestFileSourceIsSchemaDriven(t *testing.T) {
	s := mustSchema(t, "version: 1\nfields:\n  title: {type: string, derive: h1}\n  type: {type: string}\n")
	d := model.Doc{Path: "docs/a.md", Headings: []model.Heading{{Level: 1, Text: "A"}}}
	up, _ := FileSource(d, s, false)
	if len(up) != 1 || up["title"].Str != "A" {
		t.Fatalf("FileSource = %v, want only title", up)
	}
}

// A renamed field is read and written under its key.
func TestPlanWritesRenamedKeys(t *testing.T) {
	s := mustSchema(t, "version: 1\nfields:\n  type: {type: string, derive: parent-dir, key: nasc_type}\n")
	d := model.Doc{Path: "guides/a.md", Fields: map[string]model.Value{"type": {Kind: model.KindString, Str: "page"}}}
	rep := Plan([]model.Doc{d}, s, fileOpts())
	if len(rep.Results) != 1 {
		t.Fatalf("results = %v", rep.Results)
	}
	up := rep.Results[0].Updates
	if up["nasc_type"].Str != "guides" {
		t.Fatalf("updates = %v, want nasc_type: guides", up)
	}
	if _, ok := up["type"]; ok {
		t.Fatalf("site generator's own type key must not be touched: %v", up)
	}
}

// A key a site generator owns is skipped and reported, unless the schema
// names that key explicitly.
func TestPlanSkipsProtectedKeys(t *testing.T) {
	opts := fileOpts()
	opts.Protected = func(string) map[string]string { return map[string]string{"type": "Hugo uses type to pick a layout"} }
	d := model.Doc{Path: "guides/a.md"}

	rep := Plan([]model.Doc{d}, mustSchema(t, "version: 1\nfields:\n  type: {type: string, derive: parent-dir}\n"), opts)
	if len(rep.Results) != 0 || len(rep.Skipped) != 1 || rep.Skipped[0].Key != "type" {
		t.Fatalf("expected type skipped, got results %v skipped %v", rep.Results, rep.Skipped)
	}

	rep = Plan([]model.Doc{d}, mustSchema(t, "version: 1\nfields:\n  type: {type: string, derive: parent-dir, key: type}\n"), opts)
	if len(rep.Results) != 1 {
		t.Fatalf("explicit key should be written, got skipped %v", rep.Skipped)
	}
}

// Plan keeps input order when it runs documents concurrently.
func TestPlanParallelKeepsOrder(t *testing.T) {
	var docs []model.Doc
	for _, p := range []string{"a/1.md", "b/2.md", "c/3.md", "d/4.md", "e/5.md", "f/6.md"} {
		docs = append(docs, model.Doc{Path: p})
	}
	opts := fileOpts()
	opts.Jobs = 4
	rep := Plan(docs, schemaFor(t), opts)
	if len(rep.Results) != len(docs) {
		t.Fatalf("got %d results", len(rep.Results))
	}
	for i, r := range rep.Results {
		if r.Path != docs[i].Path {
			t.Fatalf("result %d is %s, want %s", i, r.Path, docs[i].Path)
		}
	}
}

func TestSlugIsUniquePerPath(t *testing.T) {
	if a, b := slug("deploy/dev/README.md"), slug("deploy/prod/README.md"); a == b {
		t.Fatalf("slugs collide: %q", a)
	}
	if got := slug("deploy/environments/dev/README.md"); got != "deploy-environments-dev-readme" {
		t.Fatalf("slug = %q", got)
	}
}
