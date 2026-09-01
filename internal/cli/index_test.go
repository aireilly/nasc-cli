// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
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
