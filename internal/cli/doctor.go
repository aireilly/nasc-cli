// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aireilly/nasc-cli/internal/gitmeta"
	"github.com/aireilly/nasc-cli/internal/scan"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/aireilly/nasc-cli/internal/ssg"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report the environment nasc sees.",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			cfg, cerr := loadConfig()
			if cerr != nil {
				fmt.Fprintf(w, "config error: %v\n", cerr)
				cfg = schema.DefaultConfig()
				cfg.Root = projectRoot()
			}
			fmt.Fprintf(w, "project root: %s\n", projectRoot())
			fmt.Fprintf(w, "git available: %t\n", gitmeta.Available(cfg.Root))
			fmt.Fprintf(w, "schema present: %t\n", exists(nascPath("schema.yaml")))
			fmt.Fprintf(w, "config present: %t\n", exists(nascPath("config.yaml")))
			paths, _ := scan.Walk(scan.Options{Root: cfg.Root, Include: cfg.Include, Exclude: cfg.Excludes()})
			fmt.Fprintf(w, "markdown files: %d\n", len(paths))
			fmt.Fprintf(w, "llm command set: %t\n", cfg.Mark.LLMCmd != "")
			for _, site := range ssg.Detect(cfg.Root) {
				keys := make([]string, 0, len(site.Keys))
				for k := range site.Keys {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				fmt.Fprintf(w, "site generator: %s in %s (mark leaves %s alone there; rename with `key:` in the schema)\n",
					site.Name, site.Dir, strings.Join(keys, ", "))
			}
			return nil
		},
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
