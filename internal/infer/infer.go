// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package infer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aireilly/nasc-cli/internal/model"
)

// FieldStat summarises one observed frontmatter field across a corpus.
type FieldStat struct {
	Name    string
	Present float64
	Types   []string
	Example string
}

// Observe tallies field presence and types across docs.
func Observe(docs []model.Doc) []FieldStat {
	type acc struct {
		count   int
		types   map[string]bool
		example string
	}
	m := map[string]*acc{}
	for _, d := range docs {
		for k, v := range d.Fields {
			a := m[k]
			if a == nil {
				a = &acc{types: map[string]bool{}}
				m[k] = a
			}
			a.count++
			a.types[typeName(v)] = true
			if a.example == "" {
				a.example = v.String()
			}
		}
	}
	total := float64(len(docs))
	var out []FieldStat
	for name, a := range m {
		var types []string
		for t := range a.types {
			types = append(types, t)
		}
		sort.Strings(types)
		present := 0.0
		if total > 0 {
			present = float64(a.count) / total
		}
		out = append(out, FieldStat{Name: name, Present: present, Types: types, Example: a.example})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Present != out[j].Present {
			return out[i].Present > out[j].Present
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func typeName(v model.Value) string {
	switch v.Kind {
	case model.KindString:
		return "string"
	case model.KindNumber:
		return "number"
	case model.KindBool:
		return "bool"
	case model.KindDate:
		return "date"
	case model.KindList:
		return "list<string>"
	default:
		return "null"
	}
}

// ToSchema emits a candidate schema.yaml. Fields at or above threshold
// presence are marked required.
func ToSchema(stats []FieldStat, threshold float64) []byte {
	var b strings.Builder
	b.WriteString("version: 1\nfields:\n")
	for _, s := range stats {
		t := schemaType(s.Types)
		fmt.Fprintf(&b, "  %s:\n    type: %s\n    required: %t\n", s.Name, t, s.Present >= threshold)
	}
	return []byte(b.String())
}

// schemaType maps observed types to schema type. If all types are the same,
// emit that type. If mixed, fall back to string.
func schemaType(types []string) string {
	if len(types) == 0 {
		return "string"
	}
	if len(types) == 1 {
		switch types[0] {
		case "string":
			return "string"
		case "number":
			return "number"
		case "bool":
			return "bool"
		case "date":
			return "date"
		case "list<string>":
			return "list"
		default:
			return "string"
		}
	}
	return "string"
}
