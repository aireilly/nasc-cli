// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

// DefaultLLMPrompt is the opening instruction buildPrompt prepends to every
// request. It frames the task for the model; the machinery that follows (the
// JSON contract, field meanings, and doc context) is fixed. Config may override
// it via mark.llm_prompt.
const DefaultLLMPrompt = "You are generating navigation metadata for a documentation file. It serves two readers at once: a human skimming an index to find the right doc, and an AI agent deciding whether to load it. Write for both."

// LLMConfig configures the llm source.
type LLMConfig struct {
	// Cmd is the agent command line. It is split with shell quoting rules
	// and run directly, without a shell.
	Cmd string
	// ExcerptBytes caps how much of each body is sent. Zero or less sends
	// the whole body.
	ExcerptBytes int
	// Prompt overrides DefaultLLMPrompt when non-empty.
	Prompt string
	// Timeout bounds each call. Zero means no limit.
	Timeout time.Duration
	// Cache, when set, answers repeat questions without a call.
	Cache *Cache
	// Existing holds, per list field, the values already common in the
	// corpus, most used first. The prompt asks the model to reuse them.
	Existing map[string][]string
}

// LLMRequest describes one document and the fields wanted for it.
type LLMRequest struct {
	Path    string
	Title   string
	Type    string
	Excerpt string
	Want    []string
	Schema  map[string]schema.Field
	// Feedback explains why a previous reply was rejected, for a retry.
	Feedback string
	// Prompt overrides DefaultLLMPrompt when non-empty.
	Prompt string
	// Existing lists values already in use for a field, to reuse.
	Existing map[string][]string
}

// outputError marks a reply the command produced but nasc could not use. A
// retry with feedback may fix it; a failed or timed-out command will not.
type outputError struct{ msg string }

func (e outputError) Error() string { return e.msg }

// RunLLM runs the command, feeds it the prompt for req on stdin, and returns
// the values that pass the schema. rejected lists, in plain words, each wanted
// field that was missing or out of bounds.
func RunLLM(cfg LLMConfig, req LLMRequest) (got map[string]model.Value, rejected []string, err error) {
	argv, err := SplitCommand(cfg.Cmd)
	if err != nil {
		return nil, nil, err
	}
	if len(argv) == 0 {
		return nil, nil, errors.New("empty llm command")
	}
	ctx := context.Background()
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// A wrapper such as `go run` may leave a child holding stdout after the
	// parent is killed; stop waiting on the pipes shortly after.
	cmd.WaitDelay = 2 * time.Second
	if req.Prompt == "" {
		req.Prompt = cfg.Prompt
	}
	cmd.Stdin = strings.NewReader(buildPrompt(req))
	var out, errbuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errbuf
	if rerr := cmd.Run(); rerr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, nil, fmt.Errorf("llm command timed out after %s", cfg.Timeout)
		}
		msg := strings.TrimSpace(errbuf.String())
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		if msg == "" {
			return nil, nil, fmt.Errorf("llm command failed: %v", rerr)
		}
		return nil, nil, fmt.Errorf("llm command failed: %v: %s", rerr, msg)
	}

	var raw map[string]json.RawMessage
	if jerr := json.Unmarshal(extractJSON(out.Bytes()), &raw); jerr != nil {
		return nil, nil, outputError{"llm reply is not a JSON object: " + jerr.Error()}
	}
	got = map[string]model.Value{}
	for _, key := range req.Want {
		rawVal, ok := raw[key]
		if !ok {
			rejected = append(rejected, key+" was missing")
			continue
		}
		v, ok := coerceLLM(rawVal)
		if !ok {
			rejected = append(rejected, key+" was not a string or list of strings")
			continue
		}
		if f, ok := req.Schema[key]; ok && v.Kind == model.KindList {
			v = normaliseList(v, f)
		}
		if f, ok := req.Schema[key]; ok {
			if why := boundsProblem(v, f); why != "" {
				rejected = append(rejected, key+" "+why)
				continue
			}
		}
		got[key] = v
	}
	return got, rejected, nil
}

