// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package model

import "testing"

func TestValueLen(t *testing.T) {
	s := Value{Kind: KindString, Str: "hello"}
	if got := s.Len(); got != 5 {
		t.Fatalf("string len = %d, want 5", got)
	}
	list := Value{Kind: KindList, Str: `["a","b","c"]`}
	if got := list.Len(); got != 3 {
		t.Fatalf("list len = %d, want 3", got)
	}
}

func TestValueStringForList(t *testing.T) {
	list := Value{Kind: KindList, Str: `["a","b"]`}
	if got := list.String(); got != "a, b" {
		t.Fatalf("list String = %q, want %q", got, "a, b")
	}
}

func TestDocFieldLookup(t *testing.T) {
	d := Doc{Fields: map[string]Value{"title": {Kind: KindString, Str: "T"}}}
	v, ok := d.Field("title")
	if !ok || v.Str != "T" {
		t.Fatalf("Field(title) = %v, %v", v, ok)
	}
	if _, ok := d.Field("missing"); ok {
		t.Fatalf("Field(missing) should be absent")
	}
}
