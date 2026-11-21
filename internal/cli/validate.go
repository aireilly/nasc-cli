// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"

	"github.com/aireilly/nasc-cli/internal/render"
	"github.com/aireilly/nasc-cli/internal/validate"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var format string
	var minSeverity string
	c := &cobra.Command{
		Use:   "validate",
		Short: "Enforce the schema across the corpus. Fails CI on error findings.",
		Long: `Enforce the schema across the corpus.

With a checked-in .nasc/schema.yaml, error findings exit 3 and fail CI.
Without one, nasc checks against its built-in default and reports every
finding as a warning, so adding nasc to CI never fails a build until the
project commits a schema of its own.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, cfg, s, isDefault, err := loadAll(cmd)
			if err != nil {
				return err
			}
			findings := validate.Check(docs, s, cfg.Root)
			if isDefault {
				fmt.Fprintf(cmd.ErrOrStderr(), "nasc: no .nasc/schema.yaml; checking against the built-in %q default as warnings only (run `nasc schema init` to enforce)\n", defaultPreset)
				for i := range findings {
					findings[i].Severity = "warn"
				}
			}
			findings = filterSeverity(findings, minSeverity)

			f := render.Detect(cmd.OutOrStdout(), format, cfg.Output.DefaultFormat)
			if err := render.Findings(cmd.OutOrStdout(), findings, f); err != nil {
				return err
			}
			if hasErrors(findings) {
				return ExitError{Code: 3, Msg: "validation failed"}
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "", "output format: table|json|jsonl|paths")
	c.Flags().BoolVar(new(bool), "json", false, "shorthand for --format json")
	c.Flags().StringVar(&minSeverity, "severity", "warn", "minimum severity to report: warn|error")
	// Map --json to --format json.
	c.PreRunE = func(cmd *cobra.Command, args []string) error {
		if j, _ := cmd.Flags().GetBool("json"); j && format == "" {
			format = "json"
		}
		return nil
	}
	return c
}

func filterSeverity(fs []validate.Finding, min string) []validate.Finding {
	if min != "error" {
		return fs
	}
	var out []validate.Finding
	for _, f := range fs {
		if f.Severity == "error" {
			out = append(out, f)
		}
	}
	return out
}

func hasErrors(fs []validate.Finding) bool {
	for _, f := range fs {
		if f.Severity == "error" {
			return true
		}
	}
	return false
}
