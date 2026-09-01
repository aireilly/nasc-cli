// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"testing"

	"github.com/aireilly/nasc-cli/internal/schema"
)

func TestRunLLMValidatesLength(t *testing.T) {
	req := LLMRequest{
		Path: "docs/a.md",
		Want: []string{"description"},
		Schema: map[string]schema.Field{
			"description": {Type: "string", MinLength: 30},
		},
	}
	// `go run` the stub; it returns a long description.
	got, err := RunLLM("go run ../../testdata/stub-llm", req)
	if err != nil {
		t.Fatal(err)
	}
	if got["description"].Len() < 30 {
		t.Fatalf("description too short: %q", got["description"].Str)
	}
}

func TestRunLLMRejectsBadJSON(t *testing.T) {
	if _, err := RunLLM("printf notjson", LLMRequest{}); err == nil {
		t.Fatalf("expected error on malformed output")
	}
}
