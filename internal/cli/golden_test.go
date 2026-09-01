// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func runInVault(t *testing.T, args ...string) string {
	t.Helper()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	vault, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "vault"))
	_ = os.Chdir(vault)
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	_ = cmd.Execute()
	return out.String()
}

func TestGoldenIndex(t *testing.T) {
	got := runInVault(t, "index")
	golden, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "golden", "index.md"))
	if *update {
		_ = os.WriteFile(golden, []byte(got), 0o644)
		return
	}
	want, _ := os.ReadFile(golden)
	if got != string(want) {
		t.Fatalf("index golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGoldenInfer(t *testing.T) {
	got := runInVault(t, "schema", "infer")
	golden, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "golden", "infer.txt"))
	if *update {
		_ = os.WriteFile(golden, []byte(got), 0o644)
		return
	}
	want, _ := os.ReadFile(golden)
	if got != string(want) {
		t.Fatalf("infer golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
