// Package ui provides formatted terminal output primitives for CLI commands.
package ui

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

// Options configures output formatting behavior.
type Options struct {
	NoColor bool
	Verbose bool
	Out     io.Writer
	ErrOut  io.Writer
}

// ServerInfo is a printable representation of server metadata.
type ServerInfo struct {
	Name     string
	Host     string
	Port     int
	User     string
	AuthType string
}

// Output renders status lines, verbose logs, spinners, and tables.
type Output struct {
	out     io.Writer
	errOut  io.Writer
	verbose bool
	success *color.Color
	failure *color.Color
	info    *color.Color
}

// New constructs an Output with color and verbosity settings.
func New(opts Options) *Output {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	errOut := opts.ErrOut
	if errOut == nil {
		errOut = io.Discard
	}

	color.NoColor = opts.NoColor

	return &Output{
		out:     out,
		errOut:  errOut,
		verbose: opts.Verbose,
		success: color.New(color.FgGreen),
		failure: color.New(color.FgRed),
		info:    color.New(color.FgBlue),
	}
}

// Success prints a green success line prefixed with a check mark.
func (o *Output) Success(message string) {
	_, _ = o.success.Fprintln(o.out, "✓ "+message)
}

// Error prints a red error line prefixed with a cross mark.
func (o *Output) Error(message string) {
	_, _ = o.failure.Fprintln(o.errOut, "✗ "+message)
}

// Info prints a blue informational line prefixed with an arrow.
func (o *Output) Info(message string) {
	_, _ = o.info.Fprintln(o.out, "→ "+message)
}

// PrintCommandOutput prints stdout/stderr only when verbose mode is enabled.
func (o *Output) PrintCommandOutput(stdout string, stderr string) {
	if !o.verbose {
		return
	}

	if trimmedOut := strings.TrimSpace(stdout); trimmedOut != "" {
		fmt.Fprintf(o.out, "[stdout]\n%s\n", trimmedOut)
	}

	if trimmedErr := strings.TrimSpace(stderr); trimmedErr != "" {
		fmt.Fprintf(o.errOut, "[stderr]\n%s\n", trimmedErr)
	}
}

// StartSpinner starts a terminal spinner with the provided message.
func (o *Output) StartSpinner(message string) *spinner.Spinner {
	spin := spinner.New(
		spinner.CharSets[14],
		100*time.Millisecond,
		spinner.WithWriter(o.out),
	)
	spin.Suffix = " " + message
	spin.Start()
	return spin
}

// StopSpinner stops a spinner if it was previously started.
func (o *Output) StopSpinner(spin *spinner.Spinner) {
	if spin != nil {
		spin.Stop()
	}
}

// PrintServersTable renders a table for server list command output.
func (o *Output) PrintServersTable(servers []ServerInfo) {
	writer := tabwriter.NewWriter(o.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "NAME\tHOST\tPORT\tUSER\tAUTH")
	for _, server := range servers {
		_, _ = fmt.Fprintf(
			writer,
			"%s\t%s\t%d\t%s\t%s\n",
			server.Name,
			server.Host,
			server.Port,
			server.User,
			server.AuthType,
		)
	}
	_ = writer.Flush()
}
