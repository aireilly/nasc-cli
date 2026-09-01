// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package gitmeta

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "Tester")
	return dir
}

func writeFile(dir, name, content string) error {
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeAndCommit(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := writeFile(dir, name, content); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", name}, {"commit", "-m", "add " + name}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestLastUpdated(t *testing.T) {
	dir := initRepo(t)
	writeAndCommit(t, dir, "a.md", "hello")
	got, err := LastUpdated(dir, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 || got[4] != '-' {
		t.Fatalf("lastUpdated = %q, want YYYY-MM-DD", got)
	}
}

func TestAvailable(t *testing.T) {
	dir := initRepo(t)
	if !Available(dir) {
		t.Fatalf("Available(%q) = false, want true", dir)
	}

	notRepo := t.TempDir()
	if Available(notRepo) {
		t.Fatalf("Available(%q) = true, want false", notRepo)
	}
}

func TestCodeownersMatch(t *testing.T) {
	tests := []struct {
		name       string
		codeowners string
		relpath    string
		want       string
	}{
		{
			name:       "simple wildcard match",
			codeowners: "*.go @gouser\n",
			relpath:    "main.go",
			want:       "@gouser",
		},
		{
			name:       "anchored root pattern",
			codeowners: "/docs/ @docsteam\n",
			relpath:    "docs/guide.md",
			want:       "@docsteam",
		},
		{
			name:       "anchored root pattern does not match nested dir",
			codeowners: "/docs/ @docsteam\n",
			relpath:    "src/docs/guide.md",
			want:       "",
		},
		{
			name:       "unanchored directory matches at any depth",
			codeowners: "node_modules/ @nobody\n",
			relpath:    "vendor/node_modules/pkg/index.js",
			want:       "@nobody",
		},
		{
			name:       "last matching pattern wins",
			codeowners: "*.md @default\n/docs/*.md @docswriter\n",
			relpath:    "docs/guide.md",
			want:       "@docswriter",
		},
		{
			name:       "first matching pattern overridden by later broader match",
			codeowners: "/docs/*.md @docswriter\n*.md @default\n",
			relpath:    "docs/guide.md",
			want:       "@default",
		},
		{
			name:       "comments and blank lines ignored",
			codeowners: "# comment\n\n*.go @gouser\n",
			relpath:    "main.go",
			want:       "@gouser",
		},
		{
			name:       "multiple owners returns first",
			codeowners: "*.go @first @second\n",
			relpath:    "main.go",
			want:       "@first",
		},
		{
			name:       "no match returns empty",
			codeowners: "*.go @gouser\n",
			relpath:    "main.py",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := writeFile(dir, "CODEOWNERS", tt.codeowners); err != nil {
				t.Fatal(err)
			}
			got := codeownersMatch(dir, tt.relpath)
			if got != tt.want {
				t.Fatalf("codeownersMatch(%q) = %q, want %q", tt.relpath, got, tt.want)
			}
		})
	}
}

func TestCodeownersMatchFileLocations(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"github location", ".github/CODEOWNERS"},
		{"docs location", "docs/CODEOWNERS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := writeFile(dir, tt.path, "*.go @gouser\n"); err != nil {
				t.Fatal(err)
			}
			got := codeownersMatch(dir, "main.go")
			if got != "@gouser" {
				t.Fatalf("codeownersMatch = %q, want @gouser", got)
			}
		})
	}
}

func TestCodeownersMatchPrecedenceOfLocations(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(dir, "CODEOWNERS", "*.go @root\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir, ".github/CODEOWNERS", "*.go @github\n"); err != nil {
		t.Fatal(err)
	}
	got := codeownersMatch(dir, "main.go")
	if got != "@root" {
		t.Fatalf("codeownersMatch = %q, want @root (root CODEOWNERS should win)", got)
	}
}

func TestBlameMajority(t *testing.T) {
	dir := initRepo(t)
	writeAndCommit(t, dir, "a.md", "line one\n")

	// Second commit under a different author, adding two more lines,
	// so that author should be the majority.
	if err := writeFile(dir, "a.md", "line one\nline two\nline three\n"); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "a.md")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	commitCmd := exec.Command("git",
		"-c", "user.email=majority@example.com",
		"-c", "user.name=Majority Author",
		"commit", "-m", "add more lines")
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	got, err := blameMajority(dir, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "majority@example.com" {
		t.Fatalf("blameMajority = %q, want majority@example.com", got)
	}
}

func TestBlameMajorityError(t *testing.T) {
	dir := initRepo(t)
	_, err := blameMajority(dir, "does-not-exist.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestOwnerFallsBackToBlame(t *testing.T) {
	dir := initRepo(t)
	writeAndCommit(t, dir, "a.md", "hello")

	got, err := Owner(dir, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "t@example.com" {
		t.Fatalf("Owner = %q, want t@example.com", got)
	}
}

func TestOwnerPrefersCodeowners(t *testing.T) {
	dir := initRepo(t)
	if err := writeFile(dir, "CODEOWNERS", "*.md @docsteam\n"); err != nil {
		t.Fatal(err)
	}
	writeAndCommit(t, dir, "CODEOWNERS", "*.md @docsteam\n")
	writeAndCommit(t, dir, "a.md", "hello")

	got, err := Owner(dir, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "@docsteam" {
		t.Fatalf("Owner = %q, want @docsteam", got)
	}
}
