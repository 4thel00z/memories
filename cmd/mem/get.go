package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewGetCmd(svc func() *internal.MemoryService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Retrieve a memory",
		Long:  `Retrieve and display the content of a memory.`,
		Args:  cobra.ExactArgs(1),
		RunE:  makeGetRunner(svc),
	}

	return cmd
}

func makeGetRunner(svc func() *internal.MemoryService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]
		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		mem, err := svc().Get(cmd.Context(), key, scopeHint)
		if err != nil {
			return fmt.Errorf("get memory: %w", err)
		}

		if asJSON {
			return outputMemoryJSON(cmd, mem)
		}

		fmt.Fprint(cmd.OutOrStdout(), string(mem.Content))
		return nil
	}
}

func outputMemoryJSON(cmd *cobra.Command, mem *internal.Memory) error {
	out := map[string]any{
		"key":        mem.Key.String(),
		"content":    string(mem.Content),
		"created_at": mem.CreatedAt,
		"updated_at": mem.UpdatedAt,
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
