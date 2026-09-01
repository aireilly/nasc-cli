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
	in, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = bytes.NewReader(in)
	var out, errbuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errbuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("llm command failed: %v: %s", err, errbuf.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
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
func LLMTier(d model.Doc, s *schema.Schema, root string) map[string]model.Value {
	if LLMCmd == "" || s == nil {
		return map[string]model.Value{}
	}
	var want []string
	sub := map[string]schema.Field{}
	for name, f := range s.Fields {
		if f.Derive != "llm" {
			continue
		}
		if _, ok := d.Field(name); ok {
			continue
		}
		want = append(want, name)
		sub[name] = f
	}
	if len(want) == 0 {
		return map[string]model.Value{}
	}
	excerpt := d.Body
	if len(excerpt) > 2000 {
		excerpt = excerpt[:2000]
	}
	req := LLMRequest{Path: d.Path, Title: d.Title, Type: d.Type, Excerpt: excerpt, Want: want, Schema: sub}
	got, err := RunLLM(LLMCmd, req)
	if err != nil {
		return map[string]model.Value{}
	}
	return got
}
