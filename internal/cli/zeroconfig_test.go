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

// With no .nasc/schema.yaml, validate falls back to the built-in default and
// enforces it (a bare doc is missing required fields), rather than refusing to
// run for lack of a schema. A note on stderr says the default is in use.
func TestValidateWithoutSchemaUsesDefault(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "no frontmatter here\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"validate", "--format", "jsonl"})
	err := cmd.Execute()

	var ec ExitError
	if !errors.As(err, &ec) || ec.Code != 3 {
		t.Fatalf("expected exit code 3 validating against the default, got %v", err)
	}
	if !strings.Contains(errOut.String(), "built-in") {
		t.Fatalf("expected a note that the default schema is in use, stderr:\n%s", errOut.String())
	}
}

// With no schema, index must stay lenient: the built-in default is nasc's
// opinion, not the project's, so docs are grouped by type rather than dumped
// into an Unmarked bucket for lacking required fields.
func TestIndexWithoutSchemaGroupsLeniently(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "guides", "a.md"), "# Alpha\n\nbody\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"index"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("index without a schema should succeed, got %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "## guides") || !strings.Contains(got, "Alpha") {
		t.Fatalf("expected the doc grouped under its type, got:\n%s", got)
	}
	if strings.Contains(got, "## Unmarked") {
		t.Fatalf("the implicit default must not segregate docs as Unmarked, got:\n%s", got)
	}
}

// The llm source does real work with no schema present: the default supplies the
// llm-derived fields, so a bare agent command can fill a description.
func TestMarkLLMWithoutSchemaUsesDefault(t *testing.T) {
	stub, err := filepath.Abs(filepath.Join("..", "..", "testdata", "stub-llm", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "guides", "a.md"), "# Alpha\n\nbody\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"mark", "--source", "llm", "--llm-cmd", "go run " + stub, "--write"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mark llm without a schema should succeed, got %v", err)
	}
	got, _ := os.ReadFile(filepath.Join("guides", "a.md"))
	if !strings.Contains(string(got), "description:") {
		t.Fatalf("expected the llm source to add a description, got:\n%s", got)
	}
}
