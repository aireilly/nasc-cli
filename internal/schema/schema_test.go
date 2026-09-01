// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package schema

import "testing"

const sample = `
version: 1
fields:
  title:
    type: string
    required: true
    derive: h1
  type:
    type: string
    required: true
    enum: [concept, reference]
  description:
    type: string
    required: true
    min_length: 30
    derive: llm
rules:
  - name: desc-trigger
    when: exists(description)
    assert: length(description) >= 30
    severity: error
    message: too short
`

func TestParseSchema(t *testing.T) {
	s, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 1 {
		t.Fatalf("version = %d", s.Version)
	}
	if !s.Fields["title"].Required {
		t.Fatalf("title should be required")
	}
	if s.Fields["description"].MinLength != 30 {
		t.Fatalf("min_length = %d", s.Fields["description"].MinLength)
	}
	if len(s.Fields["type"].Enum) != 2 {
		t.Fatalf("enum = %v", s.Fields["type"].Enum)
	}
	if len(s.Rules) != 1 || s.Rules[0].Name != "desc-trigger" {
		t.Fatalf("rules = %v", s.Rules)
	}
	if len(s.FieldOrder) != 3 || s.FieldOrder[0] != "title" {
		t.Fatalf("field order = %v", s.FieldOrder)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := LoadConfig("/does/not/exist.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if c.Root != "." {
		t.Fatalf("default root = %q", c.Root)
	}
	if c.Output.DefaultFormat != "auto" {
		t.Fatalf("default format = %q", c.Output.DefaultFormat)
	}
}
