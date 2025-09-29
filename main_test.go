package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMainWithHelpFlag(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Provide --help so cobra prints usage and returns nil (PreRunE not enforced)
	os.Args = []string{"gh-bbc-exporter", "--help"}

	assert.NotPanics(t, func() {
		main() // Executes the help path; should not call os.Exit(1)
	})
}

func TestMainWithInvalidArgs(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	// Save original osExit function
	oldOsExit := osExit

	// Override os.Exit to capture exit code instead of terminating
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		// Don't actually exit
	}

	defer func() {
		os.Args = oldArgs
		osExit = oldOsExit
	}()

	// Provide invalid flags to trigger error
	os.Args = []string{"gh-bbc-exporter", "--invalid-flag"}

	main()
	assert.Equal(t, 1, exitCode, "Expected exit code 1 on error")
}
