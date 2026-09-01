// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package parse

import (
	"strings"

	"github.com/aireilly/nasc-cli/internal/model"
)

// scanBody extracts headings and inline #tags in a single pass, skipping
// fenced code blocks.
func scanBody(body string) (headings []model.Heading, tags []string) {
	seen := map[string]bool{}
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if h, ok := heading(trimmed); ok {
			headings = append(headings, h)
			continue
		}
		for _, tag := range inlineTags(line) {
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return headings, tags
}

func heading(line string) (model.Heading, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return model.Heading{}, false
	}
	return model.Heading{Level: level, Text: strings.TrimSpace(line[level:])}, true
}

// inlineTags finds #word tokens preceded by start-of-line or whitespace,
// where word has at least one non-digit rune. Inline code spans are skipped.
func inlineTags(line string) []string {
	var out []string
	inCode := false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '`' {
			inCode = !inCode
			continue
		}
		if inCode || r != '#' {
			continue
		}
		if i > 0 && runes[i-1] != ' ' && runes[i-1] != '\t' {
			continue
		}
		j := i + 1
		hasAlpha := false
		for j < len(runes) && isTagRune(runes[j]) {
			if !(runes[j] >= '0' && runes[j] <= '9') {
				hasAlpha = true
			}
			j++
		}
		if j > i+1 && hasAlpha {
			out = append(out, string(runes[i+1:j]))
		}
		i = j - 1
	}
	return out
}

func isTagRune(r rune) bool {
	return r == '-' || r == '_' || r == '/' ||
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
