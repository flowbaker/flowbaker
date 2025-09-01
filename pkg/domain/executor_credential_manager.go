package domain

import (
	"context"
)

type CredentialGetter[T any] interface {
	GetDecryptedCredential(ctx context.Context, credentialID string) (T, error)
}

type ExecutorCredentialManager interface {
	GetDecryptedCredential(ctx context.Context, credentialID string) ([]byte, error)
	GetFullCredential(ctx context.Context, credentialID string) (Credential, error)
	GetOAuthAccount(ctx context.Context, oauthAccountID string) (OAuthAccount, error)
	UpdateOAuthAccountMetadata(ctx context.Context, oauthAccountID string, metadata map[string]interface{}) error
}
