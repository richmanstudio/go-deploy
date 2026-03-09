// Package main defines deployment commands for go-deploy.
package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/richmanstudio/go-deploy/internal/config"
	"github.com/richmanstudio/go-deploy/internal/deploy"
	sshclient "github.com/richmanstudio/go-deploy/internal/ssh"
	"github.com/richmanstudio/go-deploy/internal/ui"
	"github.com/spf13/cobra"
)

func newDeployCommand(opts *rootOptions) *cobra.Command {
	var serverFlag string

	cmd := &cobra.Command{
		Use:   "deploy [server]",
		Short: "Run deployment on one server or all servers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			output := ui.New(ui.Options{
				NoColor: opts.noColor,
				Verbose: opts.verbose,
				Out:     cmd.OutOrStdout(),
				ErrOut:  cmd.ErrOrStderr(),
			})

			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return fmt.Errorf("deploy command: load config: %w", err)
			}

			targets, err := resolveDeployTargets(args, serverFlag, cfg.Servers)
			if err != nil {
				return fmt.Errorf("deploy command: resolve targets: %w", err)
			}

			deployer := deploy.New(
				output,
				deploy.Options{
					DryRun:         opts.dryRun,
					CommandTimeout: opts.timeout,
				},
				func(server config.Server) (deploy.SSHClient, error) {
					client, clientErr := sshclient.NewClient(sshclient.ConnectionConfig{
						Host:     server.Host,
						Port:     server.Port,
						User:     server.User,
						KeyPath:  server.Key,
						Password: server.Password,
					})
					if clientErr != nil {
						return nil, fmt.Errorf("new ssh client: %w", clientErr)
					}
					return client, nil
				},
			)

			for _, target := range targets {
				server := cfg.Servers[target]
				report, deployErr := deployer.DeployToServer(context.Background(), target, server, cfg.Deploy)
				output.Info(formatDeployReport(report))
				if deployErr != nil {
					return fmt.Errorf("deploy command: %w", deployErr)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&serverFlag, "server", "", "Deploy to specific server")
	return cmd
}

func resolveDeployTargets(args []string, serverFlag string, servers map[string]config.Server) ([]string, error) {
	positional := ""
	if len(args) == 1 {
		positional = strings.TrimSpace(args[0])
	}

	selectedFlag := strings.TrimSpace(serverFlag)
	if positional != "" && selectedFlag != "" {
		return nil, errors.New("use either positional server argument or --server flag, not both")
	}

	selectedServer := selectedFlag
	if selectedServer == "" {
		selectedServer = positional
	}

	if selectedServer != "" {
		if _, ok := servers[selectedServer]; !ok {
			return nil, fmt.Errorf("server %q not found in config", selectedServer)
		}
		return []string{selectedServer}, nil
	}

	targets := make([]string, 0, len(servers))
	for name := range servers {
		targets = append(targets, name)
	}
	sort.Strings(targets)
	return targets, nil
}

func formatDeployReport(report deploy.Report) string {
	return fmt.Sprintf(
		"report server=%s status=%s steps=%d/%d rollback=%t rollback_steps=%d duration=%s",
		report.Server,
		report.Status,
		report.StepsCompleted,
		report.StepsTotal,
		report.RollbackTriggered,
		report.RollbackCompleted,
		report.Duration.Round(10*time.Millisecond),
	)
}
