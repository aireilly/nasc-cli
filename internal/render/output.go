// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/aireilly/nasc-cli/internal/validate"
	"golang.org/x/term"
)

// Format selects an output encoding.
type Format int

const (
	FormatAuto Format = iota
	FormatTable
	FormatJSON
	FormatJSONL
	FormatPaths
	// FormatGitHub emits GitHub Actions workflow commands, which show each
	// finding as an annotation on the pull request diff.
	FormatGitHub
)

// Detect resolves the output format from an explicit flag, the config default,
// and whether w is a TTY.
func Detect(w io.Writer, flag, cfgDefault string) Format {
	switch flag {
	case "table":
		return FormatTable
	case "json":
		return FormatJSON
	case "jsonl":
		return FormatJSONL
	case "paths":
		return FormatPaths
	case "github":
		return FormatGitHub
	}
	if cfgDefault == "table" {
		return FormatTable
	}
	if cfgDefault == "jsonl" {
		return FormatJSONL
	}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return FormatTable
	}
	return FormatJSONL
}

// Findings writes findings in the chosen format.
func Findings(w io.Writer, fs []validate.Finding, format Format) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(fs)
	case FormatJSONL:
		enc := json.NewEncoder(w)
		for _, f := range fs {
			if err := enc.Encode(f); err != nil {
				return err
			}
		}
		return nil
	case FormatPaths:
		seen := make(map[string]bool, len(fs))
		for _, f := range fs {
			if seen[f.Path] {
				continue
			}
			seen[f.Path] = true
			if _, err := fmt.Fprintln(w, f.Path); err != nil {
				return err
			}
		}
		return nil
	case FormatGitHub:
		for _, f := range fs {
			level := "error"
			if f.Severity != "error" {
				level = "warning"
			}
			title := "nasc " + f.Rule
			if f.Field != "" {
				title += " (" + f.Field + ")"
			}
			if _, err := fmt.Fprintf(w, "::%s file=%s,line=1,title=%s::%s\n",
				level, ghProperty(f.Path), ghProperty(title), ghData(f.Message)); err != nil {
				return err
			}
		}
		return nil
	default:
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "SEVERITY\tFILE\tFIELD\tRULE\tMESSAGE")
		for _, f := range fs {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.Severity, f.Path, f.Field, f.Rule, f.Message)
		}
		return tw.Flush()
	}
}

// ghData escapes a workflow command message.
func ghData(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	return strings.ReplaceAll(s, "\n", "%0A")
}

// ghProperty escapes a workflow command property value.
func ghProperty(s string) string {
	s = ghData(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	return strings.ReplaceAll(s, ",", "%2C")
}
