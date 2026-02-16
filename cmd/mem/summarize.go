package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewSummarizeCmd(svc func() *internal.SummarizeService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize [prefix]",
		Short: "Summarize memories using AI",
		Long:  `Generate an AI-powered summary of memories, optionally filtered by prefix.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeSummarizeRunner(svc),
	}

	return cmd
}

func makeSummarizeRunner(svc func() *internal.SummarizeService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		prefix := ""
		if len(args) > 0 {
			prefix = args[0]
		}

		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		summary, err := svc().Summarize(cmd.Context(), prefix, scopeHint)
		if err != nil {
			return fmt.Errorf("summarize: %w", err)
		}

		if asJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(summary)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "# %s\n\n%s\n", summary.Title, summary.Overview)
		if len(summary.KeyPoints) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\nKey Points:")
			for _, p := range summary.KeyPoints {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
			}
		}
		if len(summary.Tags) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTags: %v\n", summary.Tags)
		}
		return nil
	}
}
