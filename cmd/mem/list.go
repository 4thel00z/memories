package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewListCmd(svc func() *internal.MemoryService) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [prefix]",
		Aliases: []string{"ls"},
		Short:   "List memories",
		Long:    `List all memories, optionally filtered by prefix.`,
		Args:    cobra.MaximumNArgs(1),
		RunE:    makeListRunner(svc),
	}

	return cmd
}

func makeListRunner(svc func() *internal.MemoryService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		prefix := ""
		if len(args) > 0 {
			prefix = args[0]
		}

		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		memories, err := svc().List(cmd.Context(), prefix, scopeHint)
		if err != nil {
			return fmt.Errorf("list memories: %w", err)
		}

		if asJSON {
			return outputMemoriesJSON(cmd, memories)
		}

		for _, mem := range memories {
			fmt.Fprintln(cmd.OutOrStdout(), mem.Key.String())
		}
		return nil
	}
}

func outputMemoriesJSON(cmd *cobra.Command, memories []*internal.Memory) error {
	out := make([]map[string]any, 0, len(memories))
	for _, mem := range memories {
		out = append(out, map[string]any{
			"key":        mem.Key.String(),
			"created_at": mem.CreatedAt,
			"updated_at": mem.UpdatedAt,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
