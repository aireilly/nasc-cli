// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package gitmeta

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// Available reports whether git is installed and root is inside a work tree.
func Available(root string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = root
	return cmd.Run() == nil
}

// LastUpdated returns the authored date of the most recent commit touching
// relpath, as YYYY-MM-DD.
func LastUpdated(root, relpath string) (string, error) {
	out, err := git(root, "log", "-1", "--format=%aI", "--", relpath)
	if err != nil {
		return "", err
	}
	iso := strings.TrimSpace(out)
	if len(iso) >= 10 {
		return iso[:10], nil
	}
	return iso, nil
}

// Owner returns a CODEOWNERS match for relpath, falling back to the blame
// majority author.
func Owner(root, relpath string) (string, error) {
	if o := codeownersMatch(root, relpath); o != "" {
		return o, nil
	}
	return blameMajority(root, relpath)
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err
}

// codeownersLocations lists the CODEOWNERS paths to check, in priority
// order. The first one that exists is used.
var codeownersLocations = []string{
	"CODEOWNERS",
	".github/CODEOWNERS",
	"docs/CODEOWNERS",
}

// codeownersMatch returns the first owner token of the last CODEOWNERS
// pattern that matches relpath, or "" if no CODEOWNERS file is found or no
// pattern matches.
func codeownersMatch(root, relpath string) string {
	var path string
	for _, loc := range codeownersLocations {
		candidate := filepath.Join(root, loc)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			path = candidate
			break
		}
	}
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	relpath = filepath.ToSlash(relpath)

	winner := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pattern := fields[0]
		owners := fields[1:]

		ok, err := matchCodeownersPattern(pattern, relpath)
		if err != nil || !ok {
			continue
		}
		if len(owners) == 0 {
			winner = ""
			continue
		}
		winner = owners[0]
	}
	return winner
}

// matchCodeownersPattern reports whether pattern (gitignore-style, as used
// by CODEOWNERS) matches relpath. relpath must already use forward slashes.
func matchCodeownersPattern(pattern, relpath string) (bool, error) {
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")

	dirOnly := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")

	if pattern == "" {
		return false, nil
	}

	// gitignore semantics: a slash anywhere but the trailing position
	// anchors the pattern to the root, same as a leading slash.
	if !anchored && strings.Contains(pattern, "/") {
		anchored = true
	}

	g, err := glob.Compile(pattern, '/')
	if err != nil {
		return false, err
	}

	segs := strings.Split(relpath, "/")

	if anchored {
		if !dirOnly {
			return g.Match(relpath), nil
		}
		patSegs := strings.Split(pattern, "/")
		if len(segs) <= len(patSegs) {
			return false, nil
		}
		prefix := strings.Join(segs[:len(patSegs)], "/")
		return g.Match(prefix), nil
	}

	// Unanchored: pattern has no slash, so it may match a segment at any
	// depth in the path.
	if dirOnly {
		// Only directory segments (ancestors of the final path element)
		// count as a match.
		for _, seg := range segs[:len(segs)-1] {
			if g.Match(seg) {
				return true, nil
			}
		}
		return false, nil
	}

	for _, seg := range segs {
		if g.Match(seg) {
			return true, nil
		}
	}
	return false, nil
}

// blameMajority returns the author-mail with the most blamed lines in
// relpath, as reported by `git blame`.
func blameMajority(root, relpath string) (string, error) {
	out, err := git(root, "blame", "--line-porcelain", "--", relpath)
	if err != nil {
		return "", err
	}

	counts := make(map[string]int)
	var order []string
	for _, line := range strings.Split(out, "\n") {
		mail, ok := strings.CutPrefix(line, "author-mail ")
		if !ok {
			continue
		}
		mail = strings.TrimSpace(mail)
		mail = strings.TrimPrefix(mail, "<")
		mail = strings.TrimSuffix(mail, ">")
		if _, seen := counts[mail]; !seen {
			order = append(order, mail)
		}
		counts[mail]++
	}

	best := ""
	bestCount := 0
	for _, mail := range order {
		if counts[mail] > bestCount {
			bestCount = counts[mail]
			best = mail
		}
	}
	return best, nil
}
