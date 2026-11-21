// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// pre-commit passes every staged markdown file, so validate skips files
// outside the scan root instead of failing the commit.
func TestValidateSkipsFilesOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "config.yaml"), "root: docs\n")
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), strictSchema)
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "---\ntitle: A\n---\n")
	mustWrite(t, filepath.Join(dir, "README.md"), "no title\n")
	chdir(t, dir)

	_, errOut, err := run(t, "validate", "--format", "jsonl", "docs/a.md", "README.md")
	expectCode(t, err, 0)
	if !strings.Contains(errOut, "skipping README.md") {
		t.Fatalf("stderr: %s", errOut)
	}
}

// A schema change can break untouched docs, so --changed checks them all.
func TestValidateChangedSchemaChecksEverything(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), "version: 1\nfields:\n  title: {type: string}\n")
	mustWrite(t, filepath.Join(dir, "legacy.md"), "no title\n")
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "t@example.com")
	gitIn(t, dir, "config", "user.name", "T")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "init")
	gitIn(t, dir, "checkout", "-qb", "feature")
	chdir(t, dir)

	_, _, err := run(t, "validate", "--format", "jsonl", "--changed", "main")
	expectCode(t, err, 0)

	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), strictSchema)
	_, errOut, err := run(t, "validate", "--format", "jsonl", "--changed", "main")
	expectCode(t, err, 3)
	if !strings.Contains(errOut, ".nasc/schema.yaml changed") {
		t.Fatalf("stderr: %s", errOut)
	}
}

// A pointer and an index that resolve to the same file are a usage error,
// not an index that is always out of date.
func TestIndexPointerSameAsOutput(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "config.yaml"), "index:\n  output: docs/INDEX.md\n  pointer: AGENTS.md\n")
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# Alpha\n")
	chdir(t, dir)

	_, _, err := run(t, "index")
	expectCode(t, err, 0)
	_, _, err = run(t, "index", "--check")
	expectCode(t, err, 0)
	_, _, err = run(t, "index", "--check", "--output", "AGENTS.md")
	expectCode(t, err, 2)
}

// A top-level directory that loses its last doc gets its subindex emptied,
// so --check does not pass on a stale list.
func TestIndexNestedEmptiesStaleSubindex(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "top.md"), "# Top\n")
	mustWrite(t, filepath.Join(dir, "deploy", "b.md"), "# Beta\n")
	chdir(t, dir)

	_, _, err := run(t, "index", "--output", "AGENTS.md", "--nested")
	expectCode(t, err, 0)
	if err := os.Remove(filepath.Join(dir, "deploy", "b.md")); err != nil {
		t.Fatal(err)
	}
	_, _, err = run(t, "index", "--check", "--output", "AGENTS.md", "--nested")
	expectCode(t, err, 3)
	_, _, err = run(t, "index", "--output", "AGENTS.md", "--nested")
	expectCode(t, err, 0)
	got, _ := os.ReadFile(filepath.Join(dir, "deploy", "AGENTS.md"))
	if strings.Contains(string(got), "Beta") {
		t.Fatalf("stale subindex still lists Beta:\n%s", got)
	}
	_, _, err = run(t, "index", "--check", "--output", "AGENTS.md", "--nested")
	expectCode(t, err, 0)
}

// get reads a relative path from the -C directory, like git -C.
func TestGetHonoursRootFlag(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "---\ntitle: Alpha\n---\n")
	chdir(t, t.TempDir())

	out, _, err := run(t, "-C", dir, "get", "docs/a.md", "title")
	expectCode(t, err, 0)
	if strings.TrimSpace(out) != "Alpha" {
		t.Fatalf("got %q", out)
	}
}

// A rule whose expression cannot be evaluated keeps its own severity.
func TestRuleErrorKeepsSeverity(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), `version: 1
fields:
  lastUpdated: {type: string}
rules:
  - name: doc-is-fresh
    when: exists(lastUpdated)
    assert: days_since(lastUpdated) <= 365
    severity: warn
    message: stale
`)
	mustWrite(t, filepath.Join(dir, "a.md"), "---\nlastUpdated: Jan 2024\n---\n")
	chdir(t, dir)

	out, _, err := run(t, "validate", "--format", "jsonl")
	expectCode(t, err, 0)
	if !strings.Contains(out, `"severity":"warn"`) || !strings.Contains(out, "rule error") {
		t.Fatalf("out: %s", out)
	}
}
