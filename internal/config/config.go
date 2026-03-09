// Package config loads and validates deployment configuration files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const exampleYAML = `project: my-app
version: "1.0.0"

servers:
  production:
    host: "192.168.1.100"
    port: 22
    user: "deploy"
    key: "~/.ssh/id_rsa"

  staging:
    host: "192.168.1.101"
    port: 22
    user: "deploy"
    key: "~/.ssh/id_rsa"

deploy:
  steps:
    - name: "Pull latest code"
      command: "cd /app && git pull origin main"
    - name: "Build"
      command: "cd /app && go build -o bin/app ./cmd/app"

  rollback:
    steps:
      - name: "Rollback to previous"
        command: "cd /app && git checkout HEAD~1"
`

// Config is the root deployment configuration loaded from YAML.
type Config struct {
	Project string            `mapstructure:"project"`
	Version string            `mapstructure:"version"`
	Servers map[string]Server `mapstructure:"servers"`
	Deploy  DeployConfig      `mapstructure:"deploy"`
}

// Server describes one remote host and authentication parameters.
type Server struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Key      string `mapstructure:"key"`
	Password string `mapstructure:"password"`
}

// DeployConfig stores deploy and rollback execution steps.
type DeployConfig struct {
	Steps    []Step         `mapstructure:"steps"`
	Rollback RollbackConfig `mapstructure:"rollback"`
}

// RollbackConfig stores steps executed after a failed deploy.
type RollbackConfig struct {
	Steps []Step `mapstructure:"steps"`
}

// Step is one command in deploy or rollback pipeline.
type Step struct {
	Name    string        `mapstructure:"name"`
	Command string        `mapstructure:"command"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// Load reads configuration from path and validates all required fields.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: read: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validate: %w", err)
	}

	return &cfg, nil
}

// Validate checks structural and semantic correctness of configuration.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Project) == "" {
		return errors.New("project is required")
	}

	if strings.TrimSpace(c.Version) == "" {
		return errors.New("version is required")
	}

	if len(c.Servers) == 0 {
		return errors.New("servers are required")
	}

	for name, server := range c.Servers {
		if err := validateServer(name, server); err != nil {
			return fmt.Errorf("server %q: %w", name, err)
		}
	}

	if len(c.Deploy.Steps) == 0 {
		return errors.New("deploy.steps are required")
	}

	if err := validateSteps("deploy.steps", c.Deploy.Steps); err != nil {
		return err
	}

	if err := validateSteps("deploy.rollback.steps", c.Deploy.Rollback.Steps); err != nil {
		return err
	}

	return nil
}

// WriteExample writes a safe starter deploy.yaml to path if file is missing.
func WriteExample(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config: init: file already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("config: init: stat file: %w", err)
	}

	parentDir := filepath.Dir(path)
	if parentDir != "." {
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return fmt.Errorf("config: init: create parent directory: %w", err)
		}
	}

	if err := os.WriteFile(path, []byte(exampleYAML), 0o644); err != nil {
		return fmt.Errorf("config: init: write file: %w", err)
	}

	return nil
}

func validateServer(name string, s Server) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("server name is required")
	}

	if strings.TrimSpace(s.Host) == "" {
		return errors.New("host is required")
	}

	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port must be in range 1-65535, got %d", s.Port)
	}

	if strings.TrimSpace(s.User) == "" {
		return errors.New("user is required")
	}

	keyProvided := strings.TrimSpace(s.Key) != ""
	passwordProvided := strings.TrimSpace(s.Password) != ""
	if !keyProvided && !passwordProvided {
		return errors.New("either key or password is required")
	}

	if keyProvided && passwordProvided {
		return errors.New("key and password are mutually exclusive")
	}

	if keyProvided {
		expandedPath, err := expandPath(s.Key)
		if err != nil {
			return fmt.Errorf("expand key path: %w", err)
		}

		info, err := os.Stat(expandedPath)
		if err != nil {
			return fmt.Errorf("key path does not exist: %w", err)
		}

		if info.IsDir() {
			return fmt.Errorf("key path must be a file: %s", expandedPath)
		}
	}

	return nil
}

func validateSteps(path string, steps []Step) error {
	for i, step := range steps {
		if strings.TrimSpace(step.Name) == "" {
			return fmt.Errorf("%s[%d].name is required", path, i)
		}

		if strings.TrimSpace(step.Command) == "" {
			return fmt.Errorf("%s[%d].command is required", path, i)
		}

		if step.Timeout < 0 {
			return fmt.Errorf("%s[%d].timeout must be >= 0", path, i)
		}
	}

	return nil
}

func expandPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("path is empty")
	}

	if strings.HasPrefix(trimmed, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}

		trimmed = filepath.Join(homeDir, strings.TrimPrefix(trimmed, "~"))
	}

	expanded := os.ExpandEnv(trimmed)
	return filepath.Clean(expanded), nil
}
