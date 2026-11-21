// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aireilly/nasc-cli/internal/edit"
	"github.com/aireilly/nasc-cli/internal/mark"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/aireilly/nasc-cli/internal/ssg"
	"github.com/spf13/cobra"
)

// afterReadHook, when non-nil, is invoked with the full path of a doc right
// after its original bytes are read and before the write path's conflict
// re-read. It exists only so tests can force a deterministic write conflict
// instead of relying on a wall-clock race; production code leaves it nil.
var afterReadHook func(path string)

func newMarkCmd() *cobra.Command {
	var sourceCSV, llmCmd, llmTimeout string
	var write, force, dryRun, noCache bool
	var jobs int
	c := &cobra.Command{
		Use:   "mark",
		Short: "Derive agent-navigation metadata and write it into frontmatter.",
		Long: `Derive agent-navigation metadata and write it into frontmatter.

Without --write, mark prints a unified diff and changes nothing.

Exit codes: 0 changes made or shown, 1 nothing to mark, 4 at least one llm
call failed (the other changes are still written or shown), 5 a file changed
during --write.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, cfg, s, _, err := loadAll(cmd)
			if err != nil {
				return err
			}
			if llmCmd != "" {
				cfg.Mark.LLMCmd = llmCmd
			}
			if llmTimeout != "" {
				cfg.Mark.LLMTimeout = llmTimeout
			}
			if jobs > 0 {
				cfg.Mark.Jobs = jobs
			}
			timeout, terr := parseTimeout(cfg.Mark.LLMTimeout)
			if terr != nil {
				return ExitError{Code: 2, Msg: terr.Error()}
			}
			sources := splitCSV(sourceCSV)
			for _, src := range sources {
				if src != "file" && src != "git" && src != "llm" {
					return ExitError{Code: 2, Msg: fmt.Sprintf("unknown source %q (want file, git, or llm)", src)}
				}
				if src == "llm" && cfg.Mark.LLMCmd == "" {
					return ExitError{Code: 2, Msg: "the llm source needs a command: pass --llm-cmd or set mark.llm_cmd in .nasc/config.yaml"}
				}
			}
			var cache *mark.Cache
			if !noCache {
				cache = mark.OpenCache(nascPath(filepath.Join("cache", "llm.json")))
			}
			sites := ssg.Detect(cfg.Root)
			rep := mark.Plan(docs, s, mark.Options{
				Root:       cfg.Root,
				Sources:    sources,
				Force:      force,
				Jobs:       cfg.Mark.Jobs,
				OwnerBlame: cfg.Mark.OwnerFallback == "blame",
				LLM: mark.LLMConfig{
					Cmd:          cfg.Mark.LLMCmd,
					ExcerptBytes: cfg.Mark.LLMExcerptBytes,
					Prompt:       cfg.Mark.LLMPrompt,
					Timeout:      timeout,
					Cache:        cache,
				},
				Protected: func(p string) map[string]string { return ssg.Protected(sites, p) },
			})

			stderr := cmd.ErrOrStderr()
			if cerr := cache.Save(); cerr != nil {
				fmt.Fprintf(stderr, "nasc: could not save the llm cache: %v\n", cerr)
			}
			reportSkipped(stderr, rep.Skipped)
			reportUnmapped(stderr, rep.Unmapped, s)
			reportFailures(stderr, rep.Failures)

			if len(rep.Results) == 0 {
				if len(rep.Failures) > 0 {
					return ExitError{Code: 4, Msg: "llm source failed"}
				}
				return ExitError{Code: 1, Msg: "nothing to mark"}
			}

			doWrite := write || !dryRun
			applied := 0
			for _, r := range rep.Results {
				full := filepath.Join(cfg.Root, filepath.FromSlash(r.Path))
				orig, rerr := os.ReadFile(full)
				if rerr != nil {
					fmt.Fprintf(stderr, "nasc: skip %s: %v\n", r.Path, rerr)
					continue
				}
				updated, eerr := edit.Set(orig, r.Updates)
				if eerr != nil {
					fmt.Fprintf(stderr, "nasc: skip %s: %v\n", r.Path, eerr)
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
				fmt.Fprintf(stderr, "nasc: marked %d file(s)\n", applied)
			}
			if len(rep.Failures) > 0 {
				return ExitError{Code: 4, Msg: "llm source failed for some documents"}
			}
			return nil
		},
	}
	c.Flags().StringVar(&sourceCSV, "source", "file,git", "comma-separated sources: file,git,llm")
	c.Flags().BoolVar(&write, "write", false, "apply changes in place")
	c.Flags().BoolVar(&dryRun, "dry-run", true, "print the diff without writing (default)")
	c.Flags().StringVar(&llmCmd, "llm-cmd", "", "agent command for the llm source, e.g. 'claude -p'")
	c.Flags().StringVar(&llmTimeout, "llm-timeout", "", "limit for one llm call, e.g. 90s (default 2m)")
	c.Flags().IntVar(&jobs, "jobs", 0, "documents to process at once (default 4)")
	c.Flags().BoolVar(&noCache, "no-cache", false, "ask the llm again instead of reusing replies stored in .nasc/cache")
	c.Flags().BoolVar(&force, "force", false, "overwrite human-set values (loud, not default)")
	return c
}

func parseTimeout(s string) (time.Duration, error) {
	if s == "" || s == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("llm timeout %q: %v", s, err)
	}
	return d, nil
}

func reportFailures(w io.Writer, fs []mark.Failure) {
	if len(fs) == 0 {
		return
	}
	for _, f := range fs {
		fmt.Fprintf(w, "nasc: %s: %s source: %v\n", f.Path, f.Source, f.Err)
	}
	fmt.Fprintf(w, "nasc: the llm source failed for %d document(s); their other fields are still marked\n", len(fs))
}

func reportSkipped(w io.Writer, skipped []mark.Skip) {
	if len(skipped) == 0 {
		return
	}
	type agg struct {
		reason string
		n      int
	}
	byKey := map[string]*agg{}
	for _, sk := range skipped {
		a := byKey[sk.Key]
		if a == nil {
			a = &agg{reason: sk.Reason}
			byKey[sk.Key] = a
		}
		a.n++
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := byKey[k]
		fmt.Fprintf(w, "nasc: left %q alone in %d doc(s): %s. Set `key:` on that field in .nasc/schema.yaml (for example key: nasc_%s) to store it under another name.\n", k, a.n, a.reason, k)
	}
}

func reportUnmapped(w io.Writer, unmapped map[string]int, s *schema.Schema) {
	if len(unmapped) == 0 {
		return
	}
	field := "type"
	for _, n := range s.FieldOrder {
		if s.Fields[n].Derive == "parent-dir" {
			field = n
			break
		}
	}
	dirs := make([]string, 0, len(unmapped))
	total := 0
	for d, n := range unmapped {
		dirs = append(dirs, d)
		total += n
	}
	sort.Slice(dirs, func(i, j int) bool {
		if unmapped[dirs[i]] != unmapped[dirs[j]] {
			return unmapped[dirs[i]] > unmapped[dirs[j]]
		}
		return dirs[i] < dirs[j]
	})
	fmt.Fprintf(w, "nasc: %d doc(s) got no %s because their directory is not in the allowed values. Map directories under fields.%s.map in .nasc/schema.yaml:\n", total, field, field)
	const show = 10
	for i, d := range dirs {
		if i == show {
			fmt.Fprintf(w, "  … and %d more\n", len(dirs)-show)
			break
		}
		fmt.Fprintf(w, "  %s: <value>   # %d doc(s)\n", yamlKey(d), unmapped[d])
	}
}

// yamlKey quotes a directory for pasting into a YAML map.
func yamlKey(d string) string {
	if d == "." || strings.ContainsAny(d, ":#{}[],&*!|>'\"%@`") {
		return fmt.Sprintf("%q", d)
	}
	return d
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
