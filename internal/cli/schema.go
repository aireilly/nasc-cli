// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aireilly/nasc-cli/internal/presets"
	"github.com/spf13/cobra"
)

func newSchemaCmd() *cobra.Command {
	c := &cobra.Command{Use: "schema", Short: "Inspect, create, or infer a schema."}
	c.AddCommand(newSchemaInitCmd())
	c.AddCommand(newSchemaInferCmd())
	return c
}

func newSchemaInitCmd() *cobra.Command {
	var preset string
	var force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Write a starter .nasc/schema.yaml from a preset.",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := presets.Get(preset)
			if err != nil {
				return ExitError{Code: 2, Msg: err.Error()}
			}
			dest := filepath.Join(".nasc", "schema.yaml")
			if _, err := os.Stat(dest); err == nil && !force {
				return ExitError{Code: 2, Msg: dest + " exists; pass --force to overwrite"}
			}
			if err := os.MkdirAll(".nasc", 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s from preset %q\n", dest, preset)
			return nil
		},
	}
	c.Flags().StringVar(&preset, "preset", "agent-context", "preset name")
	c.Flags().BoolVar(&force, "force", false, "overwrite an existing schema")
	return c
}
