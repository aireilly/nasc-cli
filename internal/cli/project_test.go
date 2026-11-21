// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

// Run from a subdirectory, nasc finds the project's .nasc/ above it and scans
// the whole project, exactly as from the root.
func TestRunsFromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "config.yaml"), "root: .\n")
	mustWrite(t, filepath.Join(dir, "docs", "guides", "a.md"), "# Alpha\n")
	mustWrite(t, filepath.Join(dir, "top.md"), "# Top\n")
	chdir(t, filepath.Join(dir, "docs", "guides"))

	out, _, err := run(t, "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Alpha") || !strings.Contains(out, "Top") {
		t.Fatalf("expected both docs from the project root, got:\n%s", out)
	}
}

// -C points nasc at another directory.
func TestRootFlag(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "proj", "a.md"), "# Alpha\n")
	chdir(t, dir)
	out, _, err := run(t, "-C", "proj", "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Alpha") {
		t.Fatalf("expected the doc under -C, got:\n%s", out)
	}
}

// exclude in config.yaml removes files, and the default excludes drop
// changelogs and testdata without any config.
func TestConfigExcludeIsHonoured(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "config.yaml"), "exclude: [\"drafts/**\"]\n")
	mustWrite(t, filepath.Join(dir, "a.md"), "# Keep\n")
	mustWrite(t, filepath.Join(dir, "drafts", "b.md"), "# Draft\n")
	mustWrite(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n")
	mustWrite(t, filepath.Join(dir, "pkg", "testdata", "c.md"), "# Fixture\n")
	chdir(t, dir)

	out, _, err := run(t, "index")
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"Draft", "Changelog", "Fixture"} {
		if strings.Contains(out, gone) {
			t.Fatalf("%s should be excluded, got:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "Keep") {
		t.Fatalf("expected a.md in the index, got:\n%s", out)
	}
}

// index.output in config sends the index to that file, and --output - forces
// stdout.
func TestIndexUsesConfigOutput(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "config.yaml"), "index:\n  output: docs/INDEX.md\n")
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# Alpha\n")
	chdir(t, dir)

	out, _, err := run(t, "index")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected nothing on stdout, got:\n%s", out)
	}
	got, rerr := os.ReadFile(filepath.Join("docs", "INDEX.md"))
	if rerr != nil || !strings.Contains(string(got), "Alpha") {
		t.Fatalf("expected docs/INDEX.md with the index, got %v:\n%s", rerr, got)
	}

	out, _, _ = run(t, "index", "--output", "-")
	if !strings.Contains(out, "Alpha") {
		t.Fatalf("--output - should print the index, got:\n%s", out)
	}
}

// A schema field stored under another key is validated under that key.
func TestValidateFollowsRenamedKeys(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  type: {type: string, required: true, key: nasc_type, enum: [guide]}\n")
	mustWrite(t, filepath.Join(dir, "a.md"), "---\ntype: page\nnasc_type: guide\n---\n# A\n")
	chdir(t, dir)
	if _, _, err := run(t, "validate", "--format", "jsonl"); err != nil {
		t.Fatalf("validate should pass on nasc_type, got %v", err)
	}
}
