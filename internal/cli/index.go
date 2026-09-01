// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"os"

	"github.com/aireilly/nasc-cli/internal/render"
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
			s, isDefault, serr := loadSchema()
			if serr != nil {
				return ExitError{Code: 2, Msg: "schema error: " + serr.Error()}
			}
			// A doc is "unmarked" relative to the schema's required fields.
			// --strict gates CI, so it demands a checked-in schema and refuses
			// the implicit default; run `nasc schema init` to opt in.
			if strict && isDefault {
				return ExitError{Code: 2, Msg: "no schema: --strict needs a schema; run `nasc schema init` first"}
			}
			// The built-in default is nasc's opinion, not the project's, so it
			// must not segregate docs into an Unmarked bucket. Only a schema the
			// project checked in does that. With the implicit default, group
			// everything nasc can place.
			if isDefault {
				s = nil
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
