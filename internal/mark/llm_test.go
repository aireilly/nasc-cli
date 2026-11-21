// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aireilly/nasc-cli/internal/model"
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
	got, _, err := RunLLM(LLMConfig{Cmd: "go run ../../testdata/stub-llm"}, req)
	if err != nil {
		t.Fatal(err)
	}
	if got["description"].Len() < 30 {
		t.Fatalf("description too short: %q", got["description"].Str)
	}
}

func TestRunLLMRejectsBadJSON(t *testing.T) {
	if _, _, err := RunLLM(LLMConfig{Cmd: "printf notjson"}, LLMRequest{}); err == nil {
		t.Fatalf("expected error on malformed output")
	}
}

func TestSplitCommand(t *testing.T) {
	cases := map[string][]string{
		"claude -p":              {"claude", "-p"},
		`ollama run "llama3:8b"`: {"ollama", "run", "llama3:8b"},
		`llm -s 'two words' --x`: {"llm", "-s", "two words", "--x"},
		`a "b \"c\"" d\ e`:       {"a", `b "c"`, "d e"},
		`  spaced   out `:        {"spaced", "out"},
		`empty ""`:               {"empty", ""},
	}
	for in, want := range cases {
		got, err := SplitCommand(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("SplitCommand(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := SplitCommand(`a "unterminated`); err == nil {
		t.Error("expected an error for an unterminated quote")
	}
}

func descSchema() *schema.Schema {
	s, _ := schema.Parse([]byte("version: 1\nfields:\n  description: {type: string, derive: llm, min_length: 30}\n"))
	return s
}

// A command that hangs is stopped at the timeout and reported.
func TestLLMSourceTimesOut(t *testing.T) {
	cfg := LLMConfig{Cmd: "sleep 5", Timeout: 200 * time.Millisecond}
	start := time.Now()
	_, err := LLMSource(model.Doc{Path: "a.md"}, descSchema(), cfg, false)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout error, got %v", err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatalf("timeout did not stop the command promptly")
	}
}

// A command that fails is reported, never silently dropped.
func TestLLMSourceReportsCommandFailure(t *testing.T) {
	_, err := LLMSource(model.Doc{Path: "a.md"}, descSchema(), LLMConfig{Cmd: "false"}, false)
	if err == nil || !strings.Contains(err.Error(), "llm command failed") {
		t.Fatalf("expected a command failure, got %v", err)
	}
}

// An out-of-bounds value is retried once, then reported with the reason.
func TestLLMSourceRetriesThenReportsRejection(t *testing.T) {
	_, err := LLMSource(model.Doc{Path: "a.md"}, descSchema(), LLMConfig{Cmd: `printf '{"description":"too short"}'`}, false)
	if err == nil || !strings.Contains(err.Error(), "needs at least 30") {
		t.Fatalf("expected a rejection naming the bound, got %v", err)
	}
}

// The retry prompt carries the reason the first reply was rejected.
func TestBuildPromptIncludesFeedback(t *testing.T) {
	p := buildPrompt(LLMRequest{Path: "a.md", Want: []string{"description"}, Feedback: "description had 9 characters, needs at least 30"})
	if !strings.Contains(p, "needs at least 30") {
		t.Fatalf("feedback missing from prompt:\n%s", p)
	}
}
