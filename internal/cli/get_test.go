// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetSingleKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	_ = os.WriteFile(p, []byte("---\ntitle: Hello\n---\nbody\n"), 0o644)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"get", p, "title"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "Hello" {
		t.Fatalf("get title = %q", out.String())
	}
}