// buildPrompt renders the request into a self-contained instruction. It states
// the JSON-only output contract, names each requested field with its meaning and
// schema length bounds, and includes the document's path, title, type, and
// excerpt as context. The caller's command need only be a bare agent CLI.
func buildPrompt(req LLMRequest) string {
	var b strings.Builder
	prompt := req.Prompt
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
		default:
			f := req.Schema[w]
			if f.Type != "list" && w != "tags" {
				b.WriteString("  \"" + w + "\": a concise, accurate value.\n")
				continue
			}
			line := "  \"" + w + "\": an array of short lowercase topic strings."
			if f.MaxLength > 0 {
				line += fmt.Sprintf(" At most %d items.", f.MaxLength)
			}
			switch {
			case len(f.Vocabulary) > 0:
				line += " Use only values from this list: " + strings.Join(f.Vocabulary, ", ") + "."
			case len(req.Existing[w]) > 0:
				line += " Prefer values already used in this project where they fit: " + strings.Join(req.Existing[w], ", ") + "."
			}
			b.WriteString(line + "\n")
		}
	}
	if req.Feedback != "" {
		b.WriteString("\nYour previous reply could not be used: " + req.Feedback + ". Correct this and reply again with the JSON object only.\n")
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

// normaliseList lowercases and trims list items, drops empty and duplicate
// items and anything outside the field's vocabulary, and cuts the list to the
// field's maximum, so a slightly long or messy tag list is fixed rather than
// rejected.
func normaliseList(v model.Value, f schema.Field) model.Value {
	allowed := map[string]bool{}
	for _, a := range f.Vocabulary {
		allowed[a] = true
	}
	seen := map[string]bool{}
	out := []string{}
	for _, item := range v.List() {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" || seen[item] {
			continue
		}
		if len(allowed) > 0 && !allowed[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
		if f.MaxLength > 0 && len(out) == f.MaxLength {
			break
		}
	}
	b, _ := json.Marshal(out)
	return model.Value{Kind: model.KindList, Str: string(b)}
}

// boundsProblem describes how v breaks f's length bounds, or "" when it fits.
func boundsProblem(v model.Value, f schema.Field) string {
	unit := "characters"
	if v.Kind == model.KindList {
		unit = "items"
	}
	if f.MinLength > 0 && v.Len() < f.MinLength {
		return fmt.Sprintf("had %d %s, needs at least %d", v.Len(), unit, f.MinLength)
	}
	if f.MaxLength > 0 && v.Len() > f.MaxLength {
		return fmt.Sprintf("had %d %s, allows at most %d", v.Len(), unit, f.MaxLength)
	}
	return ""
}

// LLMSource builds a request for llm-derived fields the doc lacks and runs it.
// Its output is nondeterministic, so it never refreshes a field that is already
// present: re-generating on every run would churn the corpus. It fills absent
// fields, and force overwrites present ones.
//
// A reply that is unusable (not JSON, or a field out of bounds) is retried once
// with the reason attached. Whatever usable values came back are returned even
// when an error is also returned, so one bad field does not discard the rest.
func LLMSource(d model.Doc, s *schema.Schema, cfg LLMConfig, force bool) (map[string]model.Value, error) {
	if cfg.Cmd == "" || s == nil {
		return map[string]model.Value{}, nil
	}
	var want []string
	sub := map[string]schema.Field{}
	for _, name := range fieldsDerivedBy(s, "llm") {
		if !shouldSet(d, s.Key(name), force, false) {
			continue
		}
		want = append(want, name)
		sub[name] = s.Fields[name]
	}
	if len(want) == 0 {
		return map[string]model.Value{}, nil
	}
	excerpt := d.Body
	if cfg.ExcerptBytes > 0 && len(excerpt) > cfg.ExcerptBytes {
		excerpt = excerpt[:cfg.ExcerptBytes]
	}
	req := LLMRequest{Path: d.Path, Title: d.Title, Type: d.Type, Excerpt: excerpt, Want: want, Schema: sub, Prompt: cfg.Prompt, Existing: cfg.Existing}

	// The corpus hints change as docs get tagged, so they stay out of the
	// key; the doc, the fields, and the instructions decide a cache hit.
	keyReq := req
	keyReq.Existing = nil
	key := cacheKey(cfg.Cmd, buildPrompt(keyReq))
	if hit, ok := cfg.Cache.get(key); ok {
		return hit, nil
	}
	got, rejected, err := RunLLM(cfg, req)
	if err == nil && len(rejected) == 0 {
		cfg.Cache.put(key, got)
	}
	var oe outputError
	switch {
	case err != nil && !errors.As(err, &oe):
		return map[string]model.Value{}, err
	case err != nil:
		req.Feedback = err.Error()
	case len(rejected) > 0:
		req.Feedback = strings.Join(rejected, "; ")
		// Ask again only for what is still missing.
		var still []string
		for _, w := range want {
			if _, ok := got[w]; !ok {
				still = append(still, w)
			}
		}
		req.Want = still
	default:
		return got, nil
	}
	if got == nil {
		got = map[string]model.Value{}
	}
	again, rejected2, err2 := RunLLM(cfg, req)
	for k, v := range again {
		got[k] = v
	}
	if err2 != nil {
		return got, err2
	}
	if len(rejected2) > 0 {
		return got, fmt.Errorf("llm reply rejected after one retry: %s", strings.Join(rejected2, "; "))
	}
	cfg.Cache.put(key, got)
	return got, nil
}
