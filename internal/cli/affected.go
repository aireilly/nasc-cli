// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/aireilly/nasc-cli/internal/gitmeta"
	"github.com/aireilly/nasc-cli/internal/globs"
	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/render"
	"github.com/spf13/cobra"
)

// affectedDoc is one doc whose `paths` globs match changed files.
type affectedDoc struct {
	Doc     string   `json:"doc"`
	Files   []string `json:"files"`
	Updated bool     `json:"updated"`
}

func newAffectedCmd() *cobra.Command {
	var changed, format string
	var fail bool
	c := &cobra.Command{
		Use:   "affected [file...]",
		Short: "List docs whose `paths` cover changed files.",
		Long: `List docs whose ` + "`paths`" + ` globs match changed files, so a reviewer can see
which docs a code change might make wrong.

Pass the changed files, or --changed <ref> to use everything that differs from
the merge base with <ref>, including deleted files and the old path of a
renamed one. A doc counts as updated when it is itself one of the changed
files.

--format github prints a warning annotation on each changed file that a doc
covers, unless every covering doc was updated too. --fail exits 3 when a
covering doc was not updated.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, cfg, _, _, err := loadAll(cmd)
			if err != nil {
				return err
			}
			if len(args) == 0 && changed == "" {
				return ExitError{Code: 2, Msg: "pass changed files or --changed <ref>"}
			}
			var files []string
			for _, a := range args {
				rel, rerr := relToRoot(cfg.Root, a)
				if rerr != nil {
					return rerr
				}
				files = append(files, rel)
			}
			if changed != "" {
				if !gitmeta.Available(cfg.Root) {
					return ExitError{Code: 2, Msg: "--changed needs git and a work tree"}
				}
				more, cerr := gitmeta.ChangedFiles(cfg.Root, changed, true)
				if cerr != nil {
					return ExitError{Code: 2, Msg: cerr.Error()}
				}
				files = append(files, more...)
			}
			hits := findAffected(docs, files)

			w := cmd.OutOrStdout()
			switch format {
			case "", "table":
				writeAffectedTable(w, hits)
			case "jsonl":
				enc := json.NewEncoder(w)
				for _, h := range hits {
					if err := enc.Encode(h); err != nil {
						return err
					}
				}
			case "github":
				writeAffectedGitHub(w, hits, gitmeta.Prefix(cfg.Root))
			default:
				return ExitError{Code: 2, Msg: fmt.Sprintf("unknown format %q (want table, jsonl, or github)", format)}
			}
			if fail {
				for _, h := range hits {
					if !h.Updated {
						return ExitError{Code: 3, Msg: "docs cover changed files but were not updated"}
					}
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&changed, "changed", "", "use files changed since the merge base with this git ref, for example origin/main")
	c.Flags().StringVar(&format, "format", "", "output format: table|jsonl|github")
	c.Flags().BoolVar(&fail, "fail", false, "exit 3 when a covering doc was not updated")
	return c
}

func findAffected(docs []model.Doc, files []string) []affectedDoc {
	changed := map[string]bool{}
	for _, f := range files {
		changed[f] = true
	}
	var out []affectedDoc
	for _, d := range docs {
		if len(d.Paths) == 0 {
			continue
		}
		var matched []string
		for _, f := range files {
			if f == d.Path {
				continue
			}
			if globs.MatchAny(d.Paths, f) {
				matched = append(matched, f)
			}
		}
		if len(matched) == 0 {
			continue
		}
		sort.Strings(matched)
		out = append(out, affectedDoc{Doc: d.Path, Files: matched, Updated: changed[d.Path]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Doc < out[j].Doc })
	return out
}

func writeAffectedTable(w io.Writer, hits []affectedDoc) {
	if len(hits) == 0 {
		fmt.Fprintln(w, "No docs cover the changed files.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "DOC\tUPDATED\tCHANGED FILES IT COVERS")
	for _, h := range hits {
		updated := "no"
		if h.Updated {
			updated = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Doc, updated, strings.Join(h.Files, ", "))
	}
	_ = tw.Flush()
}

func writeAffectedGitHub(w io.Writer, hits []affectedDoc, prefix string) {
	byFile := map[string][]string{}
	for _, h := range hits {
		if h.Updated {
			continue
		}
		for _, f := range h.Files {
			byFile[f] = append(byFile[f], prefix+h.Doc)
		}
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		docs := byFile[f]
		msg := fmt.Sprintf("%s cover%s this file. Check that %s still match%s the change.",
			strings.Join(docs, ", "), plural(len(docs), "s", ""), plural(len(docs), "it", "they"), plural(len(docs), "es", ""))
		fmt.Fprintf(w, "::warning file=%s,title=nasc affected docs::%s\n", render.GitHubProperty(prefix+f), render.GitHubData(msg))
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
