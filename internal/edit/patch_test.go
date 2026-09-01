// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package edit

import (
	"strings"
	"testing"
)

func TestUnifiedDiffHasGitHeaders(t *testing.T) {
	d := UnifiedDiff("docs/a.md", []byte("a\nb\n"), []byte("a\nc\n"))
	if !strings.Contains(d, "--- a/docs/a.md") || !strings.Contains(d, "+++ b/docs/a.md") {
		t.Fatalf("missing git headers:\n%s", d)
	}
	if !strings.Contains(d, "-b") || !strings.Contains(d, "+c") {
		t.Fatalf("missing change lines:\n%s", d)
	}
}
