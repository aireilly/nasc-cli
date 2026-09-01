// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/aireilly/nasc-cli/internal/infer"
	"github.com/spf13/cobra"
)

func newSchemaInferCmd() *cobra.Command {
	var write bool
	var threshold float64
	c := &cobra.Command{
		Use:   "infer",
		Short: "Report observed frontmatter fields across the corpus.",
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, _, err := loadDocs(cmd)
			if err != nil {
				return err
			}
			stats := infer.Observe(docs)
			if write {
				dest := filepath.Join(".nasc", "schema.yaml")
				if err := os.MkdirAll(".nasc", 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(dest, infer.ToSchema(stats, threshold), 0o644); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote candidate %s\n", dest)
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "FIELD\tPRESENT\tTYPES\tEXAMPLE")
			for _, s := range stats {
				fmt.Fprintf(tw, "%s\t%.1f%%\t%s\t%s\n", s.Name, s.Present*100, join(s.Types), truncate(s.Example, 30))
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&write, "write", false, "write a candidate schema.yaml")
	c.Flags().Float64Var(&threshold, "threshold", 0.9, "presence at which a field is marked required")
	return c
}

func join(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}
