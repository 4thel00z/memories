package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewSearchCmd(svc func() *internal.SearchService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search memories",
		Long:  `Search memories by keyword or semantic similarity.`,
		Args:  cobra.ExactArgs(1),
		RunE:  makeSearchRunner(svc),
	}

	cmd.Flags().BoolP("semantic", "s", false, "Use semantic search")
	cmd.Flags().IntP("number", "n", 10, "Maximum results")
	return cmd
}

func makeSearchRunner(svc func() *internal.SearchService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		query := args[0]
		semantic, _ := cmd.Flags().GetBool("semantic")
		limit, _ := cmd.Flags().GetInt("number")
		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		if semantic {
			return runSemanticSearch(cmd, svc(), query, limit, scopeHint, asJSON)
		}
		return runKeywordSearch(cmd, svc(), query, scopeHint, asJSON)
	}
}

func runKeywordSearch(cmd *cobra.Command, svc *internal.SearchService, query, scopeHint string, asJSON bool) error {
	memories, err := svc.Keyword(cmd.Context(), query, scopeHint)
	if err != nil {
		return fmt.Errorf("keyword search: %w", err)
	}

	if asJSON {
		return outputMemoriesJSON(cmd, memories)
	}

	for _, mem := range memories {
		fmt.Fprintln(cmd.OutOrStdout(), mem.Key.String())
	}
	return nil
}

func runSemanticSearch(cmd *cobra.Command, svc *internal.SearchService, query string, limit int, scopeHint string, asJSON bool) error {
	results, err := svc.Semantic(cmd.Context(), query, limit, scopeHint)
	if err != nil {
		return fmt.Errorf("semantic search: %w", err)
	}

	if asJSON {
		return outputSearchResultsJSON(cmd, results)
	}

	for _, r := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "%.4f  %s\n", r.Score, r.Key.String())
	}
	return nil
}

func outputSearchResultsJSON(cmd *cobra.Command, results []internal.SearchResult) error {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"key":   r.Key.String(),
			"score": r.Score,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
