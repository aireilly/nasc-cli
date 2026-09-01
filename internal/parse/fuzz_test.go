// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package parse

import "testing"

func FuzzFile(f *testing.F) {
	f.Add([]byte("---\ntitle: t\n---\n# H\nbody\n"))
	f.Add([]byte("---\nbroken\n"))
	f.Add([]byte("\xEF\xBB\xBF---\r\na: 1\r\n---\r\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = File("fuzz.md", data) // must never panic
	})
}
