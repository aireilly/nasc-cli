// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRootCmdShowsHelp(t *testing.T) {
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "nasc") {
		t.Fatalf("help missing binary name, got: %q", out.String())
	}
}

func TestLoadDocsWithConfig(t *testing.T) {
	tmpdir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldwd)

	if err := os.MkdirAll(".nasc", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(".nasc/config.yaml", []byte("root: ."), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := os.WriteFile("valid.md", []byte("---\ntitle: Test\n---\nBody"), 0o644); err != nil {
		t.Fatalf("write valid.md: %v", err)
	}
	if err := os.WriteFile("warn.md", []byte("---\ntitle: Test\nno-close:"), 0o644); err != nil {
		t.Fatalf("write warn.md: %v", err)
	}

	cmd := NewRootCmd()
	var errout bytes.Buffer
	cmd.SetErr(&errout)

	docs, cfg, err := loadDocs(cmd)
	if err != nil {
		t.Fatalf("loadDocs: %v", err)
	}
	if len(docs) < 1 {
		t.Fatalf("expected at least 1 doc, got %d", len(docs))
	}
	if cfg == nil {
		t.Fatalf("config is nil")
	}
	if cfg.Root != "." {
		t.Fatalf("config.Root = %q, want .", cfg.Root)
	}

	stderr := errout.String()
	if len(stderr) > 0 && !strings.Contains(stderr, "warning") {
		t.Logf("warning output: %q", stderr)
	}
}
