// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package edit

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/parse"
	"gopkg.in/yaml.v3"
)

func TestSetAppendsKeyAndRecordsDerived(t *testing.T) {
	orig := []byte("---\ntitle: Auth\n---\nbody text\n")
	out, err := Set(orig, map[string]model.Value{
		"type": {Kind: model.KindString, Str: "architecture"},
	}, []string{"type"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "type: architecture") {
		t.Fatalf("type not written: %q", s)
	}
	if !strings.Contains(s, "x-nasc-derived") {
		t.Fatalf("provenance not recorded: %q", s)
	}
	if !strings.HasSuffix(s, "body text\n") {
		t.Fatalf("body changed: %q", s)
	}
	if !strings.Contains(s, "title: Auth") {
		t.Fatalf("existing key lost: %q", s)
	}
}

func TestSetWritesDateUnquotedSoItRoundTripsAsDate(t *testing.T) {
	orig := []byte("---\ntitle: Auth\n---\nbody\n")
	out, err := Set(orig, map[string]model.Value{
		"lastUpdated": {Kind: model.KindDate, Str: "2026-02-26"},
	}, []string{"lastUpdated"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"2026-02-26"`) {
		t.Fatalf("date written quoted; it will re-parse as string: %q", out)
	}
	doc := parse.File("x.md", out)
	v, ok := doc.Field("lastUpdated")
	if !ok {
		t.Fatalf("lastUpdated missing: %q", out)
	}
	if v.Kind != model.KindDate {
		t.Fatalf("lastUpdated kind = %d, want KindDate (%d); out=%q", v.Kind, model.KindDate, out)
	}
}

func TestSetInsertsBlockWhenNoFrontmatter(t *testing.T) {
	orig := []byte("# Heading\n\nbody\n")
	out, err := Set(orig, map[string]model.Value{
		"title": {Kind: model.KindString, Str: "Heading"},
	}, []string{"title"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.HasPrefix(s, "---\n") {
		t.Fatalf("no frontmatter block inserted: %q", s)
	}
	if !strings.Contains(s, "# Heading") {
		t.Fatalf("body lost: %q", s)
	}
}

func TestSetPreservesCRLF(t *testing.T) {
	orig := []byte("---\r\ntitle: t\r\n---\r\nbody\r\n")
	out, err := Set(orig, map[string]model.Value{
		"type": {Kind: model.KindString, Str: "runbook"},
	}, []string{"type"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\r\n") {
		t.Fatalf("CRLF not preserved: %q", string(out))
	}
}

// TestSetKeepsBareLFBodyLineInCRLFFrontmatterFile is a regression test for a
// bug where a real edit blanket-replaced every "\n" in the whole output
// (including the body) with "\r\n" whenever the file contained CRLF
// anywhere. A body line that was already bare LF before the edit must stay
// bare LF after it: only the freshly-encoded frontmatter may gain CRLF.
func TestSetKeepsBareLFBodyLineInCRLFFrontmatterFile(t *testing.T) {
	orig := []byte("---\r\ntitle: t\r\n---\r\nline1\nline2\r\n")
	out, err := Set(orig, map[string]model.Value{
		"type": {Kind: model.KindString, Str: "runbook"},
	}, []string{"type"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "line1\nline2\r\n") {
		t.Fatalf("body line endings were rewritten, body no longer matches original bytes: %q", s)
	}
}

// TestSetListItemWithEmbeddedCommaSpaceRoundTrips is a regression test for a
// bug where list values were rebuilt via strings.Split(v.String(), ", "),
// which incorrectly splits a single item containing the literal substring
// ", " into multiple items.
func TestSetListItemWithEmbeddedCommaSpaceRoundTrips(t *testing.T) {
	orig := []byte("---\ntitle: t\n---\nbody\n")
	out, err := Set(orig, map[string]model.Value{
		"tags": {Kind: model.KindList, Str: `["hello, world"]`},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fm, _, _, _, ok, _ := parse.SplitFrontmatter(out)
	if !ok {
		t.Fatalf("frontmatter not found in output: %q", out)
	}
	var doc struct {
		Tags []string `yaml:"tags"`
	}
	if err := yaml.Unmarshal(fm, &doc); err != nil {
		t.Fatalf("unmarshal output frontmatter: %v", err)
	}
	if len(doc.Tags) != 1 || doc.Tags[0] != "hello, world" {
		t.Fatalf("tags = %#v, want single item %q", doc.Tags, "hello, world")
	}
}

// TestSetDerivedOnlyWritesDerivedBlock is a regression test for a bug where
// a call with empty updates but non-empty derived against a file with
// parseable frontmatter was silently dropped (returned unchanged) instead
// of writing the x-nasc-derived block.
func TestSetDerivedOnlyWritesDerivedBlock(t *testing.T) {
	orig := []byte("---\ntitle: t\n---\nbody\n")
	out, err := Set(orig, map[string]model.Value{}, []string{"type"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(out, orig) {
		t.Fatalf("derived-only write was dropped: output unchanged")
	}
	if !strings.Contains(string(out), "x-nasc-derived") {
		t.Fatalf("derived block missing: %q", out)
	}
	if !strings.Contains(string(out), "type") {
		t.Fatalf("derived key missing: %q", out)
	}
}
