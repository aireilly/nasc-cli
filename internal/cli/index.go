// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aireilly/nasc-cli/internal/edit"
	"github.com/aireilly/nasc-cli/internal/render"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	var output, tmplPath string
	var strict bool
	c := &cobra.Command{
		Use:   "index",
		Short: "Generate an AGENTS.md-style navigation index.",
		Long: `Generate an AGENTS.md-style navigation index.

The index goes to --output, or to index.output in .nasc/config.yaml, or to
stdout when neither is set. Pass --output - to force stdout. Writing to a file
replaces only the region between nasc's markers and keeps the rest.`,
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
			// must not segregate docs into an Unmarked bucket. Only a schema the
			// project checked in does that.
			if isDefault {
				s = nil
			}
			dest := output
			if dest == "" && cfg.Index.Output != "" {
				dest = filepath.Join(projectRoot(), cfg.Index.Output)
			}
			if dest == "-" {
				dest = ""
			}
			tpath := tmplPath
			if tpath == "" && cfg.Index.Template != "" {
				tpath = filepath.Join(projectRoot(), cfg.Index.Template)
			}
			var tmpl string
			if tpath != "" {
				b, rerr := os.ReadFile(tpath)
				if rerr != nil {
					return ExitError{Code: 2, Msg: "index template: " + rerr.Error()}
				}
				tmpl = string(b)
			}
			data := render.BuildIndex(docs, s)

			if dest != "" {
				var buf bytes.Buffer
				if err := render.Index(&buf, data, tmpl); err != nil {
					return err
				}
				existing, rerr := os.ReadFile(dest)
				if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
					return rerr
				}
				merged := render.Merge(existing, buf.Bytes())
				if !bytes.Equal(existing, merged) {
					mode := os.FileMode(0o644)
					if info, serr := os.Stat(dest); serr == nil {
						mode = info.Mode()
					}
					if werr := edit.WriteAtomic(dest, merged, mode); werr != nil {
						return werr
					}
				}
			} else if err := render.Index(cmd.OutOrStdout(), data, tmpl); err != nil {
				return err
			}
			if strict && len(data.Unmarked) > 0 {
				return ExitError{Code: 3, Msg: "unmarked documents present"}
			}
			return nil
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "write to a file (- for stdout)")
	c.Flags().StringVar(&tmplPath, "template", "", "custom Go text/template")
	c.Flags().BoolVar(&strict, "strict", false, "exit 3 if any doc is missing schema-required fields (needs a schema)")
	return c
}
