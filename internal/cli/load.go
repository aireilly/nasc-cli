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
	"strings"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/parse"
	"github.com/aireilly/nasc-cli/internal/presets"
	"github.com/aireilly/nasc-cli/internal/render"
	"github.com/aireilly/nasc-cli/internal/scan"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/spf13/cobra"
)

// defaultPreset is the schema nasc uses when a repo has no .nasc/schema.yaml,
// so mark, index, and validate work with zero configuration. Run
// `nasc schema init` to materialize it and start diverging.
const defaultPreset = "agent-context"

// rootFlag holds the value of the global -C flag.
var rootFlag string

// projectRoot returns the directory nasc treats as the project root, as a
// path usable from the current directory. -C wins. Otherwise nasc walks up
// from the current directory and stops at the first directory holding .nasc/,
// or failing that at the git work tree top. Outside any repo it is ".".
func projectRoot() string {
	if rootFlag != "" {
		return rootFlag
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := cwd; ; {
		if isDir(filepath.Join(dir, ".nasc")) {
			return relTo(cwd, dir)
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return relTo(cwd, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func relTo(base, target string) string {
	if r, err := filepath.Rel(base, target); err == nil {
		return r
	}
	return target
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// nascPath returns a path inside the project's .nasc directory.
func nascPath(name string) string {
	return filepath.Join(projectRoot(), ".nasc", name)
}

// loadSchema reads .nasc/schema.yaml, falling back to the built-in default
// preset when no schema file exists. It reports whether the default was used,
// so callers that require an explicit, checked-in schema (index --strict) can
// still refuse the implicit one. A schema file that exists but is invalid is
// surfaced as an error rather than masked by the default.
func loadSchema() (s *schema.Schema, isDefault bool, err error) {
	s, err = schema.Load(nascPath("schema.yaml"))
	if err == nil {
		return s, false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, false, err
	}
	data, perr := presets.Get(defaultPreset)
	if perr != nil {
		return nil, false, perr
	}
	s, perr = schema.Parse(data)
	if perr != nil {
		return nil, false, perr
	}
	return s, true, nil
}

// loadConfig reads .nasc/config.yaml and rewrites Root so it resolves from
// the current directory.
func loadConfig() (*schema.Config, error) {
	cfg, err := schema.LoadConfig(nascPath("config.yaml"))
	if err != nil {
		return nil, ExitError{Code: 2, Msg: "config error: " + err.Error()}
	}
	if !filepath.IsAbs(cfg.Root) {
		cfg.Root = filepath.Join(projectRoot(), cfg.Root)
	}
	return cfg, nil
}

// loadDocs walks the repo and parses every markdown file, printing a warning
// summary to stderr.
func loadDocs(cmd *cobra.Command) ([]model.Doc, *schema.Config, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	paths, err := scan.Walk(scan.Options{Root: cfg.Root, Include: cfg.Include, Exclude: cfg.Excludes()})
	if err != nil {
		return nil, nil, err
	}
	var docs []model.Doc
	var readErrors []string
	parseWarnings := 0
	for _, rel := range paths {
		data, rerr := os.ReadFile(filepath.Join(cfg.Root, filepath.FromSlash(rel)))
		if rerr != nil {
			readErrors = append(readErrors, fmt.Sprintf("nasc: cannot read %s: %v", rel, rerr))
			continue
		}
		// A file nasc generated in full (an index written to a file of its
		// own) is output, not documentation. Indexing it would list the
		// index in itself.
		if bytes.HasPrefix(data, []byte(render.BeginMarker)) {
			continue
		}
		d := parse.File(rel, data)
		parseWarnings += len(d.Warnings)
		docs = append(docs, d)
	}
	for _, errMsg := range readErrors {
		fmt.Fprintln(cmd.ErrOrStderr(), errMsg)
	}
	if parseWarnings > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "nasc: %d warning(s) while parsing\n", parseWarnings)
	}
	return docs, cfg, nil
}

// loadAll loads config, docs and schema together, with the docs' derived view
// following any renamed keys in the schema.
func loadAll(cmd *cobra.Command) (docs []model.Doc, cfg *schema.Config, s *schema.Schema, isDefault bool, err error) {
	docs, cfg, err = loadDocs(cmd)
	if err != nil {
		return
	}
	s, isDefault, err = loadSchema()
	if err != nil {
		err = ExitError{Code: 2, Msg: "schema error: " + err.Error()}
		return
	}
	applyKeys(docs, s)
	return
}

// applyKeys re-derives a doc's title, type, tags, and paths from renamed
// frontmatter keys. parse.File reads the default key names; a schema that
// stores a field under another key (to stay clear of a site generator) needs
// the doc's derived view to follow it.
func applyKeys(docs []model.Doc, s *schema.Schema) {
	if s == nil {
		return
	}
	for i := range docs {
		d := &docs[i]
		if k := s.Key("title"); k != "title" {
			if v, ok := d.Field(k); ok && !v.IsZero() {
				d.Title = v.String()
			}
		}
		if k := s.Key("type"); k != "type" {
			// The site generator's own `type` must not group the index;
			// without nasc's key, fall back to the parent directory as
			// parse.File does.
			d.Type = ""
			if v, ok := d.Field(k); ok && !v.IsZero() {
				d.Type = v.String()
			} else if dir := path.Dir(d.Path); dir != "." {
				d.Type = path.Base(dir)
			}
		}
		if k := s.Key("tags"); k != "tags" {
			d.Tags = listValue(d, k)
		}
		if k := s.Key("paths"); k != "paths" {
			d.Paths = listValue(d, k)
		}
	}
}

func listValue(d *model.Doc, key string) []string {
	v, ok := d.Field(key)
	if !ok || v.IsZero() {
		return nil
	}
	if v.Kind == model.KindList {
		return v.List()
	}
	if s := strings.TrimSpace(v.String()); s != "" {
		return []string{s}
	}
	return nil
}
