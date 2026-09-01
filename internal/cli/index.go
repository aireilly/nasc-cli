// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"os"
	"path/filepath"

	"github.com/aireilly/nasc-cli/internal/render"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	var output, tmplPath string
	var strict bool
	c := &cobra.Command{
		Use:   "index",
		Short: "Generate an AGENTS.md-style navigation index.",
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, _, err := loadDocs(cmd)
			if err != nil {
				return err
			}
			s, serr := schema.Load(filepath.Join(".nasc", "schema.yaml"))
			// A doc is "unmarked" relative to the schema's required fields, so
			// --strict is meaningless without one. Fail loudly rather than
			// silently passing, mirroring validate.
			if strict && serr != nil {
				return ExitError{Code: 2, Msg: "no schema: --strict needs a schema; run `nasc schema init` first (" + serr.Error() + ")"}
			}
			var tmpl string
			if tmplPath != "" {
				b, rerr := os.ReadFile(tmplPath)
				if rerr != nil {
					return ExitError{Code: 2, Msg: rerr.Error()}
				}
				tmpl = string(b)
			}
			data := render.BuildIndex(docs, s)

			w := cmd.OutOrStdout()
			if output != "" {
				f, ferr := os.Create(output)
				if ferr != nil {
					return ferr
				}
				defer f.Close()
				w = f
			}
			if err := render.Index(w, data, tmpl); err != nil {
				return err
			}
			if strict && len(data.Unmarked) > 0 {
				return ExitError{Code: 3, Msg: "unmarked documents present"}
			}
			return nil
		},
	}
	c.Flags().StringVar(&output, "output", "", "write to a file instead of stdout")
	c.Flags().StringVar(&tmplPath, "template", "", "custom Go text/template")
	c.Flags().BoolVar(&strict, "strict", false, "exit 3 if any doc is missing schema-required fields (needs a schema)")
	return c
}
