// ABOUTME: Main CLI application for the Ritual workflow engine
// ABOUTME: Entry point for the Cobra-based command-line interface

package main

import (
	"os"

	"github.com/sarlalian/ritual/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
