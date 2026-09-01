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

func TestValidateFailsOnBadCorpus(t *testing.T) {
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
	cmd.SetArgs([]string{"validate", "--format", "jsonl"})
	err := cmd.Execute()

	var ec ExitError
	if !errors.As(err, &ec) || ec.Code != 3 {
		t.Fatalf("expected exit code 3, got %v", err)
	}
}

func TestValidateWarnOnlyPassesGate(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, ".nasc", "schema.yaml"), `version: 1
fields:
  title: {type: string, required: true}
rules:
  - name: title-length
    when: exists(title)
    assert: length(title) >= 10
    severity: warn
    message: title is quite short
`)
	mustWrite(t, filepath.Join(dir, "docs", "a.md"), "---\ntitle: Hi\n---\nBody.\n")

	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "--format", "jsonl"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for warn-only findings, got %v", err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
