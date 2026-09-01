// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaInitWritesFile(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"schema", "init", "--preset", "minimal"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".nasc", "schema.yaml")); err != nil {
		t.Fatalf("schema not written: %v", err)
	}
}

func TestSchemaInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// First write: minimal preset
	cmd1 := NewRootCmd()
	cmd1.SetArgs([]string{"schema", "init", "--preset", "minimal"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first init: %v", err)
	}
	schemaPath := filepath.Join(dir, ".nasc", "schema.yaml")
	initialContent, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read initial schema: %v", err)
	}
	t.Logf("initial file size: %d", len(initialContent))

	// Second write without --force: should refuse
	cmd2 := NewRootCmd()
	cmd2.SetArgs([]string{"schema", "init", "--preset", "agent-context"})
	err = cmd2.Execute()
	if err == nil {
		t.Fatalf("expected error on second init without --force, got nil")
	}
	t.Logf("second init error (expected): %v", err)

	// Verify file unchanged
	currentContent, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema after refusal: %v", err)
	}
	t.Logf("file size after refusal: %d", len(currentContent))
	if string(currentContent) != string(initialContent) {
		t.Fatalf("file was modified despite refusal:\ninitial len %d:\n%s\ncurrent len %d:\n%s", len(initialContent), initialContent, len(currentContent), currentContent)
	}

	// Third write with --force: should succeed
	cmd3 := NewRootCmd()
	cmd3.SetArgs([]string{"schema", "init", "--preset", "agent-context", "--force"})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("init with --force: %v", err)
	}

	// Verify file changed
	forcedContent, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema after --force: %v", err)
	}
	t.Logf("file size after --force: %d", len(forcedContent))
	if string(forcedContent) == string(initialContent) {
		t.Fatalf("file was not overwritten with --force")
	}
}
