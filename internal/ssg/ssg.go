// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

// Package ssg detects static site generators whose frontmatter keys would
// clash with the fields nasc writes, so mark can leave those keys alone.
package ssg

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Site is one detected site generator.
type Site struct {
	Name string
	// Dir is the repo-relative directory holding the site config ("." for
	// the root). Docs under it are subject to Keys.
	Dir string
	// Keys maps a frontmatter key the generator owns to why it matters.
	Keys map[string]string
}

type generator struct {
	name    string
	markers []string // any one of these files marks the site root
	// also, when set, must exist alongside a marker (to tell Hugo's generic
	// config.toml apart from other tools' config files).
	also []string
	keys map[string]string
}

var generators = []generator{
	{
		name:    "Docusaurus",
		markers: []string{"docusaurus.config.js", "docusaurus.config.ts", "docusaurus.config.mjs", "docusaurus.config.cjs"},
		keys:    map[string]string{"id": "Docusaurus uses id as the doc ID and URL path"},
	},
	{
		name:    "Hugo",
		markers: []string{"hugo.toml", "hugo.yaml", "hugo.yml", "hugo.json"},
		keys:    map[string]string{"type": "Hugo uses type to choose the page layout"},
	},
	{
		name:    "Hugo",
		markers: []string{"config.toml", "config.yaml", "config.yml"},
		also:    []string{"archetypes", "layouts", "themes"},
		keys:    map[string]string{"type": "Hugo uses type to choose the page layout"},
	},
}

// Detect looks for site generator configs at root and one directory below it.
func Detect(root string) []Site {
	var out []Site
	seen := map[string]bool{}
	dirs := []string{"."}
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() || strings.HasPrefix(n, ".") || n == "node_modules" || n == "vendor" {
				continue
			}
			dirs = append(dirs, n)
		}
	}
	for _, dir := range dirs {
		for _, g := range generators {
			if seen[g.name+"\x00"+dir] || !g.present(filepath.Join(root, dir)) {
				continue
			}
			seen[g.name+"\x00"+dir] = true
			out = append(out, Site{Name: g.name, Dir: dir, Keys: g.keys})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir < out[j].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (g generator) present(dir string) bool {
	found := false
	for _, m := range g.markers {
		if isFile(filepath.Join(dir, m)) {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	if len(g.also) == 0 {
		return true
	}
	for _, a := range g.also {
		if _, err := os.Stat(filepath.Join(dir, a)); err == nil {
			return true
		}
	}
	return false
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// Protected returns the keys owned by any detected site whose directory
// contains docPath, mapped to the reason.
func Protected(sites []Site, docPath string) map[string]string {
	var out map[string]string
	for _, s := range sites {
		if s.Dir != "." && docPath != s.Dir && !strings.HasPrefix(docPath, s.Dir+"/") {
			continue
		}
		for k, why := range s.Keys {
			if out == nil {
				out = map[string]string{}
			}
			out[k] = why
		}
	}
	return out
}
