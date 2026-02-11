package internal

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type EmbeddingsConfig struct {
	Backend   string `yaml:"backend"`
	Model     string `yaml:"model"`
	Dimension int    `yaml:"dimension"`
}

type ProviderConfig struct {
	APIKey  string `yaml:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty"`
	Model   string `yaml:"model"`
}

type Config struct {
	Embeddings      EmbeddingsConfig          `yaml:"embeddings"`
	Providers       map[string]ProviderConfig `yaml:"providers,omitempty"`
	DefaultProvider string                    `yaml:"default_provider,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Embeddings: EmbeddingsConfig{
			Backend:   "gollama",
			Model:     DefaultModelFilename,
			Dimension: 384,
		},
		Providers: make(map[string]ProviderConfig),
	}
}

func LoadConfig(scope Scope) (*Config, error) {
	path := scope.ConfigPath()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}

	return &cfg, nil
}

func SaveConfig(scope Scope, cfg *Config) error {
	path := scope.ConfigPath()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
