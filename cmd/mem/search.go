package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewSearchCmd(keywordUC *internal.KeywordSearchUseCase, semanticUC *internal.SemanticSearchUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search memories",
		Long:  `Search memories by keyword or semantic similarity.`,
		Args:  cobra.ExactArgs(1),
		RunE:  makeSearchRunner(keywordUC, semanticUC),
	}

	cmd.Flags().BoolP("semantic", "s", false, "Use semantic search")
	cmd.Flags().IntP("number", "n", 10, "Maximum results")
	return cmd
}

func makeSearchRunner(keywordUC *internal.KeywordSearchUseCase, semanticUC *internal.SemanticSearchUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		query := args[0]
		semantic, _ := cmd.Flags().GetBool("semantic")
		limit, _ := cmd.Flags().GetInt("number")
		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		if semantic {
			return runSemanticSearch(cmd, semanticUC, query, limit, scopeHint, asJSON)
		}
		return runKeywordSearch(cmd, keywordUC, query, scopeHint, asJSON)
	}
}

func runKeywordSearch(cmd *cobra.Command, keywordUC *internal.KeywordSearchUseCase, query, scopeHint string, asJSON bool) error {
	out, err := keywordUC.Execute(cmd.Context(), internal.SearchInput{
		Query: query, Scope: scopeHint,
	})
	if err != nil {
		return fmt.Errorf("keyword search: %w", err)
	}

	if asJSON {
		return outputSearchResultsJSON(cmd, out.Results)
	}

	for _, r := range out.Results {
		fmt.Fprintln(cmd.OutOrStdout(), r.Key)
	}
	return nil
}

func runSemanticSearch(cmd *cobra.Command, semanticUC *internal.SemanticSearchUseCase, query string, limit int, scopeHint string, asJSON bool) error {
	out, err := semanticUC.Execute(cmd.Context(), internal.SearchInput{
		Query: query, Limit: limit, Scope: scopeHint,
	})
	if err != nil {
		return fmt.Errorf("semantic search: %w", err)
	}

	if asJSON {
		return outputSearchResultsJSON(cmd, out.Results)
	}

	for _, r := range out.Results {
		fmt.Fprintf(cmd.OutOrStdout(), "%.4f  %s\n", r.Score, r.Key)
	}
	return nil
}

func outputSearchResultsJSON(cmd *cobra.Command, results []internal.SearchResultOutput) error {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"key":   r.Key,
			"score": r.Score,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
