package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewGetCmd(getUC *internal.GetMemoryUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Retrieve a memory",
		Long:  `Retrieve and display the content of a memory.`,
		Args:  cobra.ExactArgs(1),
		RunE:  makeGetRunner(getUC),
	}

	return cmd
}

func makeGetRunner(getUC *internal.GetMemoryUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]
		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		out, err := getUC.Execute(cmd.Context(), internal.GetMemoryInput{
			Key: key, Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("get memory: %w", err)
		}

		if asJSON {
			return outputGetMemoryJSON(cmd, out)
		}

		fmt.Fprint(cmd.OutOrStdout(), out.Content)
		return nil
	}
}

func outputGetMemoryJSON(cmd *cobra.Command, out *internal.GetMemoryOutput) error {
	data := map[string]any{
		"key":        out.Key,
		"content":    out.Content,
		"created_at": out.CreatedAt,
		"updated_at": out.UpdatedAt,
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
