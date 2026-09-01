// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/aireilly/nasc-cli/internal/cli"
)

func main() {
	cmd := cli.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "nasc:", err)
		var ec cli.ExitError
		if errors.As(err, &ec) {
			os.Exit(ec.Code)
		}
		os.Exit(2)
	}
}
