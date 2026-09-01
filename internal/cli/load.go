// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/parse"
	"github.com/aireilly/nasc-cli/internal/scan"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/spf13/cobra"
)

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
