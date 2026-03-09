// Package main defines the version command for the go-deploy CLI.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const appVersion = "0.1.0"

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "go-deploy version %s\n", appVersion)
		},
	}
}
