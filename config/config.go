package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultModel string            `yaml:"default_model"`
	Temperature  float64           `yaml:"temperature"`
	SystemPrompt string            `yaml:"system_prompt"`
	Providers    map[string]Provider `yaml:"providers"`
}

type Provider struct {
	Type    string   `yaml:"type"`
	BaseURL string   `yaml:"base_url"`
	APIKey  string   `yaml:"api_key"`
	Models  []string `yaml:"models"`
}

func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "nexus")
	}
	return filepath.Join(home, ".config", "nexus")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func Load() (*Config, error) {
	path := ConfigPath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(ConfigPath(), data, 0644)
}

func DefaultConfig() *Config {
	return &Config{
		DefaultModel: "gemma3:1b",
		Temperature:  0.7,
		SystemPrompt: "You are Nexus, a helpful AI assistant running in the terminal.",
		Providers: map[string]Provider{
			"ollama": {
				Type:    "ollama",
				BaseURL: "http://localhost:11434",
				Models:  []string{"gemma3:1b", "llama3.2", "qwen2.5-coder"},
			},
			"openai": {
				Type:    "openai",
				BaseURL: "https://api.openai.com",
				Models:  []string{"gpt-4o", "gpt-4o-mini", "o1-mini"},
			},
			"anthropic": {
				Type:    "anthropic",
				BaseURL: "https://api.anthropic.com",
				Models:  []string{"claude-sonnet-4-20250514", "claude-3-5-haiku-20241022"},
			},
			"lmstudio": {
				Type:    "lmstudio",
				BaseURL: "http://localhost:1234",
				Models:  []string{},
			},
		},
	}
}

func (c *Config) GetProvider(name string) (Provider, bool) {
	p, ok := c.Providers[name]
	return p, ok
}

func (c *Config) ResolveModel(model string) (provider string, baseURL string, modelName string) {
	if model != "" {
		for name, p := range c.Providers {
			for _, m := range p.Models {
				if m == model {
					return name, p.BaseURL, model
				}
			}
		}
	}

	if c.DefaultModel != "" {
		for name, p := range c.Providers {
			for _, m := range p.Models {
				if m == c.DefaultModel {
					return name, p.BaseURL, c.DefaultModel
				}
			}
		}
	}

	// fallback to ollama
	if p, ok := c.Providers["ollama"]; ok && len(p.Models) > 0 {
		return "ollama", p.BaseURL, p.Models[0]
	}

	return "", "", ""
}
