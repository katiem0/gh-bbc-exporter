package main

import (
	"os"

	"github.com/katiem0/gh-bbc-exporter/cmd"
)

var osExit = os.Exit

func main() {
	cmd := cmd.NewCmdRoot()
	if err := cmd.Execute(); err != nil {
		osExit(1)
	}
}
