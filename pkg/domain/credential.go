package domain

type CredentialType string

var (
	CredentialTypeDefault         CredentialType = "default"
	CredentialTypeOAuth           CredentialType = "oauth"
	CredentialTypeOAuthWithParams CredentialType = "oauth_with_params"
)

type Credential struct {
	ID              string
	Name            string
	WorkspaceID     string
	Type            CredentialType
	IntegrationType IntegrationType

	EncryptedPayload string         // Set if Type is Default or OAuthWithParams
	DecryptedPayload map[string]any // Set if Type is Default or OAuthWithParams
	OAuthAccountID   string         // Set if Type is OAuth or OAuthWithParams
	IsCustomOAuth    bool
}

type EncryptedExecutionCredential struct {
	ID                 string `json:"id"`
	WorkspaceID        string `json:"workspace_id"`
	EphemeralPublicKey []byte `json:"ephemeral_public_key"` // 32 bytes X25519 ephemeral public key
	EncryptedPayload   []byte `json:"encrypted_payload"`
	Nonce              []byte `json:"nonce"` // 12 bytes for ChaCha20-Poly1305
	ExpiresAt          int64  `json:"expires_at"`
	ExecutorID         string `json:"executor_id"`
}
