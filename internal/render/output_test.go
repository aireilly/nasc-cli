// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aireilly/nasc-cli/internal/validate"
)

func TestDetectFlagWinsOverConfigDefault(t *testing.T) {
	var buf bytes.Buffer
	if got := Detect(&buf, "table", "jsonl"); got != FormatTable {
		t.Fatalf("expected FormatTable when flag wins, got %v", got)
	}
}

func TestDetectUsesConfigDefaultWhenNoFlag(t *testing.T) {
	var buf bytes.Buffer
	if got := Detect(&buf, "", "table"); got != FormatTable {
		t.Fatalf("expected FormatTable from config default, got %v", got)
	}
}

func TestDetectFallsBackToJSONLOnNonTTY(t *testing.T) {
	var buf bytes.Buffer
	if got := Detect(&buf, "", ""); got != FormatJSONL {
		t.Fatalf("expected FormatJSONL for non-TTY writer with no config default, got %v", got)
	}
}

func sampleFindings() []validate.Finding {
	return []validate.Finding{
		{Path: "a.md", Field: "title", Rule: "required", Severity: "error", Message: `required field "title" is missing`},
		{Path: "a.md", Field: "description", Rule: "min_length", Severity: "warn", Message: `field "description" is shorter than 30`},
		{Path: "b.md", Field: "type", Rule: "enum", Severity: "error", Message: `field "type" value "x" is not in the allowed set`},
	}
}

func TestFindingsJSONL(t *testing.T) {
	var buf bytes.Buffer
	if err := Findings(&buf, sampleFindings(), FormatJSONL); err != nil {
		t.Fatalf("Findings: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	for _, line := range lines {
		var f validate.Finding
		if err := json.Unmarshal([]byte(line), &f); err != nil {
			t.Fatalf("line not valid JSON: %v: %q", err, line)
		}
	}
}

func TestFindingsPathsDedupesInOrder(t *testing.T) {
	fs := []validate.Finding{
		{Path: "a.md"},
		{Path: "b.md"},
		{Path: "a.md"},
		{Path: "c.md"},
	}
	var buf bytes.Buffer
	if err := Findings(&buf, fs, FormatPaths); err != nil {
		t.Fatalf("Findings: %v", err)
	}
	got := strings.TrimRight(buf.String(), "\n")
	want := "a.md\nb.md\nc.md"
	if got != want {
		t.Fatalf("paths output = %q, want %q", got, want)
	}
}

func TestFindingsTableContainsKnownField(t *testing.T) {
	var buf bytes.Buffer
	if err := Findings(&buf, sampleFindings(), FormatTable); err != nil {
		t.Fatalf("Findings: %v", err)
	}
	out := buf.String()
	if out == "" || !strings.Contains(out, "title") {
		t.Fatalf("expected non-empty table containing field name, got %q", out)
	}
}

func TestFindingsJSONContainsKnownField(t *testing.T) {
	var buf bytes.Buffer
	if err := Findings(&buf, sampleFindings(), FormatJSON); err != nil {
		t.Fatalf("Findings: %v", err)
	}
	var got []validate.Finding
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON array: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 findings decoded, got %d", len(got))
	}
}
