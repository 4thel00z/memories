package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new memory store",
		Long:  `Initialize a new .mem directory with git-based storage.`,
		RunE:  runInit,
	}

	cmd.Flags().Bool("global", false, "Initialize global scope (~/.mem)")
	return cmd
}

func runInit(cmd *cobra.Command, _ []string) error {
	isGlobal, _ := cmd.Flags().GetBool("global")

	resolver := internal.NewScopeResolver()

	var scope internal.Scope
	if isGlobal {
		scope = resolver.Global()
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		scope = internal.Scope{
			Type:    internal.ScopeProject,
			Path:    cwd,
			MemPath: filepath.Join(cwd, ".mem"),
		}
	}

	if _, err := os.Stat(scope.MemPath); err == nil {
		return fmt.Errorf("already initialized at %s", scope.MemPath)
	}

	if err := os.MkdirAll(scope.VectorPath(), 0755); err != nil {
		return fmt.Errorf("create vectors directory: %w", err)
	}

	if err := internal.InitRepository(scope); err != nil {
		return fmt.Errorf("init repository: %w", err)
	}

	cfg := internal.DefaultConfig()
	if err := internal.SaveConfig(scope, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized memory store at %s\n", scope.MemPath)
	return nil
}
