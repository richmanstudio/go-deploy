// Package ssh tests the SSH client wrapper behavior with mocked sessions.
package ssh

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         ConnectionConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid password config",
			cfg: ConnectionConfig{
				Host:     "127.0.0.1",
				Port:     22,
				User:     "deploy",
				Password: "secret",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: ConnectionConfig{
				Port:     22,
				User:     "deploy",
				Password: "secret",
			},
			wantErr:     true,
			errContains: "host is required",
		},
		{
			name: "invalid port",
			cfg: ConnectionConfig{
				Host:     "127.0.0.1",
				Port:     70000,
				User:     "deploy",
				Password: "secret",
			},
			wantErr:     true,
			errContains: "port must be in range",
		},
		{
			name: "missing auth",
			cfg: ConnectionConfig{
				Host: "127.0.0.1",
				Port: 22,
				User: "deploy",
			},
			wantErr:     true,
			errContains: "either key path or password is required",
		},
		{
			name: "both auth methods",
			cfg: ConnectionConfig{
				Host:     "127.0.0.1",
				Port:     22,
				User:     "deploy",
				Password: "secret",
				KeyPath:  "/tmp/key",
			},
			wantErr:     true,
			errContains: "mutually exclusive",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(tt.cfg)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

func TestClientRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		session     *mockSession
		timeout     time.Duration
		wantErr     bool
		errContains string
		wantCode    int
		wantStdout  string
		wantStderr  string
	}{
		{
			name: "successful command",
			session: &mockSession{
				runFn: func(command string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
					stdout.WriteString("ok")
					stderr.WriteString("")
					return nil
				},
			},
			timeout:    500 * time.Millisecond,
			wantErr:    false,
			wantCode:   0,
			wantStdout: "ok",
			wantStderr: "",
		},
		{
			name: "non-zero exit code",
			session: &mockSession{
				runFn: func(command string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
					stderr.WriteString("failed")
					return mockExitStatusError{code: 2}
				},
			},
			timeout:     500 * time.Millisecond,
			wantErr:     true,
			errContains: "exit code 2",
			wantCode:    2,
			wantStderr:  "failed",
		},
		{
			name:        "command timeout",
			session:     newBlockingMockSession(),
			timeout:     30 * time.Millisecond,
			wantErr:     true,
			errContains: "command timeout",
			wantCode:    -1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &Client{
				config: ConnectionConfig{
					Host:     "127.0.0.1",
					Port:     22,
					User:     "deploy",
					Password: "secret",
				},
				newSessionFn: func(_ *gossh.Client) (commandSession, error) {
					return tt.session, nil
				},
				isConnected: true,
			}

			result, err := client.Run(context.Background(), "echo test", tt.timeout)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantCode, result.ExitCode)
			assert.Equal(t, tt.wantStdout, result.Stdout)
			assert.Equal(t, tt.wantStderr, result.Stderr)
			assert.True(t, tt.session.closed)
		})
	}
}

func TestClientRunNotConnected(t *testing.T) {
	t.Parallel()

	client := &Client{}
	_, err := client.Run(context.Background(), "echo test", time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client is not connected")
}

func TestConnectContextCanceled(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     22,
		User:     "deploy",
		Password: "secret",
	})
	require.NoError(t, err)

	client.dialFn = func(network, address string, cfg *gossh.ClientConfig) (*gossh.Client, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, errors.New("dial should not finish before context cancellation")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = client.Connect(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial canceled")
}

type mockSession struct {
	runFn  func(command string, stdout *bytes.Buffer, stderr *bytes.Buffer) error
	closed bool
	done   chan struct{}
}

func (m *mockSession) Run(command string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	if m.runFn == nil {
		return nil
	}
	return m.runFn(command, stdout, stderr)
}

func (m *mockSession) Close() error {
	m.closed = true
	if m.done != nil {
		select {
		case <-m.done:
		default:
			close(m.done)
		}
	}
	return nil
}

func newBlockingMockSession() *mockSession {
	done := make(chan struct{})
	return &mockSession{
		done: done,
		runFn: func(command string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
			<-done
			return nil
		},
	}
}

type mockExitStatusError struct {
	code int
}

func (e mockExitStatusError) Error() string {
	return "remote command failed"
}

func (e mockExitStatusError) ExitStatus() int {
	return e.code
}
