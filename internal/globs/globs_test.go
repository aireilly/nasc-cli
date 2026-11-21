// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package globs

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, rel string
		want         bool
	}{
		{"**/*.md", "README.md", true},
		{"**/*.md", "docs/a/b.md", true},
		{"*.md", "docs/a.md", true}, // no slash: base name at any depth
		{"docs/*.md", "docs/a.md", true},
		{"docs/*.md", "docs/sub/a.md", false},
		{"docs/**", "docs/sub/a.md", true},
		{"/docs/*.md", "docs/a.md", true},
		{"CHANGELOG.md", "pkg/CHANGELOG.md", true},
		{"**/testdata/**", "testdata/x.md", true},
		{"**/testdata/**", "a/testdata/x.md", true},
		{"pkg/router/**", "pkg/router/x.go", true},
		{"pkg/router/**", "pkg/other/x.go", false},
		{"", "a.md", false},
		{"[", "a.md", false}, // invalid pattern never matches
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.rel); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.rel, got, c.want)
		}
	}
}

func TestDirExcluded(t *testing.T) {
	pats := []string{"node_modules/**", "**/testdata/**", "third_party/**", "*.md"}
	for _, d := range []string{"node_modules", "a/testdata", "testdata", "third_party"} {
		if !DirExcluded(pats, d) {
			t.Errorf("DirExcluded(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"docs", "a/third_party", "x/node_modules"} {
		if DirExcluded(pats, d) {
			t.Errorf("DirExcluded(%q) = true, want false", d)
		}
	}
}
