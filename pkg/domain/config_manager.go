package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ExecutorConfig struct {
	ExecutorID       string                `json:"executor_id,omitempty"`
	ExecutorName     string                `json:"executor_name"`
	Address          string                `json:"address"`
	Assignments      []WorkspaceAssignment `json:"workspace_assignments,omitempty"`
	SetupComplete    bool                  `json:"setup_complete"`
	Keys             CryptoKeys            `json:"keys"`
	APIBaseURL       string                `json:"api_url"`
	VerificationCode string                `json:"verification_code,omitempty"`
	LastConnected    time.Time             `json:"last_connected,omitempty"`
}

type CryptoKeys struct {
	X25519Private  string `json:"x25519_private"`
	X25519Public   string `json:"x25519_public"`
	Ed25519Private string `json:"ed25519_private"`
	Ed25519Public  string `json:"ed25519_public"`
}

type ConfigManager interface {
	IsSetupComplete(ctx context.Context) bool
	GetConfig(ctx context.Context) (ExecutorConfig, error)
	SaveConfig(ctx context.Context, config ExecutorConfig) error
	ResetConfig(ctx context.Context) error
}

const (
	ConfigFileName = "config.json"
)

type configManager struct {
	config     ExecutorConfig
	configPath string
}

func NewConfigManager() (ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".flowbaker")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, ConfigFileName)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &configManager{
			config:     ExecutorConfig{},
			configPath: configPath,
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ExecutorConfig

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &configManager{
		config:     config,
		configPath: configPath,
	}, nil
}

func (m *configManager) GetConfig(ctx context.Context) (ExecutorConfig, error) {
	return m.config, nil
}

func (m *configManager) SaveConfig(ctx context.Context, config ExecutorConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	m.config = config

	return nil
}

func (m *configManager) IsSetupComplete(ctx context.Context) bool {
	return m.config.SetupComplete
}

func (m *configManager) ResetConfig(ctx context.Context) error {
	if err := os.Remove(m.configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	m.config = ExecutorConfig{}

	return nil
}
