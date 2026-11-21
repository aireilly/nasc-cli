// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aireilly/nasc-cli/internal/globs"
	"github.com/aireilly/nasc-cli/internal/model"
)

// EvalContext carries everything an expression can reference.
type EvalContext struct {
	Doc   model.Doc
	Root  string
	Today time.Time
	// Aliases maps a schema field name to the frontmatter key that stores
	// it, so rules can name fields the way the schema does.
	Aliases map[string]string
	// Files, when set, is the shared file list path_match_count matches
	// against. Nil lists the files on each call.
	Files *FileList
}

// field looks up a field by schema name, following any alias.
func (c EvalContext) field(name string) (model.Value, bool) {
	if k, ok := c.Aliases[name]; ok {
		return c.Doc.Field(k)
	}
	return c.Doc.Field(name)
}

// EvalIn evaluates a boolean rule expression against ctx. Expressions combine
// comparisons with `&&`, `||`, `!`, and parentheses; `&&` binds tighter than
// `||`.
func EvalIn(expr string, ctx EvalContext) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false, fmt.Errorf("empty expression")
	}
	if parts := splitTop(expr, "||"); len(parts) > 1 {
		for _, p := range parts {
			ok, err := EvalIn(p, ctx)
			if err != nil || ok {
				return ok, err
			}
		}
		return false, nil
	}
	if parts := splitTop(expr, "&&"); len(parts) > 1 {
		for _, p := range parts {
			ok, err := EvalIn(p, ctx)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	}
	if strings.HasPrefix(expr, "!") && !strings.HasPrefix(expr, "!=") {
		ok, err := EvalIn(expr[1:], ctx)
		return !ok, err
	}
	if wrapped(expr) {
		return EvalIn(expr[1:len(expr)-1], ctx)
	}
	return evalComparison(expr, ctx)
}

// splitTop splits expr on op where op is outside quotes and parentheses. It
// returns a single element when op does not occur at the top level.
func splitTop(expr, op string) []string {
	var parts []string
	depth, start := 0, 0
	inQuotes := false
	for i := 0; i < len(expr); i++ {
		switch c := expr[i]; {
		case c == '"':
			inQuotes = !inQuotes
		case inQuotes:
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && strings.HasPrefix(expr[i:], op):
			parts = append(parts, expr[start:i])
			i += len(op) - 1
			start = i + 1
		}
	}
	return append(parts, expr[start:])
}

// wrapped reports whether expr is one parenthesised group, as in "(a || b)"
// but not "(a) || (b)" or "length(x)".
func wrapped(expr string) bool {
	if len(expr) < 2 || expr[0] != '(' || expr[len(expr)-1] != ')' {
		return false
	}
	depth := 0
	inQuotes := false
	for i := 0; i < len(expr); i++ {
		switch c := expr[i]; {
		case c == '"':
			inQuotes = !inQuotes
		case inQuotes:
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 && i != len(expr)-1 {
				return false
			}
		}
	}
	return true
}

func evalComparison(expr string, ctx EvalContext) (bool, error) {
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
		_, ok := ctx.field(name)
		return term{isBool: true, b: ok}, nil
	case strings.HasPrefix(s, "length(") && strings.HasSuffix(s, ")"):
		name := s[len("length(") : len(s)-1]
		v, _ := ctx.field(name)
		return term{isNum: true, num: float64(v.Len())}, nil
	case strings.HasPrefix(s, "path_match_count(") && strings.HasSuffix(s, ")"):
		return term{isNum: true, num: float64(pathMatchCount(ctx))}, nil
	case s == "today()":
		return term{str: ctx.Today.Format("2006-01-02")}, nil
	case strings.HasPrefix(s, "days_since(") && strings.HasSuffix(s, ")"):
		name := strings.TrimSpace(s[len("days_since(") : len(s)-1])
		v, ok := ctx.field(name)
		if !ok {
			return term{}, fmt.Errorf("days_since: %s is not set (guard the rule with when: exists(%s))", name, name)
		}
		t, err := time.Parse("2006-01-02", v.String())
		if err != nil {
			return term{}, fmt.Errorf("days_since: %s is %q, not a YYYY-MM-DD date", name, v.String())
		}
		today := time.Date(ctx.Today.Year(), ctx.Today.Month(), ctx.Today.Day(), 0, 0, 0, 0, time.UTC)
		return term{isNum: true, num: float64(int(today.Sub(t).Hours() / 24))}, nil
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
	v, ok := ctx.field(s)
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
	v, _ := ctx.field(field)
	return term{isBool: true, b: strings.Contains(v.String(), needle)}, nil
}

func pathMatchCount(ctx EvalContext) int {
	files := ctx.Files
	if files == nil {
		files = NewFileList(ctx.Root)
	}
	all := files.All()
	count := 0
	for _, pattern := range ctx.Doc.Paths {
		for _, f := range all {
			if globs.Match(pattern, f) {
				count++
			}
		}
	}
	return count
}

// FileList is the set of files under a root, listed once on first use and
// shared by every rule evaluation in a run.
type FileList struct {
	root  string
	once  sync.Once
	files []string
}

// NewFileList returns a lazily populated list of the files under root.
func NewFileList(root string) *FileList {
	if root == "" {
		root = "."
	}
	return &FileList{root: root}
}

// All returns every file under the root as a slash-separated relative path,
// skipping .git and node_modules.
func (l *FileList) All() []string {
	l.once.Do(func() {
		_ = filepath.WalkDir(l.root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if n := d.Name(); n == ".git" || n == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if rel, rerr := filepath.Rel(l.root, p); rerr == nil {
				l.files = append(l.files, filepath.ToSlash(rel))
			}
			return nil
		})
	})
	return l.files
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
