// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aireilly/nasc-cli/internal/edit"
	"github.com/aireilly/nasc-cli/internal/gitmeta"
	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/render"
	"github.com/aireilly/nasc-cli/internal/validate"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	var format, minSeverity, changed, baselineMode string
	c := &cobra.Command{
		Use:   "validate [path...]",
		Short: "Enforce the schema across the corpus. Fails CI on error findings.",
		Long: `Enforce the schema across the corpus.

With a checked-in .nasc/schema.yaml, error findings exit 3 and fail CI.
Without one, nasc checks against its built-in default and reports every
finding as a warning, so adding nasc to CI never fails a build until the
project commits a schema of its own.

Pass paths (files or directories) or --changed <ref> to check only some docs.
--changed checks docs changed since the merge base with <ref>, including
uncommitted and untracked files, so a pull request is judged on what it
touches.

When .nasc/baseline.json exists, findings recorded in it are reported as
suppressed and do not fail the run. Write it with --baseline write, which
records today's findings so only new problems fail CI.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			docs, cfg, s, isDefault, err := loadAll(cmd)
			if err != nil {
				return err
			}
			stderr := cmd.ErrOrStderr()

			scoped := len(args) > 0 || changed != ""
			if scoped {
				docs, err = scopeDocs(stderr, docs, cfg.Root, args, changed)
				if err != nil {
					return err
				}
				if len(docs) == 0 {
					fmt.Fprintln(stderr, "nasc: no matching docs to validate")
					return nil
				}
			}

			findings := validate.Check(docs, s, cfg.Root)
			if isDefault {
				fmt.Fprintf(stderr, "nasc: no .nasc/schema.yaml, so checking against the built-in %q default as warnings only (run `nasc schema init` to enforce)\n", defaultPreset)
				for i := range findings {
					findings[i].Severity = "warn"
				}
			}

			blPath := nascPath("baseline.json")
			switch baselineMode {
			case "write":
				if scoped {
					return ExitError{Code: 2, Msg: "--baseline write records the whole corpus, so drop the paths and --changed"}
				}
				n, werr := writeBaseline(blPath, findings)
				if werr != nil {
					return werr
				}
				fmt.Fprintf(stderr, "nasc: recorded %d finding(s) in %s\n", n, blPath)
				return nil
			case "", "ignore":
			default:
				return ExitError{Code: 2, Msg: fmt.Sprintf("--baseline: unknown mode %q (want write or ignore)", baselineMode)}
			}
			if baselineMode != "ignore" {
				bl, berr := readBaseline(blPath)
				if berr != nil {
					return ExitError{Code: 2, Msg: berr.Error()}
				}
				if bl != nil {
					var suppressed, fixed int
					findings, suppressed, fixed = applyBaseline(findings, bl, scoped)
					if suppressed > 0 {
						fmt.Fprintf(stderr, "nasc: %d known finding(s) suppressed by %s\n", suppressed, blPath)
					}
					if fixed > 0 {
						fmt.Fprintf(stderr, "nasc: %d baseline finding(s) are fixed. Run `nasc validate --baseline write` to shrink the baseline.\n", fixed)
					}
				}
			}

			findings = filterSeverity(findings, minSeverity)
			f := render.Detect(cmd.OutOrStdout(), format, cfg.Output.DefaultFormat)
			if f == render.FormatGitHub {
				// Annotations need paths from the repository top.
				prefix := gitmeta.Prefix(cfg.Root)
				for i := range findings {
					findings[i].Path = prefix + findings[i].Path
				}
			}
			if err := render.Findings(cmd.OutOrStdout(), findings, f); err != nil {
				return err
			}
			if hasErrors(findings) {
				return ExitError{Code: 3, Msg: "validation failed"}
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "", "output format: table|json|jsonl|paths|github")
	c.Flags().BoolVar(new(bool), "json", false, "shorthand for --format json")
	c.Flags().StringVar(&minSeverity, "severity", "warn", "minimum severity to report: warn|error")
	c.Flags().StringVar(&changed, "changed", "", "check only docs changed since the merge base with this git ref, for example origin/main")
	c.Flags().StringVar(&baselineMode, "baseline", "", "write records current findings in .nasc/baseline.json, ignore skips the baseline")
	// Map --json to --format json.
	c.PreRunE = func(cmd *cobra.Command, args []string) error {
		if j, _ := cmd.Flags().GetBool("json"); j && format == "" {
			format = "json"
		}
		return nil
	}
	return c
}

// scopeDocs keeps the docs named by args (files or directories, relative to
// the current directory) and, with ref set, the docs changed since ref.
func scopeDocs(stderr io.Writer, docs []model.Doc, root string, args []string, ref string) ([]model.Doc, error) {
	want := map[string]bool{}
	var dirs []string
	for _, a := range args {
		rel, err := relToRoot(root, a)
		if err != nil {
			// A file outside the scan root cannot be a doc. pre-commit passes
			// every staged markdown file, so skip it rather than fail. A
			// directory outside the root is a mistake, so it stays an error.
			if isDir(a) {
				return nil, err
			}
			fmt.Fprintf(stderr, "nasc: skipping %s, it is outside the scan root\n", a)
			continue
		}
		if isDir(a) {
			if rel == "." {
				return docs, nil
			}
			dirs = append(dirs, rel+"/")
		} else {
			want[rel] = true
		}
	}
	if ref != "" {
		if !gitmeta.Available(root) {
			return nil, ExitError{Code: 2, Msg: "--changed needs git and a work tree"}
		}
		files, err := gitmeta.ChangedFiles(root, ref, false)
		if err != nil {
			return nil, ExitError{Code: 2, Msg: err.Error()}
		}
		for _, f := range files {
			want[f] = true
		}
		// A change to the schema, config, baseline, or ignore files can change
		// the result for every doc, so check them all.
		if f := settingsChanged(ref); f != "" {
			fmt.Fprintf(stderr, "nasc: %s changed, so checking every doc\n", f)
			return docs, nil
		}
	}
	var out []model.Doc
	for _, d := range docs {
		keep := want[d.Path]
		for _, dir := range dirs {
			if strings.HasPrefix(d.Path, dir) {
				keep = true
			}
		}
		if keep {
			out = append(out, d)
		}
	}
	return out, nil
}

// baselineEntry identifies a finding independent of its message wording.
type baselineEntry struct {
	Path  string `json:"path"`
	Field string `json:"field,omitempty"`
	Rule  string `json:"rule"`
}

type baselineFile struct {
	Version  int             `json:"version"`
	Findings []baselineEntry `json:"findings"`
}

func entryOf(f validate.Finding) baselineEntry {
	return baselineEntry{Path: f.Path, Field: f.Field, Rule: f.Rule}
}

func writeBaseline(path string, findings []validate.Finding) (int, error) {
	seen := map[baselineEntry]bool{}
	var entries []baselineEntry
	for _, f := range findings {
		e := entryOf(f)
		if !seen[e] {
			seen[e] = true
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.Rule < b.Rule
	})
	if entries == nil {
		entries = []baselineEntry{}
	}
	data, err := json.MarshalIndent(baselineFile{Version: 1, Findings: entries}, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	return len(entries), edit.WriteAtomic(path, append(data, '\n'), 0o644)
}

func readBaseline(path string) (map[baselineEntry]bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var bf baselineFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	out := make(map[baselineEntry]bool, len(bf.Findings))
	for _, e := range bf.Findings {
		out[e] = true
	}
	return out, nil
}

// applyBaseline drops findings the baseline already records. It returns the
// remaining findings, how many were suppressed, and how many baseline entries
// no longer occur. Fixed entries are only counted for a full-corpus run, since
// a scoped run cannot tell a fixed finding from an unchecked one.
func applyBaseline(findings []validate.Finding, bl map[baselineEntry]bool, scoped bool) ([]validate.Finding, int, int) {
	var out []validate.Finding
	hit := map[baselineEntry]bool{}
	for _, f := range findings {
		e := entryOf(f)
		if bl[e] {
			hit[e] = true
			continue
		}
		out = append(out, f)
	}
	suppressed := len(findings) - len(out)
	fixed := 0
	if !scoped {
		for e := range bl {
			if !hit[e] {
				fixed++
			}
		}
	}
	return out, suppressed, fixed
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

// settingsChanged returns the first file that changed since the merge base
// with ref and that affects every doc's result, or "" when there is none.
func settingsChanged(ref string) string {
	files, err := gitmeta.ChangedFiles(projectRoot(), ref, true)
	if err != nil {
		return ""
	}
	for _, f := range files {
		switch {
		case f == ".nasc/schema.yaml", f == ".nasc/config.yaml", f == ".nasc/baseline.json":
			return f
		case path.Base(f) == ".nascignore", path.Base(f) == ".gitignore":
			return f
		}
	}
	return ""
}
