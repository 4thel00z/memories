package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/4thel00z/memories/internal"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// providerPrompter interactively collects provider connection details for
// any fields not already supplied (via flags). The api key is read through
// readSecret so callers can hide terminal echo; tests inject a fake.
type providerPrompter struct {
	in         *bufio.Reader
	out        io.Writer
	readSecret func(prompt string) (string, error)
}

func (p providerPrompter) fill(cfg internal.ProviderConfig) (internal.ProviderConfig, error) {
	if cfg.BaseURL == "" {
		v, err := p.readLine("Base URL: ")
		if err != nil {
			return cfg, err
		}
		cfg.BaseURL = v
	}
	if cfg.Model == "" {
		v, err := p.readLine("Model: ")
		if err != nil {
			return cfg, err
		}
		cfg.Model = v
	}
	if cfg.APIKey == "" {
		v, err := p.readSecret("API key: ")
		if err != nil {
			return cfg, err
		}
		cfg.APIKey = v
	}
	return cfg, nil
}

func (p providerPrompter) readLine(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)
	line, err := p.in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

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

// maskSecret renders an API key for display without leaking it. Short keys
// are hidden entirely; longer keys reveal only a 4-char prefix and suffix.
func maskSecret(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// orNotSet returns s, or a placeholder when it is empty.
func orNotSet(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func newProviderListCmd(listUC *internal.ProviderListUseCase) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			items, err := listUC.Execute(internal.ProviderInput{Scope: scopeHint})
			if err != nil {
				return fmt.Errorf("list providers: %w", err)
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No providers configured.")
				return nil
			}

			out := cmd.OutOrStdout()
			for _, item := range items {
				label := item.Name
				if item.IsDefault {
					label += " (default)"
				}
				fmt.Fprintln(out, label)
				fmt.Fprintf(out, "  base_url: %s\n", orNotSet(item.Config.BaseURL))
				fmt.Fprintf(out, "  model:    %s\n", orNotSet(item.Config.Model))
				fmt.Fprintf(out, "  api_key:  %s\n", maskSecret(item.Config.APIKey))
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

			cfg := internal.ProviderConfig{
				APIKey:  apiKey,
				BaseURL: baseURL,
				Model:   model,
			}

			// On an interactive terminal, prompt for any connection details
			// not supplied via flags so we never store an empty provider.
			// Piped/scripted invocations keep using flags verbatim.
			if term.IsTerminal(os.Stdin.Fd()) {
				out := cmd.OutOrStdout()
				p := providerPrompter{
					in:  bufio.NewReader(cmd.InOrStdin()),
					out: out,
					readSecret: func(prompt string) (string, error) {
						fmt.Fprint(out, prompt)
						b, err := term.ReadPassword(os.Stdin.Fd())
						fmt.Fprintln(out)
						if err != nil {
							return "", err
						}
						return strings.TrimSpace(string(b)), nil
					},
				}
				var err error
				if cfg, err = p.fill(cfg); err != nil {
					return fmt.Errorf("read provider config: %w", err)
				}
			}

			if err := addUC.Execute(internal.ProviderInput{
				Name:   name,
				Scope:  scopeHint,
				Config: cfg,
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
