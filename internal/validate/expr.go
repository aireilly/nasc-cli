// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/gobwas/glob"
)

// EvalContext carries everything an expression can reference.
type EvalContext struct {
	Doc   model.Doc
	Root  string
	Today time.Time
}

// EvalIn evaluates a boolean rule expression against ctx.
func EvalIn(expr string, ctx EvalContext) (bool, error) {
	left, op, right, hasOp := splitComparison(expr)
	if !hasOp {
		v, err := evalTerm(strings.TrimSpace(expr), ctx)
		if err != nil {
			return false, err
		}
		return truthy(v), nil
	}
	lv, err := evalTerm(strings.TrimSpace(left), ctx)
	if err != nil {
		return false, err
	}
	rv, err := evalTerm(strings.TrimSpace(right), ctx)
	if err != nil {
		return false, err
	}
	return compare(lv, op, rv)
}

// Eval is a convenience wrapper for tests and simple callers.
func Eval(expr string, d model.Doc, today time.Time) (bool, error) {
	return EvalIn(expr, EvalContext{Doc: d, Root: ".", Today: today})
}

type term struct {
	num    float64
	str    string
	isNum  bool
	isBool bool
	b      bool
}

func splitComparison(expr string) (left, op, right string, ok bool) {
	// Track if we're inside quotes to avoid splitting on operators in quoted strings
	inQuotes := false
	for i := 0; i < len(expr); i++ {
		if expr[i] == '"' {
			inQuotes = !inQuotes
			continue
		}
		if inQuotes {
			continue
		}

		// Check 2-char operators first to avoid matching > when >= appears
		if i+1 < len(expr) {
			twoChar := expr[i : i+2]
			for _, o := range []string{">=", "<=", "==", "!="} {
				if twoChar == o {
					return expr[:i], o, expr[i+len(o):], true
				}
			}
		}

		// Check 1-char operators
		oneChar := string(expr[i])
		for _, o := range []string{">", "<"} {
			if oneChar == o {
				return expr[:i], o, expr[i+len(o):], true
			}
		}
	}
	return "", "", "", false
}

func evalTerm(s string, ctx EvalContext) (term, error) {
	switch {
	case strings.HasPrefix(s, "exists(") && strings.HasSuffix(s, ")"):
		name := s[len("exists(") : len(s)-1]
		_, ok := ctx.Doc.Field(name)
		return term{isBool: true, b: ok}, nil
	case strings.HasPrefix(s, "length(") && strings.HasSuffix(s, ")"):
		name := s[len("length(") : len(s)-1]
		v, _ := ctx.Doc.Field(name)
		return term{isNum: true, num: float64(v.Len())}, nil
	case strings.HasPrefix(s, "path_match_count(") && strings.HasSuffix(s, ")"):
		return term{isNum: true, num: float64(pathMatchCount(ctx))}, nil
	case s == "today()":
		return term{str: ctx.Today.Format("2006-01-02")}, nil
	case strings.HasPrefix(s, "contains(") && strings.HasSuffix(s, ")"):
		return containsTerm(s, ctx)
	case len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"':
		return term{str: s[1 : len(s)-1]}, nil
	case strings.HasPrefix(s, `"`) || strings.HasSuffix(s, `"`):
		// Malformed string literal (unterminated or odd quote)
		return term{}, fmt.Errorf("malformed string literal: %q", s)
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return term{isNum: true, num: n}, nil
	}
	// Bare identifier: a field reference.
	v, ok := ctx.Doc.Field(s)
	if !ok {
		return term{}, nil
	}
	if v.Kind == model.KindNumber {
		return term{isNum: true, num: v.Num}, nil
	}
	return term{str: v.String()}, nil
}

func containsTerm(s string, ctx EvalContext) (term, error) {
	inner := s[len("contains(") : len(s)-1]
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return term{}, fmt.Errorf("contains needs two arguments: %q", s)
	}
	field := strings.TrimSpace(parts[0])
	needle := strings.Trim(strings.TrimSpace(parts[1]), `"`)
	v, _ := ctx.Doc.Field(field)
	return term{isBool: true, b: strings.Contains(v.String(), needle)}, nil
}

func pathMatchCount(ctx EvalContext) int {
	root := ctx.Root
	if root == "" {
		root = "."
	}
	count := 0
	for _, pattern := range ctx.Doc.Paths {
		g, err := glob.Compile(pattern, '/')
		if err != nil {
			continue
		}
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			if g.Match(filepath.ToSlash(rel)) {
				count++
			}
			return nil
		})
	}
	return count
}

func truthy(t term) bool {
	if t.isBool {
		return t.b
	}
	if t.isNum {
		return t.num != 0
	}
	return t.str != ""
}

func compare(l term, op string, r term) (bool, error) {
	if l.isNum && r.isNum {
		switch op {
		case ">=":
			return l.num >= r.num, nil
		case "<=":
			return l.num <= r.num, nil
		case ">":
			return l.num > r.num, nil
		case "<":
			return l.num < r.num, nil
		case "==":
			return l.num == r.num, nil
		case "!=":
			return l.num != r.num, nil
		}
	}

	ls, rs := l.str, r.str

	// For equality/inequality, any types can compare
	if op == "==" || op == "!=" {
		if op == "==" {
			return ls == rs, nil
		}
		return ls != rs, nil
	}

	// For ordering operators (>= > <= <), try date comparison first
	if op == ">=" || op == "<=" || op == ">" || op == "<" {
		lTime, lErr := time.Parse("2006-01-02", ls)
		rTime, rErr := time.Parse("2006-01-02", rs)
		if lErr == nil && rErr == nil {
			switch op {
			case ">=":
				return lTime.After(rTime) || lTime.Equal(rTime), nil
			case "<=":
				return lTime.Before(rTime) || lTime.Equal(rTime), nil
			case ">":
				return lTime.After(rTime), nil
			case "<":
				return lTime.Before(rTime), nil
			}
		}
	}

	// If neither operand is valid for this operation, return false for missing fields, error for type mismatch
	if l.str == "" || r.str == "" {
		return false, nil
	}
	return false, fmt.Errorf("operator %q needs numeric or date operands, got %q and %q", op, ls, rs)
}
