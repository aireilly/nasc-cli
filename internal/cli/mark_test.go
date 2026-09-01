// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkWriteIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"),
		"version: 1\nfields:\n  id: {type: string}\n  title: {type: string}\n  type: {type: string}\n")
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
		"version: 1\nfields:\n  id: {type: string}\n  title: {type: string}\n  type: {type: string}\n")
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
	cmd.SetArgs([]string{"mark", "--tier", "file", "--write"})
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
		"version: 1\nfields:\n  id: {type: string}\n  title: {type: string}\n  type: {type: string}\n")
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
	cmd.SetArgs([]string{"mark", "--tier", "file", "--write"})
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
	cmd.SetArgs([]string{"mark", "--tier", "file", "--write"})
	_ = cmd.Execute()
	after, _ := os.ReadFile(filepath.Join("docs", "auth.md"))
	if string(before) == string(after) {
		return ""
	}
	return string(after)
}
