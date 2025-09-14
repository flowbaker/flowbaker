package domain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type ExecutorConfig struct {
	ExecutorID   string `mapstructure:"executor_id"`
	ExecutorName string `mapstructure:"executor_name"`
	Address      string `mapstructure:"address"`
	APIBaseURL   string `mapstructure:"api_base_url"`

	// Crypto keys
	X25519PrivateKey  string `mapstructure:"x25519_private_key"`
	X25519PublicKey   string `mapstructure:"x25519_public_key"`
	Ed25519PrivateKey string `mapstructure:"ed25519_private_key"`
	Ed25519PublicKey  string `mapstructure:"ed25519_public_key"`

	// API signature verification
	StaticAPISignaturePublicKey string `mapstructure:"static_api_signature_public_key"`

	// Setup and workspace management
	SetupComplete               bool                  `mapstructure:"setup_complete"`
	WorkspaceAssignments        []WorkspaceAssignment `mapstructure:"workspace_assignments"`
	EnableWorkspaceRegistration bool                  `mapstructure:"enable_workspace_registration"`
	EnableStaticPasscode        bool                  `mapstructure:"enable_static_passcode"`
	StaticPasscode              string                `mapstructure:"static_passcode"`
	SkipWorkspaceAssignments    bool                  `mapstructure:"skip_workspace_assignments"`

	LastConnected string `mapstructure:"last_connected"`
}

func (c ExecutorConfig) Keys() CryptoKeys {
	return CryptoKeys{
		X25519Private:  c.X25519PrivateKey,
		X25519Public:   c.X25519PublicKey,
		Ed25519Private: c.Ed25519PrivateKey,
		Ed25519Public:  c.Ed25519PublicKey,
	}
}

func (c ExecutorConfig) Assignments() []WorkspaceAssignment {
	return c.WorkspaceAssignments
}

func (c ExecutorConfig) GetLastConnectedTime() time.Time {
	if c.LastConnected == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, c.LastConnected); err == nil {
		return parsed
	}
	return time.Time{}
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

type configManager struct {
	viper *viper.Viper
}

func NewConfigManager() (ConfigManager, error) {
	v := viper.New()

	setDefaults(v)

	v.AutomaticEnv()
	v.SetEnvPrefix("FLOWBAKER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	envMappings := map[string]string{
		"executor_id":                     "FLOWBAKER_EXECUTOR_ID",
		"executor_name":                   "FLOWBAKER_EXECUTOR_NAME",
		"address":                         "FLOWBAKER_EXECUTOR_ADDRESS",
		"api_base_url":                    "FLOWBAKER_API_URL",
		"x25519_private_key":              "FLOWBAKER_X25519_PRIVATE_KEY",
		"x25519_public_key":               "FLOWBAKER_X25519_PUBLIC_KEY",
		"ed25519_private_key":             "FLOWBAKER_ED25519_PRIVATE_KEY",
		"ed25519_public_key":              "FLOWBAKER_ED25519_PUBLIC_KEY",
		"static_api_signature_public_key": "STATIC_API_SIGNATURE_PUBLIC_KEY",
		"setup_complete":                  "FLOWBAKER_SETUP_COMPLETE",
		"enable_workspace_registration":   "FLOWBAKER_ENABLE_WORKSPACE_REGISTRATION",
		"enable_static_passcode":          "FLOWBAKER_ENABLE_STATIC_PASSCODE",
		"static_passcode":                 "FLOWBAKER_STATIC_PASSCODE",
		"skip_workspace_assignments":      "FLOWBAKER_SKIP_WORKSPACE_ASSIGNMENTS",
	}

	for configKey, envVar := range envMappings {
		if err := v.BindEnv(configKey, envVar); err != nil {
			log.Warn().Err(err).Msgf("Failed to bind environment variable %s for %s", envVar, configKey)
		}
	}

	v.SetConfigName("config")
	v.SetConfigType("json")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.flowbaker")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		log.Debug().Msg("Config file not found, using environment variables and defaults")
	} else {
		log.Debug().Msgf("Using config file: %s", v.ConfigFileUsed())
	}

	return &configManager{
		viper: v,
	}, nil
}

func (m *configManager) IsSetupComplete(ctx context.Context) bool {
	config, err := m.GetConfig(ctx)
	if err != nil {
		return false
	}

	if config.ExecutorID != "" &&
		config.ExecutorName != "" &&
		config.Address != "" &&
		config.APIBaseURL != "" &&
		config.X25519PrivateKey != "" &&
		config.Ed25519PrivateKey != "" &&
		config.StaticAPISignaturePublicKey != "" &&
		config.SetupComplete {
		return true
	}

	return config.SetupComplete && config.ExecutorID != ""
}

func (m *configManager) GetConfig(ctx context.Context) (ExecutorConfig, error) {
	var config ExecutorConfig
	if err := m.viper.Unmarshal(&config); err != nil {
		return ExecutorConfig{}, fmt.Errorf("unable to decode config: %w", err)
	}

	return config, nil
}

func (m *configManager) SaveConfig(ctx context.Context, config ExecutorConfig) error {
	m.viper.Set("executor_id", config.ExecutorID)
	m.viper.Set("executor_name", config.ExecutorName)
	m.viper.Set("address", config.Address)
	m.viper.Set("api_base_url", config.APIBaseURL)
	m.viper.Set("x25519_private_key", config.X25519PrivateKey)
	m.viper.Set("x25519_public_key", config.X25519PublicKey)
	m.viper.Set("ed25519_private_key", config.Ed25519PrivateKey)
	m.viper.Set("ed25519_public_key", config.Ed25519PublicKey)
	m.viper.Set("setup_complete", config.SetupComplete)
	m.viper.Set("workspace_assignments", config.WorkspaceAssignments)
	m.viper.Set("enable_workspace_registration", config.EnableWorkspaceRegistration)
	m.viper.Set("enable_static_passcode", config.EnableStaticPasscode)
	m.viper.Set("static_passcode", config.StaticPasscode)
	m.viper.Set("skip_workspace_assignments", config.SkipWorkspaceAssignments)
	m.viper.Set("last_connected", config.LastConnected)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".flowbaker")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")
	if err := m.viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (m *configManager) ResetConfig(ctx context.Context) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".flowbaker", "config.json")
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	for key := range m.viper.AllSettings() {
		m.viper.Set(key, nil)
	}

	setDefaults(m.viper)

	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("executor_name", "flowbaker-executor")
	v.SetDefault("address", "http://localhost:8081")
	v.SetDefault("api_base_url", "https://api.flowbaker.io")
	v.SetDefault("setup_complete", false)
	v.SetDefault("enable_workspace_registration", true)
	v.SetDefault("enable_static_passcode", false)
	v.SetDefault("skip_workspace_assignments", false)
}
