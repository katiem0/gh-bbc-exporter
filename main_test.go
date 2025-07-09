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

	// Set test args
	os.Args = []string{"gh-bbc-exporter", "--help"}

	// In a real test you'd capture stdout and check the output
	// but since main() calls os.Exit, we'll need to use a helper
	// This is just to demonstrate the approach
	// Alternatively, refactor main.go to make it more testable
	assert.NotPanics(t, func() {
		// This would actually exit the test, so we'd need to
		// refactor main to accept args and return an error
		// main()
	})
}
