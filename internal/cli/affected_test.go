// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAffected(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "docs", "router.md"), "---\ntitle: Router\npaths: [\"pkg/router/**\"]\n---\n")
	mustWrite(t, filepath.Join(dir, "docs", "cli.md"), "---\ntitle: CLI\npaths: [\"cmd/**\", \"*.sh\"]\n---\n")
	mustWrite(t, filepath.Join(dir, "docs", "none.md"), "# No paths\n")
	chdir(t, dir)

	out, _, err := run(t, "affected", "pkg/router/route.go", "scripts/build.sh", "docs/cli.md")
	expectCode(t, err, 0)
	if !strings.Contains(out, "docs/router.md  no       pkg/router/route.go") ||
		!strings.Contains(out, "docs/cli.md     yes      scripts/build.sh") {
		t.Fatalf("table:\n%s", out)
	}

	out, _, err = run(t, "affected", "--format", "github", "pkg/router/route.go", "scripts/build.sh", "docs/cli.md")
	expectCode(t, err, 0)
	if !strings.Contains(out, "::warning file=pkg/router/route.go,title=nasc affected docs::docs/router.md covers this file. Check that it still matches the change.") {
		t.Fatalf("github:\n%s", out)
	}
	if strings.Contains(out, "build.sh") {
		t.Fatalf("a doc updated in the same change needs no warning:\n%s", out)
	}

	_, _, err = run(t, "affected", "--fail", "pkg/router/route.go")
	expectCode(t, err, 3)
	_, _, err = run(t, "affected", "--fail", "pkg/router/route.go", "docs/router.md")
	expectCode(t, err, 0)
	_, _, err = run(t, "affected")
	expectCode(t, err, 2)
}
