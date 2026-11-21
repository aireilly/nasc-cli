// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const strictSchema = "version: 1\nfields:\n  title: {type: string, required: true, derive: h1}\n"

func expectCode(t *testing.T, err error, code int) {
	t.Helper()
	var ec ExitError
	if code == 0 {
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		return
	}
	if !errors.As(err, &ec) || ec.Code != code {
		t.Fatalf("expected exit code %d, got %v", code, err)
	}
}

// index --check passes on a fresh index and fails once a doc changes.
func TestIndexCheck(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# Alpha\n")
	chdir(t, dir)

	_, _, err := run(t, "index", "--check", "--output", "AGENTS.md")
	expectCode(t, err, 3) // missing file
	_, _, err = run(t, "index", "--output", "AGENTS.md")
	expectCode(t, err, 0)
	_, _, err = run(t, "index", "--check", "--output", "AGENTS.md")
	expectCode(t, err, 0)

	mustWrite(t, filepath.Join(dir, "docs", "b.md"), "# Beta\n")
	_, _, err = run(t, "index", "--check", "--output", "AGENTS.md")
	expectCode(t, err, 3)
	got, _ := os.ReadFile("AGENTS.md")
	if strings.Contains(string(got), "Beta") {
		t.Fatal("--check must not write")
	}

	_, _, err = run(t, "index", "--check")
	expectCode(t, err, 2) // no destination
}

// Paths limit validate to the named files or directories.
func TestValidateScopedToPaths(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), strictSchema)
	mustWrite(t, filepath.Join(dir, "good", "a.md"), "---\ntitle: A\n---\n")
	mustWrite(t, filepath.Join(dir, "bad", "b.md"), "no title\n")
	chdir(t, dir)

	_, _, err := run(t, "validate", "--format", "jsonl")
	expectCode(t, err, 3)
	_, _, err = run(t, "validate", "--format", "jsonl", "good")
	expectCode(t, err, 0)
	_, _, err = run(t, "validate", "--format", "jsonl", filepath.Join("good", "a.md"))
	expectCode(t, err, 0)
	_, _, err = run(t, "validate", "--format", "jsonl", "bad")
	expectCode(t, err, 3)
}

// --changed checks only docs that differ from the merge base.
func TestValidateChanged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), strictSchema)
	mustWrite(t, filepath.Join(dir, "legacy.md"), "no title\n")
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init", "-q", "-b", "main")
	gitRun("config", "user.email", "t@example.com")
	gitRun("config", "user.name", "T")
	gitRun("add", "-A")
	gitRun("commit", "-qm", "init")
	gitRun("checkout", "-qb", "feature")
	chdir(t, dir)

	mustWrite(t, filepath.Join(dir, "new.md"), "---\ntitle: New\n---\n")
	_, _, err := run(t, "validate", "--format", "jsonl", "--changed", "main")
	expectCode(t, err, 0) // the legacy doc is not part of this change

	mustWrite(t, filepath.Join(dir, "new2.md"), "no title\n")
	_, _, err = run(t, "validate", "--format", "jsonl", "--changed", "main")
	expectCode(t, err, 3)
}

// A baseline suppresses known findings so only new ones fail.
func TestValidateBaseline(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), strictSchema)
	mustWrite(t, filepath.Join(dir, "legacy.md"), "no title\n")
	chdir(t, dir)

	_, _, err := run(t, "validate", "--format", "jsonl")
	expectCode(t, err, 3)
	_, errOut, err := run(t, "validate", "--baseline", "write")
	expectCode(t, err, 0)
	if !strings.Contains(errOut, "recorded 1 finding") {
		t.Fatalf("stderr: %s", errOut)
	}
	_, errOut, err = run(t, "validate", "--format", "jsonl")
	expectCode(t, err, 0)
	if !strings.Contains(errOut, "1 known finding(s) suppressed") {
		t.Fatalf("stderr: %s", errOut)
	}

	mustWrite(t, filepath.Join(dir, "new.md"), "no title\n")
	_, _, err = run(t, "validate", "--format", "jsonl")
	expectCode(t, err, 3)
	_, _, err = run(t, "validate", "--format", "jsonl", "--baseline", "ignore", "legacy.md")
	expectCode(t, err, 3)

	mustWrite(t, filepath.Join(dir, "legacy.md"), "---\ntitle: Fixed\n---\n")
	_ = os.Remove(filepath.Join(dir, "new.md"))
	_, errOut, err = run(t, "validate", "--format", "jsonl")
	expectCode(t, err, 0)
	if !strings.Contains(errOut, "1 baseline finding(s) are fixed") {
		t.Fatalf("stderr: %s", errOut)
	}
}

// --format github prints workflow commands with repo-relative paths.
func TestValidateGitHubFormat(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), strictSchema)
	mustWrite(t, filepath.Join(dir, "docs", "b.md"), "no title\n")
	chdir(t, dir)
	out, _, err := run(t, "validate", "--format", "github")
	expectCode(t, err, 3)
	if !strings.HasPrefix(out, "::error file=docs/b.md,line=1,title=nasc required (title)::") {
		t.Fatalf("got:\n%s", out)
	}
}

// Absolute paths name docs as well as relative ones.
func TestValidateAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), strictSchema)
	mustWrite(t, filepath.Join(dir, "good", "a.md"), "---\ntitle: A\n---\n")
	mustWrite(t, filepath.Join(dir, "bad", "b.md"), "no title\n")
	chdir(t, dir)
	_, _, err := run(t, "validate", "--format", "jsonl", filepath.Join(dir, "good", "a.md"))
	expectCode(t, err, 0)
	_, _, err = run(t, "validate", "--format", "jsonl", filepath.Join(dir, "bad"))
	expectCode(t, err, 3)
	_, _, err = run(t, "affected", filepath.Join(dir, "good", "a.md"))
	expectCode(t, err, 0)
	_, _, err = run(t, "validate", "--format", "jsonl", filepath.Dir(dir))
	expectCode(t, err, 2) // outside the root
}
