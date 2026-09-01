// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkFiltersAndIgnores(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/a.md", "a")
	writeFile(t, dir, "docs/b.markdown", "b")
	writeFile(t, dir, "README.mdx", "c")
	writeFile(t, dir, "notes.txt", "skip")
	writeFile(t, dir, "node_modules/pkg/readme.md", "skip")
	writeFile(t, dir, "vendor/x.md", "skip")
	writeFile(t, dir, ".gitignore", "docs/b.markdown\n")

	got, err := Walk(Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"README.mdx", "docs/a.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Walk = %v, want %v", got, want)
	}
}
