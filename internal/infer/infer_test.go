// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package infer

import (
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
)

func TestObserveComputesPresence(t *testing.T) {
	docs := []model.Doc{
		{Fields: map[string]model.Value{"title": {Kind: model.KindString, Str: "A"}}},
		{Fields: map[string]model.Value{
			"title": {Kind: model.KindString, Str: "B"},
			"tags":  {Kind: model.KindList, Str: `["x"]`},
		}},
	}
	stats := Observe(docs)
	if stats[0].Name != "title" || stats[0].Present != 1.0 {
		t.Fatalf("title stat = %+v", stats[0])
	}
	var tags FieldStat
	for _, s := range stats {
		if s.Name == "tags" {
			tags = s
		}
	}
	if tags.Present != 0.5 {
		t.Fatalf("tags present = %v, want 0.5", tags.Present)
	}
	if tags.Types[0] != "list<string>" {
		t.Fatalf("tags type = %v", tags.Types)
	}
}

func TestObserveNameTieBreak(t *testing.T) {
	docs := []model.Doc{
		{Fields: map[string]model.Value{
			"zebra": {Kind: model.KindString, Str: "z"},
			"apple": {Kind: model.KindString, Str: "a"},
		}},
	}
	stats := Observe(docs)
	if len(stats) != 2 {
		t.Fatalf("got %d stats, want 2", len(stats))
	}
	if stats[0].Name != "apple" || stats[1].Name != "zebra" {
		t.Fatalf("expected apple then zebra, got %s then %s", stats[0].Name, stats[1].Name)
	}
}

func TestToSchemaEmitsCorrectTypes(t *testing.T) {
	stats := []FieldStat{
		{Name: "count", Present: 1.0, Types: []string{"number"}},
		{Name: "published", Present: 0.8, Types: []string{"date"}},
		{Name: "active", Present: 0.9, Types: []string{"bool"}},
		{Name: "tags", Present: 0.7, Types: []string{"list<string>"}},
		{Name: "mixed", Present: 0.6, Types: []string{"string", "number"}},
		{Name: "name", Present: 0.5, Types: []string{"string"}},
	}

	b := ToSchema(stats, 0.8)
	schema := string(b)

	tests := []struct {
		field    string
		wantType string
		required bool
	}{
		{"count", "number", true},
		{"published", "date", true},
		{"active", "bool", true},
		{"tags", "list", false},
		{"mixed", "string", false},
		{"name", "string", false},
	}

	for _, tc := range tests {
		if !contains(schema, "  "+tc.field+":") {
			t.Errorf("field %s not found in schema", tc.field)
		}
		if !contains(schema, "type: "+tc.wantType) {
			t.Errorf("field %s: expected type %s, got:\n%s", tc.field, tc.wantType, schema)
		}
		reqStr := "required: true"
		if !tc.required {
			reqStr = "required: false"
		}
		if !contains(schema, reqStr) {
			t.Errorf("field %s: expected %s, got:\n%s", tc.field, reqStr, schema)
		}
	}
}

func contains(s, substr string) bool {
	return len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
