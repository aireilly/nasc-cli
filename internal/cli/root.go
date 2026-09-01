// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package cli

import "github.com/spf13/cobra"

// NewRootCmd builds the top-level nasc command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "nasc",
		Short:         "Onboard a repository's docs for AI agents and enforce that markup in CI.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Run prints help when nasc is invoked with no subcommand.
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
	root.PersistentFlags().Bool("no-color", false, "disable coloured output")
	root.AddCommand(newSchemaCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newMarkCmd())
	root.AddCommand(newGetCmd())
	root.AddCommand(newIndexCmd())
	root.AddCommand(newDoctorCmd())
	return root
}

// ExitError carries a specific process exit code up to main.
type ExitError struct {
	Code int
	Msg  string
}

func (e ExitError) Error() string { return e.Msg }
