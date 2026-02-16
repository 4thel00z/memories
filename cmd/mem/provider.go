package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewProviderCmd(
	listUC *internal.ProviderListUseCase,
	addUC *internal.ProviderAddUseCase,
	removeUC *internal.ProviderRemoveUseCase,
	setDefUC *internal.ProviderSetDefaultUseCase,
	testUC *internal.ProviderTestUseCase,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage LLM providers",
		Long:  `List, add, remove, and test LLM providers.`,
	}

	cmd.AddCommand(
		newProviderListCmd(listUC),
		newProviderAddCmd(addUC),
		newProviderRemoveCmd(removeUC),
		newProviderDefaultCmd(setDefUC),
		newProviderTestCmd(testUC),
	)

	return cmd
}

func newProviderListCmd(listUC *internal.ProviderListUseCase) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			names, err := listUC.Execute(internal.ProviderInput{Scope: scopeHint})
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

func newProviderAddCmd(addUC *internal.ProviderAddUseCase) *cobra.Command {
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

			if err := addUC.Execute(internal.ProviderInput{
				Name:  name,
				Scope: scopeHint,
				Config: internal.ProviderConfig{
					APIKey:  apiKey,
					BaseURL: baseURL,
					Model:   model,
				},
			}); err != nil {
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

func newProviderRemoveCmd(removeUC *internal.ProviderRemoveUseCase) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			if err := removeUC.Execute(internal.ProviderInput{Name: args[0], Scope: scopeHint}); err != nil {
				return fmt.Errorf("remove provider: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed provider %s\n", args[0])
			return nil
		},
	}
}

func newProviderDefaultCmd(setDefUC *internal.ProviderSetDefaultUseCase) *cobra.Command {
	return &cobra.Command{
		Use:   "default <name>",
		Short: "Set default provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			if err := setDefUC.Execute(internal.ProviderInput{Name: args[0], Scope: scopeHint}); err != nil {
				return fmt.Errorf("set default: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default provider set to %s\n", args[0])
			return nil
		},
	}
}

func newProviderTestCmd(testUC *internal.ProviderTestUseCase) *cobra.Command {
	return &cobra.Command{
		Use:   "test <name>",
		Short: "Test provider connectivity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			if err := testUC.Execute(cmd.Context(), internal.ProviderInput{Name: args[0], Scope: scopeHint}); err != nil {
				return fmt.Errorf("test provider: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Provider %s is working\n", args[0])
			return nil
		},
	}
}
