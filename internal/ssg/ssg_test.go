// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package ssg

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "website", "docusaurus.config.ts"))
	touch(t, filepath.Join(dir, "site", "config.toml"))
	touch(t, filepath.Join(dir, "site", "layouts", "x.html"))
	touch(t, filepath.Join(dir, "tool", "config.toml")) // no Hugo dirs: not Hugo

	got := Detect(dir)
	if len(got) != 2 {
		t.Fatalf("Detect = %+v, want Hugo in site and Docusaurus in website", got)
	}
	if got[0].Name != "Hugo" || got[0].Dir != "site" || got[1].Name != "Docusaurus" || got[1].Dir != "website" {
		t.Fatalf("Detect = %+v", got)
	}

	p := Protected(got, "website/docs/intro.md")
	if _, ok := p["id"]; !ok || len(p) != 1 {
		t.Fatalf("Protected(website doc) = %v, want id", p)
	}
	if p := Protected(got, "docs/other.md"); len(p) != 0 {
		t.Fatalf("Protected(outside any site) = %v, want none", p)
	}
}

func TestDetectRootHugo(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "hugo.toml"))
	got := Detect(dir)
	if len(got) != 1 || got[0].Dir != "." {
		t.Fatalf("Detect = %+v", got)
	}
	if _, ok := Protected(got, "content/a.md")["type"]; !ok {
		t.Fatal("a root Hugo site protects type everywhere")
	}
}
