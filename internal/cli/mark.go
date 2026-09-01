// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aireilly/nasc-cli/internal/edit"
	"github.com/aireilly/nasc-cli/internal/mark"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/spf13/cobra"
)

// afterReadHook, when non-nil, is invoked with the full path of a doc right
// after its original bytes are read and before the write path's conflict
// re-read. It exists only so tests can force a deterministic write conflict
// instead of relying on a wall-clock race; production code leaves it nil.
var afterReadHook func(path string)

func newMarkCmd() *cobra.Command {
	var tierCSV, llmCmd string
	var write, patch, force, dryRun bool
	c := &cobra.Command{
		Use:   "mark",
		Short: "Derive agent-navigation metadata and write it into frontmatter.",
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, cfg, err := loadDocs(cmd)
			if err != nil {
				return err
			}
			s, _ := schema.Load(filepath.Join(".nasc", "schema.yaml"))
			if s == nil {
				s = &schema.Schema{}
			}
			if llmCmd != "" {
				cfg.Mark.LLMCmd = llmCmd
			}
			mark.LLMCmd = cfg.Mark.LLMCmd
			tiers := splitCSV(tierCSV)
			results := mark.Plan(docs, s, cfg.Root, tiers, force)
			if len(results) == 0 {
				return ExitError{Code: 1, Msg: "nothing to mark"}
			}

			doWrite := write || !dryRun

			applied := 0
			for _, r := range results {
				full := filepath.Join(cfg.Root, filepath.FromSlash(r.Path))
				orig, rerr := os.ReadFile(full)
				if rerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "nasc: skip %s: %v\n", r.Path, rerr)
					continue
				}
				updated, eerr := edit.Set(orig, r.Updates, r.Derived)
				if eerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "nasc: skip %s: %v\n", r.Path, eerr)
					continue
				}
				if bytes.Equal(orig, updated) {
					continue
				}
				if doWrite {
					if afterReadHook != nil {
						afterReadHook(full)
					}
					cur, _ := os.ReadFile(full)
					if !bytes.Equal(cur, orig) {
						return ExitError{Code: 5, Msg: "file changed under us: " + r.Path}
					}
					info, _ := os.Stat(full)
					mode := os.FileMode(0o644)
					if info != nil {
						mode = info.Mode()
					}
					if werr := edit.WriteAtomic(full, updated, mode); werr != nil {
						return werr
					}
					applied++
				} else {
					fmt.Fprint(cmd.OutOrStdout(), edit.UnifiedDiff(r.Path, orig, updated))
				}
			}
			if doWrite {
				fmt.Fprintf(cmd.ErrOrStderr(), "nasc: marked %d file(s)\n", applied)
			}
			return nil
		},
	}
	c.Flags().StringVar(&tierCSV, "tier", "file,git", "comma-separated tiers: file,git,llm")
	c.Flags().BoolVar(&write, "write", false, "apply changes in place")
	c.Flags().BoolVar(&patch, "patch", false, "emit a unified diff (default when not writing)")
	c.Flags().BoolVar(&dryRun, "dry-run", true, "print the diff without writing (default)")
	c.Flags().StringVar(&llmCmd, "llm-cmd", "", "subprocess for the llm tier, e.g. 'claude -p'")
	c.Flags().BoolVar(&force, "force", false, "overwrite human-set values (loud, not default)")
	return c
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range bytes.Split([]byte(s), []byte(",")) {
		t := string(bytes.TrimSpace(p))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
