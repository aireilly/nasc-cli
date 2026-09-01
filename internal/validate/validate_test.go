// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package validate

import (
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

func testSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Parse([]byte(`
version: 1
fields:
  title: {type: string, required: true}
  type: {type: string, required: true, enum: [architecture, runbook]}
  description: {type: string, required: true, min_length: 30}
rules:
  - name: desc-trigger
    when: exists(description)
    assert: length(description) >= 30
    severity: error
    message: description too short to be a trigger
`))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMissingRequiredField(t *testing.T) {
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{}}
	fs := Check([]model.Doc{d}, testSchema(t), ".")
	if !hasFinding(fs, "title", "required") {
		t.Fatalf("expected required-title finding, got %v", fs)
	}
}

func TestEnumViolation(t *testing.T) {
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"title":       {Kind: model.KindString, Str: "T"},
		"type":        {Kind: model.KindString, Str: "concept"},
		"description": {Kind: model.KindString, Str: "Load when editing the widget subsystem here."},
	}}
	fs := Check([]model.Doc{d}, testSchema(t), ".")
	if !hasFinding(fs, "type", "enum") {
		t.Fatalf("expected enum finding, got %v", fs)
	}
}

func TestCustomRuleFires(t *testing.T) {
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"title":       {Kind: model.KindString, Str: "T"},
		"type":        {Kind: model.KindString, Str: "runbook"},
		"description": {Kind: model.KindString, Str: "short"},
	}}
	fs := Check([]model.Doc{d}, testSchema(t), ".")
	if !hasFinding(fs, "description", "desc-trigger") {
		t.Fatalf("expected desc-trigger finding, got %v", fs)
	}
}

func TestCleanDocHasNoFindings(t *testing.T) {
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"title":       {Kind: model.KindString, Str: "T"},
		"type":        {Kind: model.KindString, Str: "runbook"},
		"description": {Kind: model.KindString, Str: "Load when responding to a production incident."},
	}}
	if fs := Check([]model.Doc{d}, testSchema(t), "."); len(fs) != 0 {
		t.Fatalf("expected no findings, got %v", fs)
	}
}

func TestWhenClauseErrorSurfacesFinding(t *testing.T) {
	s, err := schema.Parse([]byte(`
version: 1
fields:
  title: {type: string, required: true}
rules:
  - name: bad-when
    when: contains(title)
    assert: length(title) > 0
    severity: error
    message: bad when clause
`))
	if err != nil {
		t.Fatal(err)
	}
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"title": {Kind: model.KindString, Str: "Hello"},
	}}
	fs := Check([]model.Doc{d}, s, ".")
	found := false
	for _, f := range fs {
		if f.Rule == "bad-when" && f.Severity == "error" && contains(f.Message, "when-clause error") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected when-clause error finding for rule bad-when, got %v", fs)
	}
}

func TestWarnSeverityFindingProduced(t *testing.T) {
	s, err := schema.Parse([]byte(`
version: 1
fields:
  title: {type: string, required: true}
rules:
  - name: title-length
    when: exists(title)
    assert: length(title) >= 10
    severity: warn
    message: title is quite short
`))
	if err != nil {
		t.Fatal(err)
	}
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"title": {Kind: model.KindString, Str: "Hi"},
	}}
	fs := Check([]model.Doc{d}, s, ".")
	if !hasFinding(fs, "title", "title-length") {
		t.Fatalf("expected warn finding for title-length, got %v", fs)
	}
	for _, f := range fs {
		if f.Rule == "title-length" && f.Severity != "warn" {
			t.Fatalf("expected warn severity, got %q", f.Severity)
		}
	}
}

func TestTypeMismatchFinding(t *testing.T) {
	s, err := schema.Parse([]byte(`
version: 1
fields:
  count: {type: number, required: true}
`))
	if err != nil {
		t.Fatal(err)
	}
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"count": {Kind: model.KindString, Str: "not-a-number"},
	}}
	fs := Check([]model.Doc{d}, s, ".")
	if !hasFinding(fs, "count", "type") {
		t.Fatalf("expected type finding, got %v", fs)
	}
}

func TestMinLengthViolation(t *testing.T) {
	s, err := schema.Parse([]byte(`
version: 1
fields:
  summary: {type: string, required: true, min_length: 10}
`))
	if err != nil {
		t.Fatal(err)
	}
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"summary": {Kind: model.KindString, Str: "short"},
	}}
	fs := Check([]model.Doc{d}, s, ".")
	if !hasFinding(fs, "summary", "min_length") {
		t.Fatalf("expected min_length finding, got %v", fs)
	}
}

func TestMaxLengthViolation(t *testing.T) {
	s, err := schema.Parse([]byte(`
version: 1
fields:
  summary: {type: string, required: true, max_length: 5}
`))
	if err != nil {
		t.Fatal(err)
	}
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"summary": {Kind: model.KindString, Str: "way too long a value"},
	}}
	fs := Check([]model.Doc{d}, s, ".")
	if !hasFinding(fs, "summary", "max_length") {
		t.Fatalf("expected max_length finding, got %v", fs)
	}
}

func TestTypeMismatchSkipsFurtherFieldChecks(t *testing.T) {
	s, err := schema.Parse([]byte(`
version: 1
fields:
  status: {type: number, required: true, enum: ["1", "2"]}
`))
	if err != nil {
		t.Fatal(err)
	}
	d := model.Doc{Path: "a.md", Fields: map[string]model.Value{
		"status": {Kind: model.KindString, Str: "notanumber"},
	}}
	fs := Check([]model.Doc{d}, s, ".")
	count := 0
	for _, f := range fs {
		if f.Field == "status" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 finding for status field, got %d: %v", count, fs)
	}
}

func hasFinding(fs []Finding, field, ruleOrKind string) bool {
	for _, f := range fs {
		if f.Field == field && (f.Rule == ruleOrKind || contains(f.Message, ruleOrKind)) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
