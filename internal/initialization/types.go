package initialization

import "time"

// ExecutorConfig represents the persistent configuration for an executor
type ExecutorConfig struct {
	ExecutorID       string    `json:"executor_id"`
	WorkspaceID      string    `json:"workspace_id,omitempty"`
	SetupComplete    bool      `json:"setup_complete"`
	Keys             CryptoKeys `json:"keys"`
	APIBaseURL       string    `json:"api_url"`
	VerificationCode string    `json:"verification_code,omitempty"`
	LastConnected    time.Time `json:"last_connected,omitempty"`
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
	ExecutorID    string
	WorkspaceID   string
	WorkspaceName string
}