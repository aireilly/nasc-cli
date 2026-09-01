// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package parse

import (
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
)

func TestParseFullDoc(t *testing.T) {
	src := "---\ntitle: Auth flow\ntype: architecture\ntags: [auth, security]\n---\n# Heading\n\nBody with #inline tag.\n"
	d := File("docs/auth.md", []byte(src))
	if d.Title != "Auth flow" {
		t.Fatalf("title = %q", d.Title)
	}
	if d.Type != "architecture" {
		t.Fatalf("type = %q", d.Type)
	}
	if v, _ := d.Field("tags"); v.Kind != model.KindList {
		t.Fatalf("tags kind = %v", v.Kind)
	}
	if !contains(d.Tags, "auth") || !contains(d.Tags, "inline") {
		t.Fatalf("tags = %v, want auth and inline", d.Tags)
	}
}

func TestTitleFallsBackToH1(t *testing.T) {
	src := "# The Title\n\ntext\n"
	d := File("docs/x.md", []byte(src))
	if d.Title != "The Title" {
		t.Fatalf("title = %q", d.Title)
	}
	if d.FMStart != 0 || d.FMEnd != 0 {
		t.Fatalf("expected no frontmatter offsets, got %d,%d", d.FMStart, d.FMEnd)
	}
}

func TestTypeFallsBackToParentDir(t *testing.T) {
	d := File("runbooks/incident.md", []byte("# Incident\n"))
	if d.Type != "runbooks" {
		t.Fatalf("type = %q, want runbooks", d.Type)
	}
}

func TestUnclosedFrontmatterWarns(t *testing.T) {
	src := "---\ntitle: broken\n# no closing fence\nbody\n"
	d := File("docs/bad.md", []byte(src))
	if len(d.Warnings) == 0 {
		t.Fatalf("expected a warning for unclosed frontmatter")
	}
	if _, ok := d.Field("title"); ok {
		t.Fatalf("unclosed frontmatter should yield no fields")
	}
}

func TestDateCoercion(t *testing.T) {
	d := File("x.md", []byte("---\nlastUpdated: 2026-08-31\nquoted: \"2026-08-31\"\n---\n"))
	if v, _ := d.Field("lastUpdated"); v.Kind != model.KindDate {
		t.Fatalf("lastUpdated kind = %v, want Date", v.Kind)
	}
	if v, _ := d.Field("quoted"); v.Kind != model.KindString {
		t.Fatalf("quoted kind = %v, want String", v.Kind)
	}
}

func TestCRLFDetected(t *testing.T) {
	d := File("x.md", []byte("---\r\ntitle: t\r\n---\r\nbody\r\n"))
	if !d.CRLF {
		t.Fatalf("expected CRLF detected")
	}
}

func TestTagInCodeFenceIgnored(t *testing.T) {
	src := "# T\n\n```\n#notatag\n```\n\n#realtag\n"
	d := File("x.md", []byte(src))
	if contains(d.Tags, "notatag") {
		t.Fatalf("tag inside code fence should be ignored")
	}
	if !contains(d.Tags, "realtag") {
		t.Fatalf("realtag missing, got %v", d.Tags)
	}
}

func TestBoolCaseInsensitive(t *testing.T) {
	src := "---\nflag: True\nother: false\n---\n"
	d := File("x.md", []byte(src))
	if v, _ := d.Field("flag"); v.Kind != model.KindBool || v.Num != 1 {
		t.Fatalf("flag: True should be bool true (1), got kind=%v num=%v", v.Kind, v.Num)
	}
	if v, _ := d.Field("other"); v.Kind != model.KindBool || v.Num != 0 {
		t.Fatalf("other: false should be bool false (0), got kind=%v num=%v", v.Kind, v.Num)
	}
}

func TestFrontmatterByteOffsets(t *testing.T) {
	// Construct a small document with known byte offsets
	// "---\na: 1\n---\nBody text\n"
	// Bytes 0-2: "---"
	// Byte 3: "\n"
	// Bytes 4-8: "a: 1\n"
	// Bytes 9-11: "---"
	// Byte 12: "\n"
	// Bytes 13+: "Body text\n"
	src := "---\na: 1\n---\nBody text\n"
	d := File("test.md", []byte(src))

	// FMStart should be 0 (start of opening fence)
	// FMEnd should be 13 (byte after closing fence line, start of body)
	// Body should start at byte 13
	if d.FMStart != 0 {
		t.Fatalf("FMStart = %d, want 0", d.FMStart)
	}
	if d.FMEnd != 13 {
		t.Fatalf("FMEnd = %d, want 13", d.FMEnd)
	}
	if len(d.Body) == 0 || d.Body[0] != 'B' {
		t.Fatalf("body should start with 'B', got %q", d.Body)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
