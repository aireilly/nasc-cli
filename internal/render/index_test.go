// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

func TestBuildAndRenderIndex(t *testing.T) {
	s, _ := schema.Parse([]byte("version: 1\nfields:\n  title: {type: string, required: true}\n  description: {type: string, required: true}\n"))
	docs := []model.Doc{
		{Path: "docs/auth.md", Type: "architecture", Title: "Auth flow",
			Fields: map[string]model.Value{
				"title":       {Kind: model.KindString, Str: "Auth flow"},
				"description": {Kind: model.KindString, Str: "Load when touching auth."},
			}},
		{Path: "docs/raw.md", Type: "architecture", Title: "Raw",
			Fields: map[string]model.Value{"title": {Kind: model.KindString, Str: "Raw"}}},
	}
	data := BuildIndex(docs, s)
	var buf bytes.Buffer
	if err := Index(&buf, data, ""); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "## architecture") {
		t.Fatalf("missing group header:\n%s", out)
	}
	if !strings.Contains(out, "Load when touching auth.") {
		t.Fatalf("missing description trigger:\n%s", out)
	}
	if !strings.Contains(out, "## Unmarked") || !strings.Contains(out, "docs/raw.md") {
		t.Fatalf("missing unmarked section:\n%s", out)
	}
}

func TestDisplayTitleFallsBackToPath(t *testing.T) {
	cases := []struct {
		doc  model.Doc
		want string
	}{
		{model.Doc{Path: "docs/auth.md", Title: "Auth flow"}, "Auth flow"},
		{model.Doc{Path: "RELEASE-NOTES.md"}, "RELEASE-NOTES"},
		{model.Doc{Path: "pkg/epp/README.md"}, "epp"},
		{model.Doc{Path: "docs/scheduling/index.md"}, "scheduling"},
	}
	for _, c := range cases {
		if got := displayTitle(c.doc); got != c.want {
			t.Errorf("displayTitle(%q) = %q, want %q", c.doc.Path, got, c.want)
		}
	}
}

func TestMergeCreatesWhenEmpty(t *testing.T) {
	out := string(Merge(nil, []byte("BODY\n")))
	want := BeginMarker + "\nBODY\n" + EndMarker + "\n"
	if out != want {
		t.Fatalf("Merge(empty) = %q, want %q", out, want)
	}
}

func TestMergeAppendsWhenNoMarkers(t *testing.T) {
	existing := []byte("# My handwritten notes\n\nkeep me\n")
	out := string(Merge(existing, []byte("GEN\n")))
	if !strings.HasPrefix(out, "# My handwritten notes\n\nkeep me\n") {
		t.Fatalf("existing content not preserved:\n%s", out)
	}
	if !strings.Contains(out, BeginMarker+"\nGEN\n"+EndMarker) {
		t.Fatalf("generated block not appended:\n%s", out)
	}
}

func TestMergeReplacesRegionInPlace(t *testing.T) {
	existing := []byte("intro\n\n" + BeginMarker + "\nOLD\n" + EndMarker + "\n\noutro\n")
	out := string(Merge(existing, []byte("NEW\n")))
	if strings.Contains(out, "OLD") {
		t.Fatalf("old region not replaced:\n%s", out)
	}
	if !strings.Contains(out, "intro\n") || !strings.Contains(out, "outro\n") {
		t.Fatalf("surrounding prose lost:\n%s", out)
	}
	if !strings.Contains(out, BeginMarker+"\nNEW\n"+EndMarker) {
		t.Fatalf("new region missing:\n%s", out)
	}
	if strings.Count(out, BeginMarker) != 1 {
		t.Fatalf("region duplicated:\n%s", out)
	}
}
