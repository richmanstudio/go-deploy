// Package deploy contains orchestration logic for deployment and rollback workflows.
package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/richmanstudio/go-deploy/internal/config"
	sshclient "github.com/richmanstudio/go-deploy/internal/ssh"
	"github.com/richmanstudio/go-deploy/internal/ui"
)

// SSHClient is the runtime contract required by the deployer.
type SSHClient interface {
	Connect(ctx context.Context) error
	Run(ctx context.Context, command string, timeout time.Duration) (sshclient.CommandResult, error)
	Close() error
}

// ClientFactory builds SSH clients for a specific server configuration.
type ClientFactory func(server config.Server) (SSHClient, error)

// Options configure deployer execution behavior.
type Options struct {
	DryRun         bool
	CommandTimeout time.Duration
}

// Report represents execution statistics and final status of one deployment.
type Report struct {
	Server            string
	StepsTotal        int
	StepsCompleted    int
	RollbackTriggered bool
	RollbackCompleted int
	Duration          time.Duration
	Status            string
}

// Deployer executes deploy and rollback steps on one server.
type Deployer struct {
	output        *ui.Output
	options       Options
	clientFactory ClientFactory
}

// New creates a deploy orchestrator with UI and SSH client factory dependencies.
func New(output *ui.Output, options Options, clientFactory ClientFactory) *Deployer {
	if options.CommandTimeout <= 0 {
		options.CommandTimeout = 30 * time.Second
	}

	return &Deployer{
		output:        output,
		options:       options,
		clientFactory: clientFactory,
	}
}

// DeployToServer executes all deploy steps and optional rollback on failure.
func (d *Deployer) DeployToServer(
	ctx context.Context,
	serverName string,
	server config.Server,
	deployConfig config.DeployConfig,
) (Report, error) {
	startedAt := time.Now()
	report := Report{
		Server:     serverName,
		StepsTotal: len(deployConfig.Steps),
		Status:     "pending",
	}

	if d.options.DryRun {
		d.output.Info(fmt.Sprintf("[dry-run] server %s", serverName))
		for index, step := range deployConfig.Steps {
			timeout := d.resolveStepTimeout(step)
			d.output.Info(fmt.Sprintf("[dry-run] step %d/%d: %s (%s)", index+1, len(deployConfig.Steps), step.Name, timeout))
			d.output.Info(fmt.Sprintf("[dry-run] command: %s", step.Command))
		}
		report.StepsCompleted = len(deployConfig.Steps)
		report.Status = "dry-run"
		report.Duration = time.Since(startedAt)
		return report, nil
	}

	client, err := d.clientFactory(server)
	if err != nil {
		return report, fmt.Errorf("deploy: create ssh client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return report, fmt.Errorf("deploy: connect to server %q: %w", serverName, err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			d.output.Error(fmt.Sprintf("disconnect failed for server %s: %v", serverName, closeErr))
		}
	}()

	d.output.Info(fmt.Sprintf("starting deploy on %s (%s:%d)", serverName, server.Host, server.Port))

	for index, step := range deployConfig.Steps {
		stepLabel := fmt.Sprintf("step %d/%d: %s", index+1, len(deployConfig.Steps), step.Name)
		spin := d.output.StartSpinner(stepLabel)
		result, runErr := client.Run(ctx, step.Command, d.resolveStepTimeout(step))
		d.output.StopSpinner(spin)
		d.output.PrintCommandOutput(result.Stdout, result.Stderr)

		if runErr != nil {
			d.output.Error(fmt.Sprintf("%s failed", stepLabel))
			d.output.Error(fmt.Sprintf("reason: %v", runErr))

			report.Duration = time.Since(startedAt)
			report.Status = "failed"
			if len(deployConfig.Rollback.Steps) > 0 {
				report.RollbackTriggered = true
				rollbackCompleted := d.runRollback(ctx, client, deployConfig.Rollback.Steps)
				report.RollbackCompleted = rollbackCompleted
				report.Status = "rolled-back"
			}

			return report, fmt.Errorf("deploy: server %q step %q: %w", serverName, step.Name, runErr)
		}

		report.StepsCompleted++
		d.output.Success(fmt.Sprintf("%s completed", stepLabel))
	}

	report.Duration = time.Since(startedAt)
	report.Status = "success"
	return report, nil
}

func (d *Deployer) runRollback(ctx context.Context, client SSHClient, steps []config.Step) int {
	d.output.Info("starting rollback")
	completed := 0

	for index, step := range steps {
		stepLabel := fmt.Sprintf("rollback %d/%d: %s", index+1, len(steps), step.Name)
		spin := d.output.StartSpinner(stepLabel)
		result, err := client.Run(ctx, step.Command, d.resolveStepTimeout(step))
		d.output.StopSpinner(spin)
		d.output.PrintCommandOutput(result.Stdout, result.Stderr)

		if err != nil {
			d.output.Error(fmt.Sprintf("%s failed: %v", stepLabel, err))
			return completed
		}

		completed++
		d.output.Success(fmt.Sprintf("%s completed", stepLabel))
	}

	return completed
}

func (d *Deployer) resolveStepTimeout(step config.Step) time.Duration {
	if step.Timeout > 0 {
		return step.Timeout
	}
	return d.options.CommandTimeout
}
