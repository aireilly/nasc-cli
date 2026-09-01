// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package main

import (
	"encoding/json"
	"os"
)

// A deterministic stand-in for an agent CLI, used in tests.
func main() {
	var req map[string]any
	_ = json.NewDecoder(os.Stdin).Decode(&req)
	resp := map[string]any{
		"description": "Load when working on the subject this document covers, before making changes.",
		"tags":        []string{"stub", "test"},
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
