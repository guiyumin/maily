package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const configFileName = "config.json"

type Config struct {
	MaxEmails    int    `json:"max_emails"`
	DefaultLabel string `json:"default_label"`
	Theme        string `json:"theme"`
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
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
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
	data, err := json.MarshalIndent(c, "", "  ")
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
