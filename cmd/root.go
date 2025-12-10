package cmd

import (
	"github.com/katiem0/gh-bbc-exporter/cmd/export"
	"github.com/katiem0/gh-bbc-exporter/cmd/migrate"
	"github.com/spf13/cobra"
)

func NewCmdRoot() *cobra.Command {

	cmdRoot := &cobra.Command{
		Use:   "bbc-exporter",
		Short: "Export and migrate repositories from Bitbucket Cloud to GitHub",
	}
	cmdRoot.PersistentFlags().Bool("help", false, "Show help for command")

	cmdRoot.AddCommand(export.NewCmdExport())
	cmdRoot.AddCommand(migrate.NewCmdMigrate())
	cmdRoot.CompletionOptions.DisableDefaultCmd = true
	cmdRoot.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
	return cmdRoot
}
