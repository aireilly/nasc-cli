// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import (
	"runtime/debug"
	"strings"
)

// Version is set at build time via -ldflags. It defaults to a dev marker.
var Version = "0.0.0-dev"

func init() {
	// go install and pre-commit build without the release ldflags, but the
	// module version is still in the build info.
	if Version != "0.0.0-dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			Version = strings.TrimPrefix(v, "v")
		}
	}
}
