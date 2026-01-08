package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFileName = "config.yml"

// AIAccount represents an OpenAI-compatible API configuration
type AIAccount struct {
	Name    string `yaml:"name,omitempty"`    // friendly name (e.g., "nvidia", "openai")
	BaseURL string `yaml:"base_url"`          // API base URL
	APIKey  string `yaml:"api_key"`           // API key
	Model   string `yaml:"model"`             // model to use
}

type Config struct {
	MaxEmails    int    `yaml:"max_emails"`
	DefaultLabel string `yaml:"default_label"`
	Theme        string `yaml:"theme"`

	// AI API accounts - tried in order from first to last
	AIAccounts []AIAccount `yaml:"ai_accounts,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		MaxEmails:    50,
		DefaultLabel: "INBOX",
		Theme:        "default",
	}
}

func Load() (Config, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return DefaultConfig(), err
	}

	configPath := filepath.Join(configDir, configFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}

	// Apply defaults for zero values
	if cfg.MaxEmails == 0 {
		cfg.MaxEmails = 50
	}
	if cfg.DefaultLabel == "" {
		cfg.DefaultLabel = "INBOX"
	}
	if cfg.Theme == "" {
		cfg.Theme = "default"
	}

	return cfg, nil
}

func (c Config) Save() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, configFileName)
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "maily"), nil
}

func GetConfigDir() (string, error) {
	return getConfigDir()
}
