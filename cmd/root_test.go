package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewCmdRoot(t *testing.T) {
	cmd := NewCmdRoot()

	assert.NotNil(t, cmd)
	assert.Equal(t, "bbc-exporter", cmd.Use)
	assert.Equal(t, "Export and migrate repositories from Bitbucket Cloud to GitHub", cmd.Short)
}

func TestNewCmdRootHasExportSubcommand(t *testing.T) {
	cmd := NewCmdRoot()

	var exportCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Use == "export [flags]" {
			exportCmd = subCmd
			break
		}
	}

	assert.NotNil(t, exportCmd, "export subcommand should exist")
}

func TestNewCmdRootHasMigrateSubcommand(t *testing.T) {
	cmd := NewCmdRoot()

	var migrateCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Use == "migrate [flags]" {
			migrateCmd = subCmd
			break
		}
	}

	assert.NotNil(t, migrateCmd, "migrate subcommand should exist")
}

func TestNewCmdRootHasHelpFlag(t *testing.T) {
	cmd := NewCmdRoot()

	helpFlag := cmd.PersistentFlags().Lookup("help")
	assert.NotNil(t, helpFlag, "help flag should be defined")
	assert.Equal(t, "false", helpFlag.DefValue)
}

func TestNewCmdRootCompletionDisabled(t *testing.T) {
	cmd := NewCmdRoot()

	assert.True(t, cmd.CompletionOptions.DisableDefaultCmd,
		"Default completion command should be disabled")
}

func TestNewCmdRootHelpCommandHidden(t *testing.T) {
	cmd := NewCmdRoot()

	var helpCmd *cobra.Command
	for _, subCmd := range cmd.Commands() {
		if subCmd.Use == "no-help" {
			helpCmd = subCmd
			break
		}
	}

	helpFlag := cmd.PersistentFlags().Lookup("help")
	assert.NotNil(t, helpFlag, "help flag should be defined")

	if helpCmd != nil {
		assert.True(t, helpCmd.Hidden, "Help command should be hidden")
	}
}

func TestNewCmdRootSubcommandCount(t *testing.T) {
	cmd := NewCmdRoot()

	// Should have exactly 2 subcommands: export and migrate
	assert.Equal(t, 2, len(cmd.Commands()), "Root command should have 2 subcommands")
}

func TestNewCmdRootExecuteHelp(t *testing.T) {
	cmd := NewCmdRoot()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "bbc-exporter")
	assert.Contains(t, output, "export")
	assert.Contains(t, output, "migrate")
}

func TestNewCmdRootExecuteNoArgs(t *testing.T) {
	cmd := NewCmdRoot()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.NoError(t, err)

	// Should show help/usage when no args provided
	output := buf.String()
	assert.Contains(t, output, "bbc-exporter")
}

func TestNewCmdRootInvalidSubcommand(t *testing.T) {
	cmd := NewCmdRoot()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"invalid-command"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestNewCmdRootExportSubcommandHelp(t *testing.T) {
	cmd := NewCmdRoot()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"export", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "export")
	assert.Contains(t, output, "workspace")
	assert.Contains(t, output, "repo")
}

func TestNewCmdRootMigrateSubcommandHelp(t *testing.T) {
	cmd := NewCmdRoot()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"migrate", "--help"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "migrate")
	assert.Contains(t, output, "target-org")
	assert.Contains(t, output, "GitHub")
}

func TestNewCmdRootSubcommandNames(t *testing.T) {
	cmd := NewCmdRoot()

	subcommandNames := make([]string, 0)
	for _, subCmd := range cmd.Commands() {
		subcommandNames = append(subcommandNames, subCmd.Name())
	}

	assert.Contains(t, subcommandNames, "export")
	assert.Contains(t, subcommandNames, "migrate")
}

func TestNewCmdRootNoRunFunction(t *testing.T) {
	cmd := NewCmdRoot()

	// Root command should not have a Run function - it's just a container
	assert.Nil(t, cmd.Run, "Root command should not have Run function")
	assert.Nil(t, cmd.RunE, "Root command should not have RunE function")
}

func TestNewCmdRootPersistentFlagsInheritance(t *testing.T) {
	cmd := NewCmdRoot()

	// Help flag should be available on root
	helpFlag := cmd.PersistentFlags().Lookup("help")
	assert.NotNil(t, helpFlag)

	// Subcommands should inherit persistent flags
	for _, subCmd := range cmd.Commands() {
		inheritedHelp := subCmd.InheritedFlags().Lookup("help")
		assert.NotNil(t, inheritedHelp,
			"Subcommand %s should inherit help flag", subCmd.Name())
	}
}
