package internal

import (
	"context"
	"fmt"
	"reflect"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"charm.land/fantasy/schema"
)

type FantasyConfig struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

var _ Provider = (*FantasyProvider)(nil)

type FantasyProvider struct {
	model fantasy.LanguageModel
	name  string
}

func NewFantasyProvider(ctx context.Context, cfg FantasyConfig) (*FantasyProvider, error) {
	var provider fantasy.Provider
	var err error

	switch cfg.Provider {
	case "openai":
		opts := []openai.Option{openai.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
		}
		provider, err = openai.New(opts...)

	case "anthropic":
		opts := []anthropic.Option{anthropic.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(cfg.BaseURL))
		}
		provider, err = anthropic.New(opts...)

	case "openrouter":
		opts := []openrouter.Option{openrouter.WithAPIKey(cfg.APIKey)}
		provider, err = openrouter.New(opts...)

	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	model, err := provider.LanguageModel(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("get language model: %w", err)
	}

	return &FantasyProvider{
		model: model,
		name:  cfg.Provider,
	}, nil
}

func (p *FantasyProvider) Complete(ctx context.Context, prompt string) (string, error) {
	agent := fantasy.NewAgent(p.model)

	result, err := agent.Generate(ctx, fantasy.AgentCall{
		Prompt: prompt,
	})
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	return result.Response.Content.Text(), nil
}

func (p *FantasyProvider) GenerateObject(ctx context.Context, prompt string, target any) error {
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	s := schema.Generate(t)

	call := fantasy.ObjectCall{
		Prompt: fantasy.Prompt{fantasy.NewUserMessage(prompt)},
		Schema: s,
	}

	resp, err := p.model.GenerateObject(ctx, call)
	if err != nil {
		return fmt.Errorf("generate object: %w", err)
	}

	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	objVal := reflect.ValueOf(resp.Object)
	if objVal.IsValid() && objVal.Type().AssignableTo(targetVal.Elem().Type()) {
		targetVal.Elem().Set(objVal)
	}

	return nil
}

func (p *FantasyProvider) Stream(ctx context.Context, prompt string) (<-chan string, error) {
	agent := fantasy.NewAgent(p.model)

	ch := make(chan string, 100)

	go func() {
		defer close(ch)

		_, err := agent.Stream(ctx, fantasy.AgentStreamCall{
			Prompt: prompt,
			OnTextDelta: func(_, text string) error {
				if text != "" {
					ch <- text
				}
				return nil
			},
		})
		if err != nil {
			ch <- fmt.Sprintf("\n[error: %v]", err)
		}
	}()

	return ch, nil
}
