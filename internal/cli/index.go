// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aireilly/nasc-cli/internal/edit"
	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/render"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/spf13/cobra"
)

// autoLoaded names files that agents read at the start of every task, so
// their size costs tokens on every run.
var autoLoaded = map[string]bool{"AGENTS.md": true, "CLAUDE.md": true, "GEMINI.md": true}

// indexFile is one file the index command writes or checks.
type indexFile struct {
	path    string
	content []byte
}

func newIndexCmd() *cobra.Command {
	var output, tmplPath, pointer string
	var strict, check, nested bool
	var maxLines int
	c := &cobra.Command{
		Use:   "index",
		Short: "Generate an AGENTS.md-style navigation index.",
		Long: `Generate an AGENTS.md-style navigation index.

The index goes to --output, or to index.output in .nasc/config.yaml, or to
stdout when neither is set. Pass --output - to force stdout. Writing to a file
replaces only the region between nasc's markers and keeps the rest. Links are
relative to the file they are written in.

Agents load AGENTS.md on every task, so a long index there costs tokens every
time. Two options keep it small:

  --pointer AGENTS.md   writes the full index to --output (for example
                        docs/INDEX.md) and only a short pointer to it in
                        AGENTS.md.
  --nested              writes one index per top-level directory, named like
                        --output, and links them from the top-level file.

nasc warns when the index in AGENTS.md, CLAUDE.md, or GEMINI.md runs past
--max-lines (default 150; 0 turns the warning off).

--check writes nothing. It exits 3 when any output file is missing or would
change, so CI can catch an index that has drifted from the docs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, cfg, s, isDefault, err := loadAll(cmd)
			if err != nil {
				return err
			}
			// --strict gates CI, so it demands a checked-in schema and refuses
			// the implicit default; run `nasc schema init` to opt in.
			if strict && isDefault {
				return ExitError{Code: 2, Msg: "no schema: --strict needs a schema; run `nasc schema init` first"}
			}
			// The built-in default is nasc's opinion, not the project's, so it
			// must not segregate docs into an Unmarked bucket.
			if isDefault {
				s = nil
			}
			fromConfig := func(flag, cfgVal string) string {
				if flag != "" || cfgVal == "" {
					return flag
				}
				return filepath.Join(projectRoot(), cfgVal)
			}
			dest := fromConfig(output, cfg.Index.Output)
			if dest == "-" {
				dest = ""
			}
			ptr := fromConfig(pointer, cfg.Index.Pointer)
			nested = nested || cfg.Index.Nested
			if !cmd.Flags().Changed("max-lines") && cfg.Index.MaxLines != nil {
				maxLines = *cfg.Index.MaxLines
			}
			tpath := fromConfig(tmplPath, cfg.Index.Template)
			var tmpl string
			if tpath != "" {
				b, rerr := os.ReadFile(tpath)
				if rerr != nil {
					return ExitError{Code: 2, Msg: "index template: " + rerr.Error()}
				}
				tmpl = string(b)
			}

			switch {
			case check && dest == "":
				return ExitError{Code: 2, Msg: "--check needs an output file: pass --output or set index.output in .nasc/config.yaml"}
			case (ptr != "" || nested) && dest == "":
				return ExitError{Code: 2, Msg: "--pointer and --nested need an output file: pass --output or set index.output"}
			case ptr != "" && nested:
				return ExitError{Code: 2, Msg: "use --pointer or --nested, not both"}
			}

			if dest == "" {
				data := render.BuildIndex(docs, s)
				if err := render.Index(cmd.OutOrStdout(), data, tmpl); err != nil {
					return err
				}
				return strictResult(strict, data)
			}

			files, data, err := planIndex(docs, s, cfg.Root, dest, ptr, nested, tmpl)
			if err != nil {
				return err
			}
			stderr := cmd.ErrOrStderr()
			for _, f := range files {
				if n := render.Lines(f.content); maxLines > 0 && autoLoaded[filepath.Base(f.path)] && n > maxLines {
					fmt.Fprintf(stderr, "nasc: the index in %s is %d lines. Agents load %s on every task, so each line costs tokens every time. Use --pointer or --nested to keep it short.\n",
						f.path, n, filepath.Base(f.path))
				}
			}

			var stale []string
			for _, f := range files {
				existing, rerr := os.ReadFile(f.path)
				if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
					return rerr
				}
				merged := render.Merge(existing, f.content)
				if bytes.Equal(existing, merged) {
					continue
				}
				if check {
					stale = append(stale, f.path)
					continue
				}
				mode := os.FileMode(0o644)
				if info, serr := os.Stat(f.path); serr == nil {
					mode = info.Mode()
				}
				if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
					return err
				}
				if werr := edit.WriteAtomic(f.path, merged, mode); werr != nil {
					return werr
				}
			}
			if check {
				if len(stale) > 0 {
					return ExitError{Code: 3, Msg: "out of date: " + strings.Join(stale, ", ") + "; run `nasc index` and commit the result"}
				}
				fmt.Fprintln(stderr, "nasc: index is up to date")
			}
			return strictResult(strict, data)
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "write to a file (- for stdout)")
	c.Flags().StringVar(&tmplPath, "template", "", "custom Go text/template")
	c.Flags().StringVar(&pointer, "pointer", "", "write only a short pointer to the --output file into this file, e.g. AGENTS.md")
	c.Flags().BoolVar(&nested, "nested", false, "write one index per top-level directory, linked from --output")
	c.Flags().IntVar(&maxLines, "max-lines", 150, "warn when an index in AGENTS.md, CLAUDE.md or GEMINI.md is longer (0 disables)")
	c.Flags().BoolVar(&check, "check", false, "write nothing; exit 3 if any output file is out of date")
	c.Flags().BoolVar(&strict, "strict", false, "exit 3 if any doc is missing schema-required fields (needs a schema)")
	return c
}

func strictResult(strict bool, data render.IndexData) error {
	if strict && len(data.Unmarked) > 0 {
		return ExitError{Code: 3, Msg: "unmarked documents present"}
	}
	return nil
}

// planIndex renders every file an index run produces. It returns the files
// and the index data for the whole corpus (used by --strict).
func planIndex(docs []model.Doc, s *schema.Schema, root, dest, ptr string, nested bool, tmpl string) ([]indexFile, render.IndexData, error) {
	all := render.BuildIndex(docs, s)
	var files []indexFile
	renderTo := func(file string, ds []model.Doc, subs []render.Subindex) error {
		data := render.BuildIndex(relinked(ds, root, filepath.Dir(file)), s)
		data.Subindexes = subs
		var buf bytes.Buffer
		if err := render.Index(&buf, data, tmpl); err != nil {
			return err
		}
		files = append(files, indexFile{path: file, content: buf.Bytes()})
		return nil
	}

	if !nested {
		if err := renderTo(dest, docs, nil); err != nil {
			return nil, all, err
		}
		if ptr != "" {
			files = append(files, indexFile{path: ptr, content: render.Pointer(linkFrom(filepath.Dir(ptr), dest))})
		}
		return files, all, nil
	}

	// Nested: docs at the top level stay in dest, every top-level directory
	// gets its own file with the same name, and dest links to each.
	byDir := map[string][]model.Doc{}
	var top []model.Doc
	for _, d := range docs {
		if i := strings.IndexByte(d.Path, '/'); i >= 0 {
			byDir[d.Path[:i]] = append(byDir[d.Path[:i]], d)
		} else {
			top = append(top, d)
		}
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	base := filepath.Base(dest)
	var subs []render.Subindex
	for _, dir := range dirs {
		file := filepath.Join(root, dir, base)
		if err := renderTo(file, byDir[dir], nil); err != nil {
			return nil, all, err
		}
		subs = append(subs, render.Subindex{Dir: dir, Path: linkFrom(filepath.Dir(dest), file), Count: len(byDir[dir])})
	}
	if err := renderTo(dest, top, subs); err != nil {
		return nil, all, err
	}
	return files, all, nil
}

// relinked returns copies of docs whose Path is relative to dir, so links in
// an index written there resolve.
func relinked(docs []model.Doc, root, dir string) []model.Doc {
	out := make([]model.Doc, len(docs))
	for i, d := range docs {
		d.Path = linkFrom(dir, filepath.Join(root, filepath.FromSlash(d.Path)))
		out[i] = d
	}
	return out
}

// linkFrom returns target as a slash-separated path relative to dir.
func linkFrom(dir, target string) string {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return path.Clean(filepath.ToSlash(rel))
}
