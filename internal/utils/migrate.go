package utils

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

func RunGitHubImport(exportFlags *data.CmdExportFlags, migrateFlags *data.CmdMigrateFlags, archivePath string, logger *zap.Logger) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI not found. Please install it: https://cli.github.com")
	}

	checkCmd := exec.Command("gh", "gei", "--version")
	if err := checkCmd.Run(); err != nil {
		logger.Info("Installing GitHub Enterprise Importer extension...")
		installCmd := exec.Command("gh", "extension", "install", "github/gh-gei")
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install GEI extension: %w", err)
		}
	}

	if migrateFlags.TargetRepo == "" {
		migrateFlags.TargetRepo = exportFlags.Repository
	}

	args := []string{
		"gei", "migrate-repo",
		"--github-source-org", exportFlags.Workspace,
		"--source-repo", exportFlags.Repository,
		"--github-target-org", migrateFlags.TargetOrg,
		"--target-repo", migrateFlags.TargetRepo,
		"--git-archive-path", archivePath,
		"--metadata-archive-path", archivePath,
	}

	if migrateFlags.UseGitHubStorage {
		args = append(args, "--use-github-storage")
	}

	env := os.Environ()
	if migrateFlags.GitHubPAT != "" {
		env = append(env, fmt.Sprintf("GH_PAT=%s", migrateFlags.GitHubPAT))
	} else if pat := os.Getenv("GITHUB_PAT"); pat != "" {
		env = append(env, fmt.Sprintf("GH_PAT=%s", pat))
	}

	cmd := exec.Command("gh", args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("Running GitHub Enterprise Importer...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GEI migration failed: %w", err)
	}

	return nil
}
