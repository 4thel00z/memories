package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewProviderCmd(svc func() *internal.ProviderService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage LLM providers",
		Long:  `List, add, remove, and test LLM providers.`,
	}

	cmd.AddCommand(
		newProviderListCmd(svc),
		newProviderAddCmd(svc),
		newProviderRemoveCmd(svc),
		newProviderDefaultCmd(svc),
		newProviderTestCmd(svc),
	)

	return cmd
}

func newProviderListCmd(svc func() *internal.ProviderService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			names, err := svc().List(scopeHint)
			if err != nil {
				return fmt.Errorf("list providers: %w", err)
			}

			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No providers configured.")
				return nil
			}

			for _, name := range names {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			return nil
		},
	}
}

func newProviderAddCmd(svc func() *internal.ProviderService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			scopeHint, _ := cmd.Flags().GetString("scope")
			apiKey, _ := cmd.Flags().GetString("api-key")
			baseURL, _ := cmd.Flags().GetString("base-url")
			model, _ := cmd.Flags().GetString("model")

			cfg := internal.ProviderConfig{
				APIKey:  apiKey,
				BaseURL: baseURL,
				Model:   model,
			}

			if err := svc().Add(name, cfg, scopeHint); err != nil {
				return fmt.Errorf("add provider: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added provider %s\n", name)
			return nil
		},
	}

	cmd.Flags().String("api-key", "", "API key")
	cmd.Flags().String("base-url", "", "Base URL")
	cmd.Flags().String("model", "", "Model name")
	return cmd
}

func newProviderRemoveCmd(svc func() *internal.ProviderService) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			if err := svc().Remove(args[0], scopeHint); err != nil {
				return fmt.Errorf("remove provider: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed provider %s\n", args[0])
			return nil
		},
	}
}

func newProviderDefaultCmd(svc func() *internal.ProviderService) *cobra.Command {
	return &cobra.Command{
		Use:   "default <name>",
		Short: "Set default provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			if err := svc().SetDefault(args[0], scopeHint); err != nil {
				return fmt.Errorf("set default: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default provider set to %s\n", args[0])
			return nil
		},
	}
}

func newProviderTestCmd(svc func() *internal.ProviderService) *cobra.Command {
	return &cobra.Command{
		Use:   "test <name>",
		Short: "Test provider connectivity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			if err := svc().Test(cmd.Context(), args[0], scopeHint); err != nil {
				return fmt.Errorf("test provider: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Provider %s is working\n", args[0])
			return nil
		},
	}
}
