// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

// Package globs matches repo-relative, slash-separated paths against the glob
// patterns used in nasc config and frontmatter.
//
// The rules follow what people expect from .gitignore-style config:
//
//   - `*` matches within one path segment, `**` matches across segments.
//   - `**/` also matches zero directories, so `**/*.md` matches README.md and
//     `docs/**/*.md` matches docs/a.md.
//   - A pattern with no slash matches the file's base name at any depth, so
//     `CHANGELOG.md` excludes every CHANGELOG.md in the tree. A leading `/`
//     anchors it to the root instead.
//   - A trailing `/**` matches everything under that directory.
package globs

import (
	"strings"
	"sync"

	"github.com/gobwas/glob"
)

var (
	mu    sync.Mutex
	cache = map[string]*glob.Pattern{}
)

func compile(p string) (*glob.Pattern, error) {
	mu.Lock()
	defer mu.Unlock()
	if g, ok := cache[p]; ok {
		return g, nil
	}
	g, err := glob.Compile(p, '/')
	if err != nil {
		return nil, err
	}
	cache[p] = g
	return g, nil
}

// Match reports whether rel matches pattern. An invalid pattern never matches.
func Match(pattern, rel string) bool {
	pattern = strings.TrimSpace(pattern)
	// A leading `/` (or `./`) anchors the pattern to the root, as in
	// .gitignore, so `/CHANGELOG.md` matches only the top-level file.
	anchored := strings.HasPrefix(pattern, "/") || strings.HasPrefix(pattern, "./")
	pattern = strings.TrimPrefix(strings.TrimPrefix(pattern, "./"), "/")
	if pattern == "" {
		return false
	}
	rel = strings.TrimPrefix(rel, "./")
	if !anchored && !strings.Contains(pattern, "/") {
		base := rel
		if i := strings.LastIndexByte(rel, '/'); i >= 0 {
			base = rel[i+1:]
		}
		return matchOne(pattern, base)
	}
	return matchDeep(pattern, rel)
}

// matchDeep matches pattern against rel and lets every `**/` also match zero
// directories, so `**/x` matches `x` at the root and `docs/**/*.md` matches
// `docs/a.md`.
func matchDeep(pattern, rel string) bool {
	if matchOne(pattern, rel) {
		return true
	}
	if strings.HasPrefix(pattern, "**/") && matchDeep(pattern[len("**/"):], rel) {
		return true
	}
	for i := 0; ; {
		j := strings.Index(pattern[i:], "/**/")
		if j < 0 {
			return false
		}
		i += j
		if matchDeep(pattern[:i]+"/"+pattern[i+len("/**/"):], rel) {
			return true
		}
		i++
	}
}

func matchOne(pattern, s string) bool {
	g, err := compile(pattern)
	if err != nil {
		return false
	}
	return g.Match(s)
}

// MatchAny reports whether rel matches any of patterns.
func MatchAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		if Match(p, rel) {
			return true
		}
	}
	return false
}

// DirExcluded reports whether a whole directory can be skipped because every
// path under it is excluded. It applies the same rules as Match, so it only
// fires for patterns ending in `/**` (or `**`) that cover relDir.
func DirExcluded(patterns []string, relDir string) bool {
	probe := relDir + "/\x00nasc-probe"
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if !strings.HasSuffix(p, "**") {
			continue
		}
		if Match(p, probe) {
			return true
		}
	}
	return false
}
