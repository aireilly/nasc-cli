// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Kind discriminates the concrete type a Value holds.
type Kind uint8

const (
	KindNull Kind = iota
	KindString
	KindNumber
	KindBool
	KindDate
	KindList
)

// Value is a single frontmatter value with its parsed kind. Lists are stored
// JSON-encoded in Str. Dates are stored as YYYY-MM-DD in Str.
type Value struct {
	Kind Kind
	Str  string
	Num  float64
}

// list decodes a KindList Value into its elements. Returns nil for other kinds.
func (v Value) list() []string {
	if v.Kind != KindList {
		return nil
	}
	var out []string
	_ = json.Unmarshal([]byte(v.Str), &out)
	return out
}

// List returns the decoded elements of a KindList value, nil for other kinds.
func (v Value) List() []string { return v.list() }

// Len returns element count for lists and rune count for strings. Other kinds
// return 0.
func (v Value) Len() int {
	switch v.Kind {
	case KindList:
		return len(v.list())
	case KindString, KindDate:
		return utf8.RuneCountInString(v.Str)
	default:
		return 0
	}
}

// String renders the value for display. Lists join with ", ".
func (v Value) String() string {
	switch v.Kind {
	case KindNull:
		return ""
	case KindNumber:
		return strconv.FormatFloat(v.Num, 'g', -1, 64)
	case KindBool:
		if v.Num != 0 {
			return "true"
		}
		return "false"
	case KindList:
		return strings.Join(v.list(), ", ")
	default:
		return v.Str
	}
}

// IsZero reports whether the value is null or empty.
func (v Value) IsZero() bool {
	return v.Kind == KindNull || (v.Kind == KindString && v.Str == "")
}
