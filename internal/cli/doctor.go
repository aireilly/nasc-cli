// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aireilly/nasc-cli/internal/gitmeta"
	"github.com/aireilly/nasc-cli/internal/scan"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report the environment nasc sees.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := schema.LoadConfig(filepath.Join(".nasc", "config.yaml"))
			if cfg == nil {
				cfg = schema.DefaultConfig()
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "git available: %t\n", gitmeta.Available(cfg.Root))
			fmt.Fprintf(w, "schema present: %t\n", exists(filepath.Join(".nasc", "schema.yaml")))
			fmt.Fprintf(w, "config present: %t\n", exists(filepath.Join(".nasc", "config.yaml")))
			paths, _ := scan.Walk(scan.Options{Root: cfg.Root})
			fmt.Fprintf(w, "markdown files: %d\n", len(paths))
			fmt.Fprintf(w, "llm command set: %t\n", cfg.Mark.LLMCmd != "")
			return nil
		},
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
