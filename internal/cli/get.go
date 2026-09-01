// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/aireilly/nasc-cli/internal/parse"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <path> [key]",
		Short: "Read one frontmatter value from a document.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return ExitError{Code: 2, Msg: err.Error()}
			}
			d := parse.File(args[0], data)
			if len(args) == 1 {
				var keys []string
				for k := range d.Fields {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", k, d.Fields[k].String())
				}
				return nil
			}
			v, ok := d.Field(args[1])
			if !ok {
				return ExitError{Code: 1, Msg: "no such key: " + args[1]}
			}
			fmt.Fprintln(cmd.OutOrStdout(), v.String())
			return nil
		},
	}
}
