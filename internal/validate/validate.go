// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package validate

import (
	"fmt"
	"strings"
	"time"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

// Finding is one validation problem on one document.
type Finding struct {
	Path     string `json:"path"`
	Field    string `json:"field"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Check validates every doc against the schema and returns all findings.
func Check(docs []model.Doc, s *schema.Schema, root string) []Finding {
	var out []Finding
	now := time.Now()
	aliases := s.Aliases()
	files := NewFileList(root)
	for _, d := range docs {
		out = append(out, checkFields(d, s)...)
		for _, r := range s.Rules {
			out = append(out, checkRule(d, r, root, now, aliases, files)...)
		}
	}
	return out
}

func checkFields(d model.Doc, s *schema.Schema) []Finding {
	var out []Finding
	for _, name := range fieldNames(s) {
		f := s.Fields[name]
		v, present := d.Field(s.Key(name))
		if !present {
			if f.Required {
				out = append(out, Finding{d.Path, name, "required", "error",
					fmt.Sprintf("required field %q is missing", name)})
			}
			continue
		}
		if !typeMatches(f.Type, v.Kind) {
			out = append(out, Finding{d.Path, name, "type", "error",
				fmt.Sprintf("field %q should be %s", name, f.Type)})
			continue
		}
		if len(f.Enum) > 0 && !inEnum(v.String(), f.Enum) {
			out = append(out, Finding{d.Path, name, "enum", "error",
				fmt.Sprintf("field %q value %q is not in the allowed set", name, v.String())})
		}
		if f.MinLength > 0 && v.Len() < f.MinLength {
			out = append(out, Finding{d.Path, name, "min_length", "error",
				fmt.Sprintf("field %q is shorter than %d", name, f.MinLength)})
		}
		if f.MaxLength > 0 && v.Len() > f.MaxLength {
			out = append(out, Finding{d.Path, name, "max_length", "error",
				fmt.Sprintf("field %q is longer than %d", name, f.MaxLength)})
		}
		if len(f.Vocabulary) > 0 {
			var odd []string
			for _, item := range valueItems(v) {
				if !inEnum(item, f.Vocabulary) {
					odd = append(odd, item)
				}
			}
			if len(odd) > 0 {
				out = append(out, Finding{d.Path, name, "vocabulary", "warn",
					fmt.Sprintf("field %q uses %s, which the vocabulary does not list", name, strings.Join(odd, ", "))})
			}
		}
	}
	return out
}

func checkRule(d model.Doc, r schema.Rule, root string, now time.Time, aliases map[string]string, files *FileList) []Finding {
	ctx := EvalContext{Doc: d, Root: root, Today: now, Aliases: aliases, Files: files}
	sev := r.Severity
	if sev == "" {
		sev = "error"
	}
	// An expression that cannot be evaluated takes the rule's own severity,
	// so a broken value in a warn rule does not fail CI.
	if r.When != "" {
		gate, err := EvalIn(r.When, ctx)
		if err != nil {
			return []Finding{{d.Path, "", r.Name, sev, "when-clause error: " + err.Error()}}
		}
		if !gate {
			return nil
		}
	}
	ok, err := EvalIn(r.Assert, ctx)
	if err != nil {
		return []Finding{{d.Path, "", r.Name, sev, "rule error: " + err.Error()}}
	}
	if ok {
		return nil
	}
	field := ruleField(r)
	return []Finding{{d.Path, field, r.Name, sev, r.Message}}
}

// ruleField extracts the field a rule is about, from its assert expression,
// for nicer reporting. Best effort.
func ruleField(r schema.Rule) string {
	for _, prefix := range []string{"length(", "exists(", "contains("} {
		if i := indexOfStr(r.Assert, prefix); i >= 0 {
			rest := r.Assert[i+len(prefix):]
			if j := indexOfStr(rest, ")"); j >= 0 {
				name := rest[:j]
				if k := indexOfStr(name, ","); k >= 0 {
					name = name[:k]
				}
				return name
			}
		}
	}
	return ""
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func fieldNames(s *schema.Schema) []string {
	if len(s.FieldOrder) > 0 {
		return s.FieldOrder
	}
	var out []string
	for k := range s.Fields {
		out = append(out, k)
	}
	return out
}

func typeMatches(declared string, k model.Kind) bool {
	switch declared {
	case "string":
		return k == model.KindString
	case "number":
		return k == model.KindNumber
	case "bool":
		return k == model.KindBool
	case "date":
		return k == model.KindDate
	case "list":
		return k == model.KindList
	}
	return true
}

func valueItems(v model.Value) []string {
	if v.Kind == model.KindList {
		return v.List()
	}
	if v.IsZero() {
		return nil
	}
	return []string{v.String()}
}

func inEnum(v string, enum []string) bool {
	for _, e := range enum {
		if e == v {
			return true
		}
	}
	return false
}
