// Package deploy tests deploy orchestration and rollback behavior.
package deploy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/richmanstudio/go-deploy/internal/config"
	sshclient "github.com/richmanstudio/go-deploy/internal/ssh"
	"github.com/richmanstudio/go-deploy/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployToServerHappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockSSHClient{
		runResponses: []runResponse{
			{result: sshclient.CommandResult{Stdout: "ok-1", ExitCode: 0}},
			{result: sshclient.CommandResult{Stdout: "ok-2", ExitCode: 0}},
		},
	}

	deployer := New(
		ui.New(ui.Options{NoColor: true}),
		Options{CommandTimeout: 5 * time.Second},
		func(server config.Server) (SSHClient, error) {
			return mock, nil
		},
	)

	report, err := deployer.DeployToServer(context.Background(), "production", testServer(), testDeployConfig())
	require.NoError(t, err)

	assert.True(t, mock.connected)
	assert.True(t, mock.closed)
	assert.Equal(t, []string{"echo step-1", "echo step-2"}, mock.commands)

	assert.Equal(t, "success", report.Status)
	assert.Equal(t, 2, report.StepsTotal)
	assert.Equal(t, 2, report.StepsCompleted)
	assert.False(t, report.RollbackTriggered)
	assert.Equal(t, 0, report.RollbackCompleted)
}

func TestDeployToServerRollbackOnFailure(t *testing.T) {
	t.Parallel()

	mock := &mockSSHClient{
		runResponses: []runResponse{
			{result: sshclient.CommandResult{Stdout: "ok-1", ExitCode: 0}},
			{result: sshclient.CommandResult{Stderr: "boom", ExitCode: 2}, err: errors.New("step failed")},
			{result: sshclient.CommandResult{Stdout: "rb-1", ExitCode: 0}},
			{result: sshclient.CommandResult{Stdout: "rb-2", ExitCode: 0}},
		},
	}

	cfg := testDeployConfig()
	cfg.Rollback.Steps = []config.Step{
		{Name: "Rollback 1", Command: "echo rb-1"},
		{Name: "Rollback 2", Command: "echo rb-2"},
	}

	deployer := New(
		ui.New(ui.Options{NoColor: true}),
		Options{CommandTimeout: 5 * time.Second},
		func(server config.Server) (SSHClient, error) {
			return mock, nil
		},
	)

	report, err := deployer.DeployToServer(context.Background(), "production", testServer(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step")

	assert.Equal(t, "rolled-back", report.Status)
	assert.Equal(t, 2, report.StepsTotal)
	assert.Equal(t, 1, report.StepsCompleted)
	assert.True(t, report.RollbackTriggered)
	assert.Equal(t, 2, report.RollbackCompleted)
	assert.Equal(
		t,
		[]string{"echo step-1", "echo step-2", "echo rb-1", "echo rb-2"},
		mock.commands,
	)
}

type runResponse struct {
	result sshclient.CommandResult
	err    error
}

type mockSSHClient struct {
	connected    bool
	closed       bool
	commands     []string
	runResponses []runResponse
	runIndex     int
}

func (m *mockSSHClient) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *mockSSHClient) Run(ctx context.Context, command string, timeout time.Duration) (sshclient.CommandResult, error) {
	m.commands = append(m.commands, command)
	if m.runIndex >= len(m.runResponses) {
		return sshclient.CommandResult{}, errors.New("unexpected run call")
	}
	response := m.runResponses[m.runIndex]
	m.runIndex++
	return response.result, response.err
}

func (m *mockSSHClient) Close() error {
	m.closed = true
	return nil
}

func testServer() config.Server {
	return config.Server{
		Host:     "127.0.0.1",
		Port:     22,
		User:     "deploy",
		Password: "secret",
	}
}

func testDeployConfig() config.DeployConfig {
	return config.DeployConfig{
		Steps: []config.Step{
			{Name: "Step 1", Command: "echo step-1"},
			{Name: "Step 2", Command: "echo step-2"},
		},
	}
}
