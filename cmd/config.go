package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Config holds all executor configuration
type Config struct {
	// Basic executor settings
	HTTPAddress string
	ExecutorID  string
	WorkspaceID string
	APIBaseURL  string

	// Cryptographic keys
	X25519PrivateKey    string
	Ed25519PrivateKey   string
	APISigningPublicKey string // Ed25519 public key for API request signature verification
}

// LoadConfig loads configuration from files and environment variables
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Configure environment variables - do this BEFORE reading config
	v.AutomaticEnv()
	v.SetEnvPrefix("") // No prefix for env vars
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set up explicit mappings between struct fields and environment variables
	envMappings := map[string]string{
		"HTTPAddress":         "HTTP_ADDRESS",
		"ExecutorID":          "EXECUTOR_ID",
		"WorkspaceID":         "WORKSPACE_ID",
		"APIBaseURL":          "API_BASE_URL",
		"X25519PrivateKey":    "EXECUTOR_X25519_PRIVATE_KEY",
		"Ed25519PrivateKey":   "EXECUTOR_ED25519_PRIVATE_KEY",
		"APISigningPublicKey": "API_SIGNING_PUBLIC_KEY",
	}

	for configKey, envVar := range envMappings {
		if err := v.BindEnv(configKey, envVar); err != nil {
			log.Warn().Err(err).Msgf("Failed to bind environment variable %s for %s", envVar, configKey)
		}
	}

	// Configure the config file settings
	v.SetConfigName("executor_config") // Name of config file without extension
	v.SetConfigType("yaml")            // Type of config file
	// Add search paths for the config file
	v.AddConfigPath(".")                // Current working directory
	v.AddConfigPath("./config")         // Config subdirectory
	v.AddConfigPath("$HOME/.flowbaker") // Home directory

	// Try to read from config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, will just use environment variables and defaults
		log.Debug().Msg("Config file not found, using environment variables and defaults")
	} else {
		log.Info().Msgf("Using config file: %s", v.ConfigFileUsed())
	}

	// Unmarshal config into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	// Validate required fields
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	log.Debug().Msgf("Config loaded: ExecutorID=%s, WorkspaceID=%s, APIBaseURL=%s",
		config.ExecutorID, config.WorkspaceID, config.APIBaseURL)

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server settings
	v.SetDefault("HTTPAddress", ":8081")
}

// validateConfig validates the required configuration fields
func validateConfig(config *Config) error {
	var missingVars []string

	if config.ExecutorID == "" {
		missingVars = append(missingVars, "EXECUTOR_ID")
	}

	if config.APIBaseURL == "" {
		missingVars = append(missingVars, "API_BASE_URL")
	}

	if config.X25519PrivateKey == "" {
		missingVars = append(missingVars, "EXECUTOR_X25519_PRIVATE_KEY")
	}

	if config.Ed25519PrivateKey == "" {
		missingVars = append(missingVars, "EXECUTOR_ED25519_PRIVATE_KEY")
	}

	if config.APISigningPublicKey == "" {
		missingVars = append(missingVars, "API_SIGNING_PUBLIC_KEY")
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %s\n\nGenerate keys with: %s generate-keys --executor-id %s",
			strings.Join(missingVars, ", "),
			os.Args[0],
			getEnvDefault("EXECUTOR_ID", "your-executor-id"))
	}

	return nil
}

// Helper function for getting environment variable with default
func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
