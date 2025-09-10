package initialization

import "time"

// WorkspaceAPIKey represents an assignment-specific API public key
type WorkspaceAPIKey struct {
	WorkspaceID  string `json:"workspace_id"`
	APIPublicKey string `json:"api_public_key"`
}

// ExecutorConfig represents the persistent configuration for an executor
type ExecutorConfig struct {
	ExecutorID       string            `json:"executor_id,omitempty"`
	ExecutorName     string            `json:"executor_name"`
	Address          string            `json:"address"`
	WorkspaceIDs     []string          `json:"workspace_ids,omitempty"`
	WorkspaceAPIKeys []WorkspaceAPIKey `json:"workspace_api_keys,omitempty"` // Assignment-specific API public keys
	SetupComplete    bool              `json:"setup_complete"`
	Keys             CryptoKeys        `json:"keys"`
	APIBaseURL       string            `json:"api_url"`
	APIPublicKey     string            `json:"api_public_key,omitempty"` // Global API public key (backward compatibility)
	VerificationCode string            `json:"verification_code,omitempty"`
	LastConnected    time.Time         `json:"last_connected,omitempty"`
}

// CryptoKeys holds all cryptographic keys for the executor
type CryptoKeys struct {
	X25519Private  string `json:"x25519_private"`
	X25519Public   string `json:"x25519_public"`
	Ed25519Private string `json:"ed25519_private"`
	Ed25519Public  string `json:"ed25519_public"`
}

// SetupResult contains the results of the initialization process
type SetupResult struct {
	ExecutorID     string
	ExecutorName   string
	WorkspaceIDs   []string
	WorkspaceNames []string
}
