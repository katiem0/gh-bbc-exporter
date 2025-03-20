package cmd

import (
	"github.com/spf13/cobra"
)

func NewCmdRoot() *cobra.Command {

	cmdRoot := &cobra.Command{
		Use:   "migrate-rules <command> [flags]",
		Short: "List and create organization and repository rulesets.",
		Long:  "List and create repository/organization level rulesets for repositories in an organization.",
	}

	cmdRoot.AddCommand(listCmd.NewCmdList())
	cmdRoot.CompletionOptions.DisableDefaultCmd = true
	cmdRoot.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
	return cmdRoot
}
