// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package presets

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed all:files
var files embed.FS

// Get returns the raw bytes of a named preset.
func Get(name string) ([]byte, error) {
	data, err := files.ReadFile("files/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("unknown preset %q", name)
	}
	return data, nil
}

// Names lists the available presets, sorted.
func Names() []string {
	entries, _ := files.ReadDir("files")
	var out []string
	for _, e := range entries {
		out = append(out, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	sort.Strings(out)
	return out
}
