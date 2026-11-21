// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aireilly/nasc-cli/internal/gitmeta"
	"github.com/aireilly/nasc-cli/internal/mark"
	"github.com/aireilly/nasc-cli/internal/scan"
	"github.com/aireilly/nasc-cli/internal/schema"
	"github.com/aireilly/nasc-cli/internal/ssg"
	"github.com/aireilly/nasc-cli/internal/validate"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var testLLM bool
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Report the environment nasc sees.",
		Long: `Report the environment nasc sees: project root, git, schema, site
generators, and how many docs pass validation.

--llm sends one short test prompt to the configured llm command and reports
whether a usable reply came back. This spends one call.`,
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
				fmt.Fprintf(w, "site generator: %s in %s (mark leaves %s alone there unless the schema renames the field with `key:`)\n",
					site.Name, site.Dir, strings.Join(keys, ", "))
			}
			if cerr == nil {
				reportCorpus(cmd, w)
			}
			if testLLM {
				reportLLM(w, cfg)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&testLLM, "llm", false, "send one test prompt to the llm command")
	return c
}

// reportCorpus prints how many docs pass validation and how many lack
// schema-required fields.
func reportCorpus(cmd *cobra.Command, w io.Writer) {
	docs, cfg, s, isDefault, err := loadAll(cmd)
	if err != nil {
		fmt.Fprintf(w, "corpus: %v\n", err)
		return
	}
	findings := validate.Check(docs, s, cfg.Root)
	withErrors := map[string]bool{}
	missing := map[string]bool{}
	for _, f := range findings {
		if f.Severity == "error" {
			withErrors[f.Path] = true
		}
		if f.Rule == "required" {
			missing[f.Path] = true
		}
	}
	label := "checked-in schema"
	if isDefault {
		label = "built-in default schema"
	}
	fmt.Fprintf(w, "docs passing the %s: %d of %d\n", label, len(docs)-len(withErrors), len(docs))
	fmt.Fprintf(w, "docs missing required fields: %d\n", len(missing))
	if exists(nascPath("baseline.json")) {
		fmt.Fprintf(w, "baseline present: true\n")
	}
}

func reportLLM(w io.Writer, cfg *schema.Config) {
	if cfg.Mark.LLMCmd == "" {
		fmt.Fprintln(w, "llm test: no command set (mark.llm_cmd)")
		return
	}
	timeout, err := parseTimeout(cfg.Mark.LLMTimeout)
	if err != nil {
		fmt.Fprintf(w, "llm test: %v\n", err)
		return
	}
	start := time.Now()
	got, rejected, err := mark.RunLLM(mark.LLMConfig{Cmd: cfg.Mark.LLMCmd, Timeout: timeout, Prompt: cfg.Mark.LLMPrompt}, mark.LLMRequest{
		Path:    "docs/example.md",
		Title:   "Example",
		Excerpt: "This page explains how to rotate the API signing key without downtime.",
		Want:    []string{"description"},
	})
	took := time.Since(start).Round(100 * time.Millisecond)
	switch {
	case err != nil:
		fmt.Fprintf(w, "llm test: failed after %s: %v\n", took, err)
	case len(rejected) > 0:
		fmt.Fprintf(w, "llm test: reply unusable after %s: %s\n", took, strings.Join(rejected, "; "))
	default:
		fmt.Fprintf(w, "llm test: ok in %s: %q\n", took, got["description"].String())
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
