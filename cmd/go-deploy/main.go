// Package main wires CLI commands for go-deploy.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "command execution failed: %v\n", err)
		os.Exit(1)
	}
}
