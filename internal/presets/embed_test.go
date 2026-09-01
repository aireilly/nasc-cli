// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package presets

import (
	"testing"

	"github.com/aireilly/nasc-cli/internal/schema"
)

func TestPresetsParse(t *testing.T) {
	for _, name := range Names() {
		data, err := Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		if _, err := schema.Parse(data); err != nil {
			t.Fatalf("preset %q does not parse: %v", name, err)
		}
	}
}

func TestGetUnknown(t *testing.T) {
	if _, err := Get("nope"); err == nil {
		t.Fatalf("expected error for unknown preset")
	}
}
