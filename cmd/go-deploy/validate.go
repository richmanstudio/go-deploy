// Package main defines configuration validation commands for go-deploy.
package main

import (
	"fmt"

	"github.com/richmanstudio/go-deploy/internal/config"
	"github.com/richmanstudio/go-deploy/internal/ui"
	"github.com/spf13/cobra"
)

func newValidateCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate deployment config",
		RunE: func(cmd *cobra.Command, args []string) error {
			output := ui.New(ui.Options{
				NoColor: opts.noColor,
				Verbose: opts.verbose,
				Out:     cmd.OutOrStdout(),
				ErrOut:  cmd.ErrOrStderr(),
			})

			if _, err := config.Load(opts.configPath); err != nil {
				return fmt.Errorf("validate command: %w", err)
			}

			output.Success(fmt.Sprintf("configuration is valid: %s", opts.configPath))
			return nil
		},
	}
}
