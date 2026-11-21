// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package gitmeta

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if got != "@github" {
		t.Fatalf("codeownersMatch = %q, want @github (.github/CODEOWNERS wins, as on GitHub)", got)
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

	got, err := Owner(dir, "a.md", true)
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

	got, err := Owner(dir, "a.md", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "@docsteam" {
		t.Fatalf("Owner = %q, want @docsteam", got)
	}
}

// Without the blame fallback, a file no CODEOWNERS rule covers has no owner,
// so no personal email address is written into the doc.
func TestOwnerNoFallbackLeavesEmpty(t *testing.T) {
	dir := initRepo(t)
	writeAndCommit(t, dir, "a.md", "hello")
	got, err := Owner(dir, "a.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("Owner = %q, want empty without blame fallback", got)
	}
}

// When nasc scans a subdirectory, CODEOWNERS patterns still resolve against the
// repository top.
func TestCodeownersFromSubdirRoot(t *testing.T) {
	dir := initRepo(t)
	writeAndCommit(t, dir, ".github/CODEOWNERS", "/docs/api/ @api-team\n")
	writeAndCommit(t, dir, "docs/api/ref.md", "x")
	got := codeownersMatch(filepath.Join(dir, "docs"), "api/ref.md")
	if got != "@api-team" {
		t.Fatalf("codeownersMatch = %q, want @api-team", got)
	}
}

func commitAt(t *testing.T, dir, name, content, date string) {
	t.Helper()
	if err := writeFile(dir, name, content); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", name}, {"commit", "-m", "edit " + name}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// A commit that only changes frontmatter (such as nasc's own mark commit) does
// not move lastUpdated, so mark is stable across runs.
func TestLastUpdatedSkipsFrontmatterOnlyCommits(t *testing.T) {
	dir := initRepo(t)
	commitAt(t, dir, "a.md", "# A\n\nbody\n", "2024-01-10T12:00:00Z")
	commitAt(t, dir, "a.md", "---\nlastUpdated: 2024-01-10\n---\n# A\n\nbody\n", "2024-03-01T12:00:00Z")
	commitAt(t, dir, "a.md", "---\nlastUpdated: 2024-01-10\ntags: [x]\n---\n# A\n\nbody\n", "2024-04-01T12:00:00Z")
	got, err := LastUpdated(dir, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2024-01-10" {
		t.Fatalf("lastUpdated = %q, want 2024-01-10 (frontmatter-only commits skipped)", got)
	}

	commitAt(t, dir, "a.md", "---\nlastUpdated: 2024-01-10\ntags: [x]\n---\n# A\n\nnew body\n", "2024-05-02T12:00:00Z")
	got, _ = LastUpdated(dir, "a.md")
	if got != "2024-05-02" {
		t.Fatalf("lastUpdated = %q, want 2024-05-02 after a body change", got)
	}
}

func TestStripFrontmatter(t *testing.T) {
	cases := map[string]string{
		"no fm\n":                        "no fm\n",
		"---\na: 1\n---\nbody\n":         "body\n",
		"---\r\na: 1\r\n---\r\nbody\r\n": "body\n",
		"\uFEFF---\na: 1\n---\nbody":     "body",
		"---\n---\nbody":                 "body",
		"---\na: 1\n...\nbody":           "body",
		"---\nunclosed\n":                "---\nunclosed\n",
	}
	for in, want := range cases {
		if got := StripFrontmatter(in); got != want {
			t.Errorf("StripFrontmatter(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChangedFiles(t *testing.T) {
	dir := initRepo(t)
	writeAndCommit(t, dir, "docs/old.md", "old")
	cmd := exec.Command("git", "branch", "-M", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	cmd = exec.Command("git", "checkout", "-q", "-b", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	writeAndCommit(t, dir, "docs/committed.md", "new")
	if err := writeFile(dir, "docs/old.md", "edited"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir, "docs/untracked.md", "u"); err != nil {
		t.Fatal(err)
	}
	// A non-ASCII name must come back unquoted.
	if err := writeFile(dir, "docs/café.md", "c"); err != nil {
		t.Fatal(err)
	}

	got, err := ChangedFiles(dir, "main", false)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"docs/committed.md": true, "docs/old.md": true, "docs/untracked.md": true, "docs/café.md": true}
	if len(got) != len(want) {
		t.Fatalf("ChangedFiles = %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected %q in %v", g, got)
		}
	}

	// From a subdirectory root, paths are relative to it.
	got, err = ChangedFiles(filepath.Join(dir, "docs"), "main", false)
	if err != nil || len(got) != 4 || strings.HasPrefix(got[0], "docs/") {
		t.Fatalf("ChangedFiles(sub) = %v, %v", got, err)
	}

	// withDeleted also lists files the change removed.
	cmd = exec.Command("git", "rm", "-qf", "docs/old.md")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, _ = ChangedFiles(dir, "main", false)
	for _, g := range got {
		if g == "docs/old.md" {
			t.Fatalf("deleted file listed without withDeleted: %v", got)
		}
	}
	got, _ = ChangedFiles(dir, "main", true)
	found := false
	for _, g := range got {
		found = found || g == "docs/old.md"
	}
	if !found {
		t.Fatalf("withDeleted should list docs/old.md: %v", got)
	}
	if Prefix(filepath.Join(dir, "docs")) != "docs/" {
		t.Fatalf("Prefix = %q", Prefix(filepath.Join(dir, "docs")))
	}
}
