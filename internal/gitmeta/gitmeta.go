// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package gitmeta

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gobwas/glob"
)

var (
	availMu    sync.Mutex
	availCache = map[string]bool{}
)

// Available reports whether git is installed and root is inside a work tree.
// The answer is cached per root, since mark asks once per document.
func Available(root string) bool {
	availMu.Lock()
	defer availMu.Unlock()
	if v, ok := availCache[root]; ok {
		return v
	}
	ok := false
	if _, err := exec.LookPath("git"); err == nil {
		cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
		cmd.Dir = root
		ok = cmd.Run() == nil
	}
	availCache[root] = ok
	return ok
}

// maxHistory caps how many commits LastUpdated inspects for one file before
// giving up and using the oldest one it saw.
const maxHistory = 50

// LastUpdated returns the authored date, as YYYY-MM-DD, of the most recent
// commit that changed relpath's body. Commits that touched only the
// frontmatter are skipped, so committing nasc's own edits does not move the
// date, and re-running mark after that commit is a no-op.
//
// In a shallow clone the history may stop early; the oldest commit available
// is then used.
func LastUpdated(root, relpath string) (string, error) {
	out, err := git(root, "log", fmt.Sprintf("-n%d", maxHistory), "--format=%H %aI", "--", relpath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", nil
	}
	date := ""
	for _, line := range lines {
		hash, iso, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		date = iso
		if bodyChanged(root, hash, relpath) {
			break
		}
	}
	if len(date) >= 10 {
		return date[:10], nil
	}
	return date, nil
}

// bodyChanged reports whether commit changed relpath outside its frontmatter.
// A commit whose parent is missing (root commit, or the edge of a shallow
// clone) counts as a change.
func bodyChanged(root, commit, relpath string) bool {
	after, err := git(root, "show", commit+":./"+relpath)
	if err != nil {
		return true
	}
	before, err := git(root, "show", commit+"^:./"+relpath)
	if err != nil {
		return true
	}
	return StripFrontmatter(after) != StripFrontmatter(before)
}

// StripFrontmatter returns the document body with any leading YAML
// frontmatter removed and line endings normalised to LF.
func StripFrontmatter(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimPrefix(s, "\uFEFF")
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	rest := s[len("---\n"):]
	for _, fence := range []string{"\n---\n", "\n...\n"} {
		if i := strings.Index("\n"+rest, fence); i >= 0 {
			return rest[i+len(fence)-1:]
		}
	}
	if strings.HasSuffix(rest, "\n---") || rest == "---" {
		return ""
	}
	return s
}

// Owner returns the first owner from the CODEOWNERS rule matching relpath.
// When nothing matches and blameFallback is set, it falls back to the email of
// the author with the most blamed lines; otherwise it returns "".
func Owner(root, relpath string, blameFallback bool) (string, error) {
	if o := codeownersMatch(root, relpath); o != "" {
		return o, nil
	}
	if !blameFallback {
		return "", nil
	}
	return blameMajority(root, relpath)
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err
}

// codeownersLocations lists the CODEOWNERS paths to check, in the order
// GitHub uses. The first one that exists is used.
var codeownersLocations = []string{
	".github/CODEOWNERS",
	"CODEOWNERS",
	"docs/CODEOWNERS",
}

// topLevel returns the git work tree's top directory for root, and root's
// path relative to it ("" at the top). Outside git it returns root itself.
func topLevel(root string) (top, prefix string) {
	t, err := git(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return root, ""
	}
	p, _ := git(root, "rev-parse", "--show-prefix")
	return strings.TrimSpace(t), strings.TrimSpace(p)
}

// codeownersMatch returns the first owner token of the last CODEOWNERS
// pattern that matches relpath, or "" if no CODEOWNERS file is found or no
// pattern matches.
func codeownersMatch(root, relpath string) string {
	// CODEOWNERS lives at the repository top and its patterns are relative to
	// it, so a scan root below the top needs its prefix added back.
	top, prefix := topLevel(root)
	relpath = prefix + filepath.ToSlash(relpath)
	var path string
	for _, loc := range codeownersLocations {
		candidate := filepath.Join(top, loc)
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
