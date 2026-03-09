// Package config tests validation and loading behavior for deploy config.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	keyPath := createTempKeyFile(t)

	tests := []struct {
		name        string
		mutate      func(cfg *Config)
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid config",
			mutate:  func(cfg *Config) {},
			wantErr: false,
		},
		{
			name: "empty host",
			mutate: func(cfg *Config) {
				server := cfg.Servers["production"]
				server.Host = ""
				cfg.Servers["production"] = server
			},
			wantErr:     true,
			errContains: "host is required",
		},
		{
			name: "invalid port",
			mutate: func(cfg *Config) {
				server := cfg.Servers["production"]
				server.Port = 70000
				cfg.Servers["production"] = server
			},
			wantErr:     true,
			errContains: "port must be in range",
		},
		{
			name: "missing auth method",
			mutate: func(cfg *Config) {
				server := cfg.Servers["production"]
				server.Key = ""
				server.Password = ""
				cfg.Servers["production"] = server
			},
			wantErr:     true,
			errContains: "either key or password is required",
		},
		{
			name: "missing step command",
			mutate: func(cfg *Config) {
				cfg.Deploy.Steps[0].Command = ""
			},
			wantErr:     true,
			errContains: ".command is required",
		},
		{
			name: "missing key file",
			mutate: func(cfg *Config) {
				server := cfg.Servers["production"]
				server.Key = filepath.Join(t.TempDir(), "missing_key")
				cfg.Servers["production"] = server
			},
			wantErr:     true,
			errContains: "key path does not exist",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := validConfig(keyPath)
			tt.mutate(&cfg)

			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	keyPath := createTempKeyFile(t)
	configPath := filepath.Join(t.TempDir(), "deploy.yaml")

	content := fmt.Sprintf(`project: sample
version: "1.0.0"
servers:
  production:
    host: "127.0.0.1"
    port: 22
    user: "deploy"
    key: %q
deploy:
  steps:
    - name: "Build"
      command: "go build ./..."
      timeout: 10s
`, keyPath)

	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "sample", cfg.Project)
	assert.Equal(t, "deploy", cfg.Servers["production"].User)
	assert.Equal(t, "Build", cfg.Deploy.Steps[0].Name)
	assert.Equal(t, "go build ./...", cfg.Deploy.Steps[0].Command)
	assert.Equal(t, "10s", cfg.Deploy.Steps[0].Timeout.String())
}

func TestWriteExample(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "deploy.yaml")
	require.NoError(t, WriteExample(path))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "project: my-app")

	err = WriteExample(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file already exists")
}

func validConfig(keyPath string) Config {
	return Config{
		Project: "my-app",
		Version: "1.0.0",
		Servers: map[string]Server{
			"production": {
				Host: "127.0.0.1",
				Port: 22,
				User: "deploy",
				Key:  keyPath,
			},
		},
		Deploy: DeployConfig{
			Steps: []Step{
				{
					Name:    "Build",
					Command: "go build ./...",
				},
			},
			Rollback: RollbackConfig{
				Steps: []Step{
					{
						Name:    "Rollback",
						Command: "git checkout HEAD~1",
					},
				},
			},
		},
	}
}

func createTempKeyFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "id_rsa")
	err := os.WriteFile(path, []byte("dummy-private-key"), 0o600)
	require.NoError(t, err)
	return path
}
