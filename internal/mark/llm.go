// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

// LLMCmd is set by the mark command before Plan runs the llm tier.
var LLMCmd string

// LLMExcerptBytes caps how many bytes of each doc body are sent to the llm as
// context. Zero or less means send the whole file untruncated. The mark command
// sets it from config before Plan runs the llm tier.
var LLMExcerptBytes int

// DefaultLLMPrompt is the opening instruction buildPrompt prepends to every
// request. It frames the task for the model; the machinery that follows (the
// JSON contract, field meanings, and doc context) is fixed. Config may override
// it via mark.llm_prompt.
const DefaultLLMPrompt = "You are generating navigation metadata for a documentation file. It serves two readers at once: a human skimming an index to find the right doc, and an AI agent deciding whether to load it. Write for both."

// LLMPrompt is the opening instruction sent to the llm. The mark command sets it
// from config before Plan runs the llm tier; an empty value falls back to
// DefaultLLMPrompt.
var LLMPrompt string

// LLMRequest is the JSON written to the subprocess stdin.
type LLMRequest struct {
	Path    string                  `json:"path"`
	Title   string                  `json:"title"`
	Type    string                  `json:"type"`
	Excerpt string                  `json:"excerpt"`
	Want    []string                `json:"want"`
	Schema  map[string]schema.Field `json:"schema"`
}

// RunLLM runs cmdline, feeds it req as JSON, and returns validated values.
func RunLLM(cmdline string, req LLMRequest) (map[string]model.Value, error) {
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty llm command")
	}
	cmd := exec.Command(fields[0], fields[1:]...)
	// nasc supplies the whole instruction itself, so a bare agent CLI like
	// `claude -p` works with no wrapper: the prompt on stdin states the
	// JSON-only contract and carries the doc context.
	cmd.Stdin = strings.NewReader(buildPrompt(req))
	var out, errbuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errbuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("llm command failed: %v: %s", err, errbuf.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(extractJSON(out.Bytes()), &raw); err != nil {
		return nil, fmt.Errorf("llm output is not JSON: %w", err)
	}
	result := map[string]model.Value{}
	for _, key := range req.Want {
		rawVal, ok := raw[key]
		if !ok {
			continue
		}
		v, ok := coerceLLM(rawVal)
		if !ok {
			continue
		}
		if f, ok := req.Schema[key]; ok && !withinBounds(v, f) {
			continue // drop values that violate the schema
		}
		result[key] = v
	}
	return result, nil
}

// buildPrompt renders the request into a self-contained instruction. It states
// the JSON-only output contract, names each requested field with its meaning and
// schema length bounds, and includes the document's path, title, type, and
// excerpt as context. The caller's --llm-cmd need only be a bare agent CLI.
func buildPrompt(req LLMRequest) string {
	var b strings.Builder
	prompt := LLMPrompt
	if prompt == "" {
		prompt = DefaultLLMPrompt
	}
	b.WriteString(prompt + "\n\n")
	b.WriteString("File: " + req.Path + "\n")
	if req.Title != "" {
		b.WriteString("Title: " + req.Title + "\n")
	}
	if req.Type != "" {
		b.WriteString("Type: " + req.Type + "\n")
	}
	if req.Excerpt != "" {
		b.WriteString("\nExcerpt:\n" + req.Excerpt + "\n")
	}
	b.WriteString("\nReturn ONLY a single JSON object, with no prose and no markdown fences, containing exactly these keys:\n")
	for _, w := range req.Want {
		switch w {
		case "description":
			line := "  \"description\": one sentence, written for both humans and agents, that says what a reader will learn or be able to do after loading this doc. Use direct, active language. Start with a verb such as \"Learn how to\" for task docs or \"Learn about\" for conceptual docs. Do not phrase it as a loading trigger and do not open with \"Load when\"."
			if bounds := lengthHint(req.Schema[w]); bounds != "" {
				line += " " + bounds
			}
			b.WriteString(line + "\n")
		case "tags":
			b.WriteString("  \"tags\": an array of short lowercase topic strings.\n")
		default:
			b.WriteString("  \"" + w + "\": a concise, accurate value.\n")
		}
	}
	return b.String()
}

// lengthHint describes a field's character bounds in plain words, or "" when the
// field sets none.
func lengthHint(f schema.Field) string {
	switch {
	case f.MinLength > 0 && f.MaxLength > 0:
		return fmt.Sprintf("Between %d and %d characters.", f.MinLength, f.MaxLength)
	case f.MinLength > 0:
		return fmt.Sprintf("At least %d characters.", f.MinLength)
	case f.MaxLength > 0:
		return fmt.Sprintf("At most %d characters.", f.MaxLength)
	default:
		return ""
	}
}

// extractJSON returns the outermost {...} object from b, tolerating surrounding
// prose or markdown fences that agents sometimes add. When no braces are found
// it returns b unchanged so the caller's JSON parse reports the real error.
func extractJSON(b []byte) []byte {
	start := bytes.IndexByte(b, '{')
	end := bytes.LastIndexByte(b, '}')
	if start < 0 || end < start {
		return b
	}
	return b[start : end+1]
}

func coerceLLM(raw json.RawMessage) (model.Value, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return model.Value{Kind: model.KindString, Str: s}, true
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		b, _ := json.Marshal(list)
		return model.Value{Kind: model.KindList, Str: string(b)}, true
	}
	return model.Value{}, false
}

func withinBounds(v model.Value, f schema.Field) bool {
	if f.MinLength > 0 && v.Len() < f.MinLength {
		return false
	}
	if f.MaxLength > 0 && v.Len() > f.MaxLength {
		return false
	}
	return true
}

// LLMTier builds a request for llm-derived fields the doc lacks and runs it.
// Its output is nondeterministic, so it never refreshes a field that is already
// present: re-generating on every run would churn the corpus. It fills absent
// fields, and force overwrites present ones.
func LLMTier(d model.Doc, s *schema.Schema, root string, force bool) map[string]model.Value {
	if LLMCmd == "" || s == nil {
		return map[string]model.Value{}
	}
	var want []string
	sub := map[string]schema.Field{}
	for name, f := range s.Fields {
		if f.Derive != "llm" {
			continue
		}
		if !shouldSet(d, name, force, false) {
			continue
		}
		want = append(want, name)
		sub[name] = f
	}
	if len(want) == 0 {
		return map[string]model.Value{}
	}
	excerpt := d.Body
	if LLMExcerptBytes > 0 && len(excerpt) > LLMExcerptBytes {
		excerpt = excerpt[:LLMExcerptBytes]
	}
	req := LLMRequest{Path: d.Path, Title: d.Title, Type: d.Type, Excerpt: excerpt, Want: want, Schema: sub}
	got, err := RunLLM(LLMCmd, req)
	if err != nil {
		return map[string]model.Value{}
	}
	return got
}
