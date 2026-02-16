package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewSummarizeCmd(summarizeUC *internal.SummarizeUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize [prefix]",
		Short: "Summarize memories using AI",
		Long:  `Generate an AI-powered summary of memories, optionally filtered by prefix.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeSummarizeRunner(summarizeUC),
	}

	return cmd
}

func makeSummarizeRunner(summarizeUC *internal.SummarizeUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		prefix := ""
		if len(args) > 0 {
			prefix = args[0]
		}

		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		out, err := summarizeUC.Execute(cmd.Context(), internal.SummarizeInput{
			Prefix: prefix, Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("summarize: %w", err)
		}

		if asJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "# %s\n\n%s\n", out.Title, out.Overview)
		if len(out.KeyPoints) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\nKey Points:")
			for _, p := range out.KeyPoints {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
			}
		}
		if len(out.Tags) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nTags: %v\n", out.Tags)
		}
		return nil
	}
}
