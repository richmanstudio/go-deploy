// Package main defines the servers listing command for go-deploy.
package main

import (
	"fmt"
	"sort"

	"github.com/richmanstudio/go-deploy/internal/config"
	"github.com/richmanstudio/go-deploy/internal/ui"
	"github.com/spf13/cobra"
)

func newServersCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "servers",
		Short: "Show servers from deployment config",
		RunE: func(cmd *cobra.Command, args []string) error {
			output := ui.New(ui.Options{
				NoColor: opts.noColor,
				Verbose: opts.verbose,
				Out:     cmd.OutOrStdout(),
				ErrOut:  cmd.ErrOrStderr(),
			})

			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return fmt.Errorf("servers command: load config: %w", err)
			}

			rows := buildServersTableRows(cfg.Servers)
			output.PrintServersTable(rows)
			return nil
		},
	}
}

func buildServersTableRows(servers map[string]config.Server) []ui.ServerInfo {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]ui.ServerInfo, 0, len(names))
	for _, name := range names {
		server := servers[name]
		rows = append(rows, ui.ServerInfo{
			Name:     name,
			Host:     server.Host,
			Port:     server.Port,
			User:     server.User,
			AuthType: authType(server),
		})
	}
	return rows
}

func authType(server config.Server) string {
	switch {
	case server.Key != "":
		return "key"
	case server.Password != "":
		return "password"
	default:
		return "unknown"
	}
}
