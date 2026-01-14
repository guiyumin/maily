package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFileName = "config.yml"

// AIProviderType represents the type of AI provider
type AIProviderType string

const (
	AIProviderTypeCLI AIProviderType = "cli" // CLI tool (codex, gemini, claude, vibe, ollama)
	AIProviderTypeAPI AIProviderType = "api" // OpenAI-compatible HTTP API
)

// AIProvider represents a unified AI provider configuration
// Providers are tried in order from first to last
type AIProvider struct {
	Type    AIProviderType `yaml:"type"`               // "cli" or "api"
	Name    string         `yaml:"name"`               // CLI name (codex, gemini, claude) or friendly name for API
	Model   string         `yaml:"model"`              // model to use (required)
	BaseURL string         `yaml:"base_url,omitempty"` // API base URL (required for type: api)
	APIKey  string         `yaml:"api_key,omitempty"`  // API key (required for type: api)
}

// NativeNotificationConfig configures native OS notifications
type NativeNotificationConfig struct {
	Enabled          bool `yaml:"enabled" json:"enabled"`
	NewEmail         bool `yaml:"new_email" json:"new_email"`
	CalendarReminder bool `yaml:"calendar_reminder" json:"calendar_reminder"`
}

// TelegramConfig configures Telegram bot notifications
type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	BotToken string `yaml:"bot_token" json:"bot_token"`
	ChatID   string `yaml:"chat_id" json:"chat_id"`
}

// NotificationConfig configures all notification channels
type NotificationConfig struct {
	Native   NativeNotificationConfig `yaml:"native" json:"native"`
	Telegram *TelegramConfig          `yaml:"telegram,omitempty" json:"telegram,omitempty"`
}

// GitHubConfig configures GitHub integration
type GitHubConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Token       string `yaml:"token" json:"token"`
	ParseEmails bool   `yaml:"parse_emails" json:"parse_emails"`
}

// IntegrationsConfig configures external service integrations
type IntegrationsConfig struct {
	GitHub *GitHubConfig `yaml:"github,omitempty" json:"github,omitempty"`
}

type Config struct {
	MaxEmails    int    `yaml:"max_emails" json:"max_emails"`
	DefaultLabel string `yaml:"default_label" json:"default_label"`
	Theme        string `yaml:"theme" json:"theme"`
	Language     string `yaml:"language,omitempty" json:"language,omitempty"` // Language code (en, ko, ja, etc.) - empty means auto-detect

	// AI providers - tried in order from first to last
	// Each provider can be a CLI tool or an OpenAI-compatible API
	AIProviders []AIProvider `yaml:"ai_providers,omitempty" json:"ai_providers,omitempty"`

	// Notification settings
	Notifications *NotificationConfig `yaml:"notifications,omitempty" json:"notifications,omitempty"`

	// External integrations
	Integrations *IntegrationsConfig `yaml:"integrations,omitempty" json:"integrations,omitempty"`
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
