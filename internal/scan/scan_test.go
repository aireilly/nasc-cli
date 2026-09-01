// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkFiltersAndIgnores(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/a.md", "a")
	writeFile(t, dir, "docs/b.markdown", "b")
	writeFile(t, dir, "README.mdx", "c")
	writeFile(t, dir, "notes.txt", "skip")
	writeFile(t, dir, "node_modules/pkg/readme.md", "skip")
	writeFile(t, dir, "vendor/x.md", "skip")
	writeFile(t, dir, ".gitignore", "docs/b.markdown\n")

	got, err := Walk(Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"README.mdx", "docs/a.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Walk = %v, want %v", got, want)
	}
}

// Agent instruction and skill files are not documentation, so Walk never yields
// them: CLAUDE.md/AGENTS.md/GEMINI.md and any SKILL.md by name, and everything
// under a .claude or .cursor directory anywhere in the tree.
func TestWalkSkipsAgentFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/real.md", "keep")
	writeFile(t, dir, "AGENTS.md", "skip")
	writeFile(t, dir, "CLAUDE.md", "skip")
	writeFile(t, dir, "GEMINI.md", "skip")
	writeFile(t, dir, "docs/SKILL.md", "skip")
	writeFile(t, dir, ".claude/skills/x/SKILL.md", "skip")
	writeFile(t, dir, ".claude/commands/y.md", "skip")
	writeFile(t, dir, "examples/.claude/skills/z/SKILL.md", "skip")
	writeFile(t, dir, ".cursor/rules.md", "skip")
	writeFile(t, dir, ".github/PULL_REQUEST_TEMPLATE.md", "skip")
	writeFile(t, dir, ".github/ISSUE_TEMPLATE/bug.md", "skip")
	writeFile(t, dir, ".vscode/notes.md", "skip")

	got, err := Walk(Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"docs/real.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Walk = %v, want %v", got, want)
	}
}
