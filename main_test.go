package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	code := m.Run()
	cleanupTestExportDirs()
	os.Exit(code)
}

func TestMainWithHelpFlag(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"gh-bbc-exporter", "--help"}

	assert.NotPanics(t, func() {
		main()
	})
}

func TestMainWithInvalidArgs(t *testing.T) {
	oldArgs := os.Args
	oldOsExit := osExit

	var exitCode int
	osExit = func(code int) {
		exitCode = code
	}

	defer func() {
		os.Args = oldArgs
		osExit = oldOsExit
	}()

	os.Args = []string{"gh-bbc-exporter", "--invalid-flag"}

	main()
	assert.Equal(t, 1, exitCode, "Expected exit code 1 on error")
}

func cleanupTestExportDirs() {
	matches, err := filepath.Glob("./bitbucket-export-*")
	if err != nil {
		return
	}

	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil {
			fmt.Printf("Warning: Failed to remove %s: %v\n", match, err)
		}
		archivePath := match + ".tar.gz"
		if _, err := os.Stat(archivePath); err == nil {
			if err := os.Remove(archivePath); err != nil {
				fmt.Printf("Warning: Failed to remove archive %s: %v\n", archivePath, err)
			}
		}
	}
}
