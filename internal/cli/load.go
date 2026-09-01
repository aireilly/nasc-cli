// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/parse"
	"github.com/aireilly/nasc-cli/internal/presets"
	"github.com/aireilly/nasc-cli/internal/scan"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/spf13/cobra"
)

// defaultPreset is the schema nasc uses when a repo has no .nasc/schema.yaml,
// so mark, index, and validate work with zero configuration. Run
// `nasc schema init` to materialize it and start diverging.
const defaultPreset = "agent-context"

// loadSchema reads .nasc/schema.yaml, falling back to the built-in default
// preset when no schema file exists. It reports whether the default was used,
// so callers that require an explicit, checked-in schema (index --strict) can
// still refuse the implicit one. A schema file that exists but is invalid is
// surfaced as an error rather than masked by the default.
func loadSchema() (s *schema.Schema, isDefault bool, err error) {
	path := filepath.Join(".nasc", "schema.yaml")
	s, err = schema.Load(path)
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

// loadDocs walks the repo and parses every markdown file, printing a warning
// summary to stderr.
func loadDocs(cmd *cobra.Command) ([]model.Doc, *schema.Config, error) {
	cfg, err := schema.LoadConfig(filepath.Join(".nasc", "config.yaml"))
	if err != nil {
		return nil, nil, err
	}
	paths, err := scan.Walk(scan.Options{Root: cfg.Root, Include: cfg.Include, Exclude: cfg.Exclude})
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
