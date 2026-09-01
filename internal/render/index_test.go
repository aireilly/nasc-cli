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
