// Package ssh provides an SSH client wrapper used by deploy workflows.
package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

const defaultDialTimeout = 10 * time.Second

// ConnectionConfig describes SSH connection and authentication options.
type ConnectionConfig struct {
	Host        string
	Port        int
	User        string
	KeyPath     string
	Password    string
	DialTimeout time.Duration
}

// CommandResult contains command output and exit status from remote execution.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Client provides methods to connect, execute commands, and close SSH sessions.
type Client struct {
	config       ConnectionConfig
	dialFn       dialFunc
	newSessionFn newSessionFunc
	sshClient    *gossh.Client
	isConnected  bool
}

type dialFunc func(network, address string, cfg *gossh.ClientConfig) (*gossh.Client, error)
type newSessionFunc func(client *gossh.Client) (commandSession, error)

type commandSession interface {
	Run(command string, stdout *bytes.Buffer, stderr *bytes.Buffer) error
	Close() error
}

// NewClient creates an SSH client wrapper with validated connection configuration.
func NewClient(cfg ConnectionConfig) (*Client, error) {
	if err := validateConnectionConfig(cfg); err != nil {
		return nil, fmt.Errorf("ssh: new client: %w", err)
	}

	return &Client{
		config:       cfg,
		dialFn:       gossh.Dial,
		newSessionFn: defaultNewSession,
	}, nil
}

// Connect opens an SSH connection using password or key-based authentication.
func (c *Client) Connect(ctx context.Context) error {
	if c.isConnected {
		return nil
	}

	authMethod, err := buildAuthMethod(c.config)
	if err != nil {
		return fmt.Errorf("ssh: connect: build auth method: %w", err)
	}

	dialTimeout := c.config.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultDialTimeout
	}

	clientConfig := &gossh.ClientConfig{
		User:            c.config.User,
		Auth:            []gossh.AuthMethod{authMethod},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // Training project; strict host verification will be added later.
		Timeout:         dialTimeout,
	}

	address := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	sshClient, err := dialWithContext(ctx, c.dialFn, "tcp", address, clientConfig)
	if err != nil {
		return fmt.Errorf("ssh: connect: dial %s: %w", address, err)
	}

	c.sshClient = sshClient
	c.isConnected = true
	return nil
}

// Run executes one remote command and returns command output with exit code.
func (c *Client) Run(ctx context.Context, command string, timeout time.Duration) (CommandResult, error) {
	if !c.isConnected {
		return CommandResult{}, errors.New("ssh: run: client is not connected")
	}

	session, err := c.newSessionFn(c.sshClient)
	if err != nil {
		return CommandResult{}, fmt.Errorf("ssh: run: create session: %w", err)
	}

	execCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	errCh := make(chan error, 1)

	go func() {
		errCh <- session.Run(command, &stdout, &stderr)
	}()

	select {
	case runErr := <-errCh:
		if closeErr := session.Close(); closeErr != nil {
			return CommandResult{}, fmt.Errorf("ssh: run: close session: %w", closeErr)
		}

		exitCode, err := mapRunError(runErr)
		if err != nil {
			return CommandResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: exitCode,
			}, fmt.Errorf("ssh: run: %w", err)
		}

		return CommandResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
		}, nil

	case <-execCtx.Done():
		if closeErr := session.Close(); closeErr != nil {
			return CommandResult{}, fmt.Errorf("ssh: run: close timed out session: %w", closeErr)
		}

		return CommandResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: -1,
		}, fmt.Errorf("ssh: run: command timeout: %w", execCtx.Err())
	}
}

// Close gracefully terminates the underlying SSH connection.
func (c *Client) Close() error {
	if !c.isConnected {
		return nil
	}

	if err := c.sshClient.Close(); err != nil {
		return fmt.Errorf("ssh: close: %w", err)
	}

	c.isConnected = false
	c.sshClient = nil
	return nil
}

func validateConnectionConfig(cfg ConnectionConfig) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return errors.New("host is required")
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port must be in range 1-65535, got %d", cfg.Port)
	}

	if strings.TrimSpace(cfg.User) == "" {
		return errors.New("user is required")
	}

	keyProvided := strings.TrimSpace(cfg.KeyPath) != ""
	passwordProvided := strings.TrimSpace(cfg.Password) != ""
	if !keyProvided && !passwordProvided {
		return errors.New("either key path or password is required")
	}

	if keyProvided && passwordProvided {
		return errors.New("key path and password are mutually exclusive")
	}

	return nil
}

func buildAuthMethod(cfg ConnectionConfig) (gossh.AuthMethod, error) {
	if strings.TrimSpace(cfg.Password) != "" {
		return gossh.Password(cfg.Password), nil
	}

	keyPath, err := expandPath(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("expand key path: %w", err)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	signer, err := gossh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return gossh.PublicKeys(signer), nil
}

func dialWithContext(
	ctx context.Context,
	dial dialFunc,
	network string,
	address string,
	cfg *gossh.ClientConfig,
) (*gossh.Client, error) {
	type dialResult struct {
		client *gossh.Client
		err    error
	}

	resultCh := make(chan dialResult, 1)
	go func() {
		client, err := dial(network, address, cfg)
		resultCh <- dialResult{client: client, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("dial canceled: %w", ctx.Err())
	case result := <-resultCh:
		if result.err != nil {
			return nil, fmt.Errorf("dial failed: %w", result.err)
		}
		return result.client, nil
	}
}

func mapRunError(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	var exitErr interface{ ExitStatus() int }
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), fmt.Errorf("command failed with exit code %d: %w", exitErr.ExitStatus(), err)
	}

	return -1, fmt.Errorf("command failed: %w", err)
}

func defaultNewSession(client *gossh.Client) (commandSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}

	return &sshSession{session: session}, nil
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

type sshSession struct {
	session *gossh.Session
}

func (s *sshSession) Run(command string, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	s.session.Stdout = stdout
	s.session.Stderr = stderr
	return s.session.Run(command)
}

func (s *sshSession) Close() error {
	return s.session.Close()
}
