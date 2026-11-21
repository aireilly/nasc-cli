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
)

func TestMarkWriteIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  id: {type: string, derive: slug}\n  title: {type: string, derive: h1}\n  type: {type: string, derive: parent-dir}\n")
	mustWrite(t, filepath.Join(dir, "docs", "auth.md"), "# Auth flow\n\nbody\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	first := runMark(t)
	if first == "" {
		t.Fatal("first mark made no change")
	}
	second := runMark(t)
	if second != "" {
		t.Fatalf("second mark should be a no-op, got:\n%s", second)
	}
}

func TestMarkWriteConflictReturnsExitCode5(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  id: {type: string, derive: slug}\n  title: {type: string, derive: h1}\n  type: {type: string, derive: parent-dir}\n")
	mustWrite(t, filepath.Join(dir, "docs", "auth.md"), "# Auth flow\n\nbody\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	const external = "# Auth flow\n\nchanged externally\n"
	afterReadHook = func(path string) {
		_ = os.WriteFile(path, []byte(external), 0o644)
	}
	t.Cleanup(func() { afterReadHook = nil })

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"mark", "--source", "file", "--write"})
	err := cmd.Execute()

	var ec ExitError
	if !errors.As(err, &ec) || ec.Code != 5 {
		t.Fatalf("expected exit code 5, got %v", err)
	}

	after, rerr := os.ReadFile(filepath.Join("docs", "auth.md"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(after) != external {
		t.Fatalf("file was clobbered; want externally-written bytes, got:\n%s", after)
	}
}

func TestMarkNothingToMarkReturnsExitCode1(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  id: {type: string, derive: slug}\n  title: {type: string, derive: h1}\n  type: {type: string, derive: parent-dir}\n")
	mustWrite(t, filepath.Join(dir, "docs", "auth.md"), "# Auth flow\n\nbody\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	first := runMark(t)
	if first == "" {
		t.Fatal("first mark made no change")
	}

	before, _ := os.ReadFile(filepath.Join("docs", "auth.md"))
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"mark", "--source", "file", "--write"})
	err := cmd.Execute()

	var ec ExitError
	if !errors.As(err, &ec) || ec.Code != 1 {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	after, _ := os.ReadFile(filepath.Join("docs", "auth.md"))
	if string(before) != string(after) {
		t.Fatalf("file bytes changed on a nothing-to-mark run:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func runMark(t *testing.T) string {
	t.Helper()
	before, _ := os.ReadFile(filepath.Join("docs", "auth.md"))
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"mark", "--source", "file", "--write"})
	_ = cmd.Execute()
	after, _ := os.ReadFile(filepath.Join("docs", "auth.md"))
	if string(before) == string(after) {
		return ""
	}
	return string(after)
}

// A failing llm command is reported and exits 4, while the fields other
// sources derived are still written.
func TestMarkLLMFailureExits4AndKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  title: {type: string, derive: h1}\n  description: {type: string, derive: llm}\n")
	mustWrite(t, filepath.Join(dir, "docs", "auth.md"), "# Auth flow\n\nbody\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var errOut bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"mark", "--source", "file,llm", "--llm-cmd", "false", "--write"})
	err := cmd.Execute()

	var ec ExitError
	if !errors.As(err, &ec) || ec.Code != 4 {
		t.Fatalf("expected exit code 4, got %v", err)
	}
	if !strings.Contains(errOut.String(), "docs/auth.md") || !strings.Contains(errOut.String(), "llm") {
		t.Fatalf("expected a per-file llm failure on stderr, got:\n%s", errOut.String())
	}
	got, _ := os.ReadFile(filepath.Join("docs", "auth.md"))
	if !strings.Contains(string(got), "title: Auth flow") {
		t.Fatalf("file-derived title should still be written, got:\n%s", got)
	}
}

// Asking for the llm source with no command is a usage error, not a silent
// no-op.
func TestMarkLLMWithoutCommandIsUsageError(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "# A\n")
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"mark", "--source", "llm"})
	var ec ExitError
	if err := cmd.Execute(); !errors.As(err, &ec) || ec.Code != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// In a Hugo site, mark leaves `type` alone and says how to rename it.
func TestMarkLeavesSiteGeneratorKeysAlone(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "hugo.toml"), "baseURL = 'https://example.org/'\n")
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  title: {type: string, derive: h1}\n  type: {type: string, derive: parent-dir}\n")
	mustWrite(t, filepath.Join(dir, "content", "posts", "a.md"), "# A\n")
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var errOut bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"mark", "--source", "file", "--write"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join("content", "posts", "a.md"))
	if strings.Contains(string(got), "type:") {
		t.Fatalf("type must not be written in a Hugo site, got:\n%s", got)
	}
	if !strings.Contains(errOut.String(), "key:") {
		t.Fatalf("expected a hint about key:, got:\n%s", errOut.String())
	}
}

// Directories outside the enum get no type, and mark lists them.
func TestMarkReportsUnmappedDirectories(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  type: {type: string, derive: parent-dir, enum: [procedure], map: {guides: procedure}}\n")
	mustWrite(t, filepath.Join(dir, "guides", "a.md"), "# A\n")
	mustWrite(t, filepath.Join(dir, "deploy", "dev", "b.md"), "# B\n")
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var errOut bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"mark", "--source", "file", "--write"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join("guides", "a.md"))
	b, _ := os.ReadFile(filepath.Join("deploy", "dev", "b.md"))
	if !strings.Contains(string(a), "type: procedure") || strings.Contains(string(b), "type:") {
		t.Fatalf("a:\n%s\nb:\n%s", a, b)
	}
	if !strings.Contains(errOut.String(), "deploy/dev") {
		t.Fatalf("expected deploy/dev listed as unmapped, got:\n%s", errOut.String())
	}
}
