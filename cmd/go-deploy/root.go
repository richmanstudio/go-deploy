// Package main contains the root Cobra command for the go-deploy CLI.
package main

import (
	"time"

	"github.com/spf13/cobra"
)

type rootOptions struct {
	configPath string
	dryRun     bool
	verbose    bool
	noColor    bool
	timeout    time.Duration
}

func newRootCommand() *cobra.Command {
	opts := &rootOptions{
		configPath: "./deploy.yaml",
		timeout:    30 * time.Second,
	}

	rootCmd := &cobra.Command{
		Use:   "go-deploy",
		Short: "CLI tool for automated deployments over SSH",
	}

	rootCmd.PersistentFlags().StringVarP(&opts.configPath, "config", "c", "./deploy.yaml", "Path to deploy config file")
	rootCmd.PersistentFlags().BoolVar(&opts.dryRun, "dry-run", false, "Show what will be executed without running commands")
	rootCmd.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "Enable verbose command output")
	rootCmd.PersistentFlags().BoolVar(&opts.noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 30*time.Second, "Global command timeout")

	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newValidateCommand(opts))
	rootCmd.AddCommand(newInitCommand(opts))
	rootCmd.AddCommand(newDeployCommand(opts))
	rootCmd.AddCommand(newServersCommand(opts))

	return rootCmd
}
