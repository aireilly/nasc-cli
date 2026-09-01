// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package edit

import (
	"fmt"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

// UnifiedDiff returns a git-apply-compatible unified diff for one file.
func UnifiedDiff(path string, old, new []byte) string {
	edits := myers.ComputeEdits(span.URIFromPath(path), string(old), string(new))
	u := gotextdiff.ToUnified("a/"+path, "b/"+path, string(old), edits)
	return fmt.Sprint(u)
}
