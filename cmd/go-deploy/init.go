// Package main defines initialization commands for go-deploy.
package main

import (
	"fmt"

	"github.com/richmanstudio/go-deploy/internal/config"
	"github.com/richmanstudio/go-deploy/internal/ui"
	"github.com/spf13/cobra"
)

func newInitCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create example deploy config",
		RunE: func(cmd *cobra.Command, args []string) error {
			output := ui.New(ui.Options{
				NoColor: opts.noColor,
				Verbose: opts.verbose,
				Out:     cmd.OutOrStdout(),
				ErrOut:  cmd.ErrOrStderr(),
			})

			if err := config.WriteExample(opts.configPath); err != nil {
				return fmt.Errorf("init command: %w", err)
			}

			output.Success(fmt.Sprintf("created example config: %s", opts.configPath))
			return nil
		},
	}
}
