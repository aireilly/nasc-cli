// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package edit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aireilly/nasc-cli/internal/model"
)

// A no-op Set (empty updates) must return the file byte for byte.
func TestSetNoOpIsByteIdentical(t *testing.T) {
	root := "../../testdata/vault"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skip("vault not present")
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		p := filepath.Join(root, e.Name())
		orig, _ := os.ReadFile(p)
		out, err := Set(orig, map[string]model.Value{})
		if err != nil {
			t.Fatalf("%s: Set error %v", e.Name(), err)
		}
		if string(out) != string(orig) {
			t.Errorf("%s: no-op Set changed bytes", e.Name())
		}
	}
}
