// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"strings"
	"testing"

	"github.com/aireilly/nasc-cli/internal/schema"
)

// The prompt nasc sends must be self-contained: it states the JSON-only
// contract, names every requested key, carries the doc context, and passes the
// schema length bounds through. This is what lets `--llm-cmd 'claude -p'` work
// without the caller supplying any instruction of their own.
func TestBuildPromptSuppliesJSONContract(t *testing.T) {
	req := LLMRequest{
		Path:    "docs/auth.md",
		Title:   "Auth flow",
		Type:    "guides",
		Excerpt: "How tokens are minted and rotated.",
		Want:    []string{"description", "tags"},
		Schema: map[string]schema.Field{
			"description": {Type: "string", MinLength: 30, MaxLength: 200},
		},
	}
	p := buildPrompt(req)
	for _, want := range []string{"JSON", "description", "tags", "docs/auth.md", "Auth flow", "guides", "tokens are minted", "30", "200"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt missing %q\n---\n%s", want, p)
		}
	}
}

// Real agents often wrap the object in a ```json fence or a line of prose.
// extractJSON must pull the object out so parsing still succeeds.
func TestExtractJSONFindsObjectInNoise(t *testing.T) {
	in := "Sure, here you go:\n```json\n{\"description\":\"x\",\"tags\":[\"a\"]}\n```\nHope that helps!"
	got := string(extractJSON([]byte(in)))
	if got != `{"description":"x","tags":["a"]}` {
		t.Fatalf("extractJSON = %q", got)
	}
}

func TestExtractJSONLeavesCleanObjectAlone(t *testing.T) {
	in := `{"description":"x"}`
	if got := string(extractJSON([]byte(in))); got != in {
		t.Fatalf("extractJSON mangled a clean object: %q", got)
	}
}

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
