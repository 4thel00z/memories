package main

import (
	"fmt"
	"os"
	"path/filepath"

	memories "github.com/4thel00z/memories"
	"github.com/spf13/cobra"
)

func NewSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage Claude Code skills",
		Long:  `Install and manage Claude Code skills for use with mem.`,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}

	cmd.AddCommand(newSkillInstallCmd())
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the using-mem skill into .claude/skills/",
		Long:  `Installs the bundled using-mem Claude Code skill into the current project's .claude/skills/ directory.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			dir := filepath.Join(cwd, ".claude", "skills", "using-mem")
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create skill directory: %w", err)
			}

			data, err := memories.Skills.ReadFile("skills/using-mem/SKILL.md")
			if err != nil {
				return fmt.Errorf("read embedded skill: %w", err)
			}

			dest := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(dest, data, 0644); err != nil {
				return fmt.Errorf("write skill file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed using-mem skill to %s\n", dest)
			return nil
		},
	}
}
