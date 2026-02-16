package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewLogCmd(logUC *internal.LogUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show commit history",
		Long:  `Show the commit history for the memory store.`,
		RunE:  makeLogRunner(logUC),
	}

	cmd.Flags().IntP("number", "n", 10, "Limit number of commits")
	cmd.Flags().Bool("oneline", false, "Show each commit on one line")
	return cmd
}

func makeLogRunner(logUC *internal.LogUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("number")
		oneline, _ := cmd.Flags().GetBool("oneline")
		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		out, err := logUC.Execute(cmd.Context(), internal.LogInput{
			Limit: limit, Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("get log: %w", err)
		}

		if asJSON {
			return outputCommitsJSON(cmd, out.Commits)
		}

		for _, c := range out.Commits {
			if oneline {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", c.Hash[:7], c.Message)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "commit %s\n", c.Hash)
				fmt.Fprintf(cmd.OutOrStdout(), "Date:   %s\n\n", c.Timestamp.Format("Mon Jan 2 15:04:05 2006 -0700"))
				fmt.Fprintf(cmd.OutOrStdout(), "    %s\n\n", c.Message)
			}
		}
		return nil
	}
}

func outputCommitsJSON(cmd *cobra.Command, commits []internal.CommitOutput) error {
	out := make([]map[string]any, 0, len(commits))
	for _, c := range commits {
		out = append(out, map[string]any{
			"hash":      c.Hash,
			"message":   c.Message,
			"timestamp": c.Timestamp,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
