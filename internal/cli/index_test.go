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
