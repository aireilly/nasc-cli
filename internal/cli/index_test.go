// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aireilly/nasc-cli/internal/render"
)

// Without a schema there is no definition of "marked", so --strict must fail
// loudly (like validate) instead of silently exiting 0.
func TestIndexStrictWithoutSchemaErrors(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# Title\n\nbody\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"index", "--strict"})
	err := cmd.Execute()

	var ec ExitError
	if !errors.As(err, &ec) || ec.Code != 2 {
		t.Fatalf("expected exit code 2 for --strict without schema, got %v", err)
	}
}

// Writing the index to a file that already exists preserves the hand-written
// content and refreshes only the nasc-managed region between the markers.
func TestIndexOutputPreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# Title\n\nbody\n")
	agents := filepath.Join(dir, "AGENTS.md")
	mustWrite(t, agents, "# Project agents\n\nHand-written intro that must survive.\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"index", "--output", "AGENTS.md"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("index --output failed: %v", err)
	}

	got, _ := os.ReadFile(agents)
	s := string(got)
	if !strings.Contains(s, "Hand-written intro that must survive.") {
		t.Fatalf("existing prose lost:\n%s", s)
	}
	if !strings.Contains(s, render.BeginMarker) || !strings.Contains(s, render.EndMarker) {
		t.Fatalf("managed region markers missing:\n%s", s)
	}

	// A second run must not duplicate the region.
	cmd2 := NewRootCmd()
	cmd2.SetOut(&out)
	cmd2.SetErr(&out)
	cmd2.SetArgs([]string{"index", "--output", "AGENTS.md"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second index run failed: %v", err)
	}
	got2, _ := os.ReadFile(agents)
	if strings.Count(string(got2), render.BeginMarker) != 1 {
		t.Fatalf("managed region duplicated on re-run:\n%s", string(got2))
	}
}

// With a schema present, --strict exits 3 when a doc is missing required fields.
func TestIndexStrictWithSchemaFailsOnUnmarked(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  title: {type: string, required: true}\n")
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "no frontmatter here\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"index", "--strict"})
	err := cmd.Execute()

	var ec ExitError
	if !errors.As(err, &ec) || ec.Code != 3 {
		t.Fatalf("expected exit code 3 for unmarked doc under --strict, got %v", err)
	}
}

// An index written below the root links to docs relative to its own folder,
// and its own file is not indexed on the next run.
func TestIndexLinksRelativeToOutput(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "guides", "a.md"), "# Alpha\n")
	mustWrite(t, filepath.Join(dir, "README.md"), "# Readme\n")
	chdir(t, dir)

	_, _, err := run(t, "index", "--output", filepath.Join("docs", "INDEX.md"))
	expectCode(t, err, 0)
	got, _ := os.ReadFile(filepath.Join("docs", "INDEX.md"))
	if !strings.Contains(string(got), "](guides/a.md)") || !strings.Contains(string(got), "](../README.md)") {
		t.Fatalf("links not relative to docs/:\n%s", got)
	}
	_, _, err = run(t, "index", "--check", "--output", filepath.Join("docs", "INDEX.md"))
	expectCode(t, err, 0) // INDEX.md does not list itself
}

// --pointer puts the full index in one file and a short pointer in AGENTS.md.
func TestIndexPointer(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# Alpha\n")
	mustWrite(t, filepath.Join(dir, "AGENTS.md"), "# Agents\n\nBuild with make.\n")
	chdir(t, dir)

	_, _, err := run(t, "index", "--output", filepath.Join("docs", "INDEX.md"), "--pointer", "AGENTS.md")
	expectCode(t, err, 0)
	agents, _ := os.ReadFile("AGENTS.md")
	if !strings.Contains(string(agents), "Build with make.") || !strings.Contains(string(agents), "[docs/INDEX.md](docs/INDEX.md)") {
		t.Fatalf("AGENTS.md:\n%s", agents)
	}
	if strings.Contains(string(agents), "Alpha") {
		t.Fatalf("AGENTS.md should hold only the pointer:\n%s", agents)
	}
	_, _, err = run(t, "index", "--check", "--output", filepath.Join("docs", "INDEX.md"), "--pointer", "AGENTS.md")
	expectCode(t, err, 0)
}

// --nested writes one index per top-level directory and links them.
func TestIndexNested(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# Readme\n")
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# Alpha\n")
	mustWrite(t, filepath.Join(dir, "deploy", "dev", "b.md"), "# Beta\n")
	chdir(t, dir)

	_, _, err := run(t, "index", "--output", "INDEX.md", "--nested")
	expectCode(t, err, 0)
	top, _ := os.ReadFile("INDEX.md")
	docs, _ := os.ReadFile(filepath.Join("docs", "INDEX.md"))
	deploy, _ := os.ReadFile(filepath.Join("deploy", "INDEX.md"))
	if !strings.Contains(string(top), "Readme") || strings.Contains(string(top), "Alpha") ||
		!strings.Contains(string(top), "[docs](docs/INDEX.md)") || !strings.Contains(string(top), "[deploy](deploy/INDEX.md)") {
		t.Fatalf("top:\n%s", top)
	}
	if !strings.Contains(string(docs), "](a.md)") || !strings.Contains(string(deploy), "](dev/b.md)") {
		t.Fatalf("docs:\n%s\ndeploy:\n%s", docs, deploy)
	}
	_, _, err = run(t, "index", "--check", "--output", "INDEX.md", "--nested")
	expectCode(t, err, 0)
	mustWrite(t, filepath.Join(dir, "deploy", "c.md"), "# Gamma\n")
	_, _, err = run(t, "index", "--check", "--output", "INDEX.md", "--nested")
	expectCode(t, err, 3)
}

// A long index in AGENTS.md draws a warning about per-task token cost.
func TestIndexWarnsWhenAgentsFileIsLong(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		mustWrite(t, filepath.Join(dir, "docs", n+".md"), "# "+n+"\n")
	}
	chdir(t, dir)
	_, errOut, err := run(t, "index", "--output", "AGENTS.md", "--max-lines", "3")
	expectCode(t, err, 0)
	if !strings.Contains(errOut, "on every task") {
		t.Fatalf("expected a size warning, got:\n%s", errOut)
	}
	_, errOut, _ = run(t, "index", "--output", "AGENTS.md", "--max-lines", "0")
	if strings.Contains(errOut, "on every task") {
		t.Fatalf("--max-lines 0 should disable the warning:\n%s", errOut)
	}
}
