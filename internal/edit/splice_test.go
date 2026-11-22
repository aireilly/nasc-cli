// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package edit

import (
	"strings"
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
)

func str(s string) model.Value { return model.Value{Kind: model.KindString, Str: s} }

// Keys that mark does not set keep their exact bytes. This is the case that
// churned 76 lines in a real repository: flow mappings lost their inner
// spaces when the whole block was re-encoded.
func TestSetLeavesUntouchedKeysByteIdentical(t *testing.T) {
	head := `title: "Quoted title"   # a comment
# a comment above sources
sources:
  - { id: "issue-1", resource: "https://example.com/1", author: "human:a" }
verified: { by: "human:b", at: "2026-10-29T00:00:00Z" }
notes: |
  line one
  line two
tags: ["a", "b"]
`
	orig := "---\n" + head + "---\nbody\n"
	out, err := Set([]byte(orig), map[string]model.Value{"owner": str("@team")})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\n" + head + "owner: '@team'\n---\nbody\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// Replacing a key rewrites only that key's lines, including a block
// sequence written at the key's own indent and a multi-line block scalar.
func TestSetReplacesOnlyTheChangedKey(t *testing.T) {
	orig := `---
description: >
  old text
  that wraps
tags:
- x
- y

keep: { a: 1 }
---
body
`
	out, err := Set([]byte(orig), map[string]model.Value{
		"description": str("new"),
		"tags":        model.Value{Kind: model.KindList, Str: `["z"]`},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `---
description: new
tags:
  - z

keep: { a: 1 }
---
body
`
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

// The deprecated provenance key is removed without touching its neighbours.
func TestSetRemovesProvenanceKeyInPlace(t *testing.T) {
	orig := "---\na: { x: 1 }\nx-nasc-generated:\n  by: nasc\nb: 2\n---\n"
	out, err := Set([]byte(orig), map[string]model.Value{"b": str("3")})
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\na: { x: 1 }\nb: \"3\"\n---\n"; string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// Anchors make a line-level edit unsafe, so Set re-encodes the block and
// the result still parses with the update applied.
func TestSetFallsBackWithAnchors(t *testing.T) {
	orig := "---\nbase: &b { x: 1 }\nother: *b\n---\n"
	out, err := Set([]byte(orig), map[string]model.Value{"title": str("T")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "title: T") || !strings.Contains(string(out), "*b") {
		t.Fatalf("got %q", out)
	}
}

// A CRLF frontmatter keeps CRLF on both kept and new lines.
func TestSetSpliceKeepsCRLF(t *testing.T) {
	orig := "---\r\na: { x: 1 }\r\n---\r\nbody\r\n"
	out, err := Set([]byte(orig), map[string]model.Value{"b": str("2")})
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\r\na: { x: 1 }\r\nb: \"2\"\r\n---\r\nbody\r\n"; string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}
