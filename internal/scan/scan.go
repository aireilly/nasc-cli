// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// Options configures a walk.
type Options struct {
	Root    string
	Include []string // reserved for config-driven globs; extension filter is default
	Exclude []string
}

// alwaysSkip names non-dot directories that never hold documentation. Dot
// directories are handled separately: every directory whose name starts with a
// dot is skipped, so tooling and agent config (.git, .github, .nasc, .claude,
// .cursor, .vscode, and the like) stays out of the corpus.
var alwaysSkip = map[string]bool{
	"node_modules": true, "vendor": true,
}

// alwaysSkipFile names markdown files that are agent instructions or skill
// definitions rather than documentation. nasc never marks or indexes them, so a
// project's CLAUDE.md and any SKILL.md stay under human control and the index
// output file never lists itself.
var alwaysSkipFile = map[string]bool{
	"AGENTS.md": true, "CLAUDE.md": true, "GEMINI.md": true, "SKILL.md": true,
}

var mdExt = map[string]bool{".md": true, ".markdown": true, ".mdx": true}

// Walk returns sorted repo-relative markdown paths under opts.Root.
func Walk(opts Options) ([]string, error) {
	root := opts.Root
	if root == "" {
		root = "."
	}
	gi := loadIgnore(filepath.Join(root, ".gitignore"), filepath.Join(root, ".nascignore"))

	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // never abort the whole walk on one bad entry
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if alwaysSkip[d.Name()] || strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			if gi != nil && gi.MatchesPath(rel+"/") {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // do not follow symlinks
		}
		if !mdExt[strings.ToLower(filepath.Ext(rel))] {
			return nil
		}
		if alwaysSkipFile[d.Name()] {
			return nil // agent instructions and skill files, not docs
		}
		if gi != nil && gi.MatchesPath(rel) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// loadIgnore merges any present ignore files into one matcher.
func loadIgnore(paths ...string) *ignore.GitIgnore {
	var lines []string
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines = append(lines, strings.Split(string(b), "\n")...)
	}
	if len(lines) == 0 {
		return nil
	}
	return ignore.CompileIgnoreLines(lines...)
}
