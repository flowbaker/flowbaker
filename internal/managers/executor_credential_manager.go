package managers

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/pkg/clients/flowbaker"
	"fmt"
)

type executorCredentialManager struct {
	client        flowbaker.ClientInterface
	decryptionSvc domain.ExecutorCredentialDecryptionService
}

func NewExecutorCredentialManager(client flowbaker.ClientInterface, decryptionSvc domain.ExecutorCredentialDecryptionService) domain.ExecutorCredentialManager {
	return &executorCredentialManager{
		client:        client,
		decryptionSvc: decryptionSvc,
	}
}

func (e *executorCredentialManager) GetDecryptedCredential(ctx context.Context, credentialID string) ([]byte, error) {
	workflowExecutionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return nil, fmt.Errorf("workflow execution context is required")
	}

	encryptedCred, err := e.client.GetCredential(ctx, workflowExecutionContext.WorkspaceID, credentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encrypted credential: %w", err)
	}

	credential := domain.EncryptedExecutionCredential{
		ID:                 encryptedCred.ID,
		WorkspaceID:        encryptedCred.WorkspaceID,
		EphemeralPublicKey: encryptedCred.EphemeralPublicKey,
		EncryptedPayload:   encryptedCred.EncryptedPayload,
		Nonce:              encryptedCred.Nonce,
		ExpiresAt:          encryptedCred.ExpiresAt,
		ExecutorID:         encryptedCred.ExecutorID,
	}

	decryptedBytes, err := e.decryptionSvc.DecryptCredential(credential)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credential: %w", err)
	}

	return decryptedBytes, nil
}

func (e *executorCredentialManager) GetOAuthAccount(ctx context.Context, oauthAccountID string) (domain.OAuthAccount, error) {
	workflowExecutionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return domain.OAuthAccount{}, fmt.Errorf("workflow execution context is required")
	}

	response, err := e.client.GetOAuthAccount(ctx, workflowExecutionContext.WorkspaceID, oauthAccountID)
	if err != nil {
		return domain.OAuthAccount{}, fmt.Errorf("failed to get OAuth account: %w", err)
	}

	// Convert flowbaker.OAuthAccount to domain.OAuthAccount
	domainAccount := domain.OAuthAccount{
		ID:        response.OAuthAccount.ID,
		UserID:    response.OAuthAccount.UserID,
		OAuthName: response.OAuthAccount.OAuthName,
		OAuthType: domain.OAuthType(response.OAuthAccount.OAuthType),
		Metadata:  response.OAuthAccount.Metadata,
		// Note: EncryptedSensitiveData is intentionally not included as it's not available from executor endpoint
	}

	return domainAccount, nil
}

func (e *executorCredentialManager) GetFullCredential(ctx context.Context, credentialID string) (domain.Credential, error) {
	workflowExecutionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return domain.Credential{}, fmt.Errorf("workflow execution context is required")
	}

	encryptedCred, err := e.client.GetFullCredential(ctx, workflowExecutionContext.WorkspaceID, credentialID)
	if err != nil {
		return domain.Credential{}, fmt.Errorf("failed to get encrypted full credential: %w", err)
	}

	// Use the same decryption service but handle EncryptedFullCredential format
	credential := domain.EncryptedExecutionCredential{
		ID:                 encryptedCred.ID,
		WorkspaceID:        encryptedCred.WorkspaceID,
		EphemeralPublicKey: encryptedCred.EphemeralPublicKey,
		EncryptedPayload:   encryptedCred.EncryptedPayload,
		Nonce:              encryptedCred.Nonce,
		ExpiresAt:          encryptedCred.ExpiresAt,
		ExecutorID:         encryptedCred.ExecutorID,
	}

	decryptedBytes, err := e.decryptionSvc.DecryptCredential(credential)
	if err != nil {
		return domain.Credential{}, fmt.Errorf("failed to decrypt full credential: %w", err)
	}

	// Unmarshal the decrypted bytes into full Credential struct
	var fullCredential domain.Credential
	if err := json.Unmarshal(decryptedBytes, &fullCredential); err != nil {
		return domain.Credential{}, fmt.Errorf("failed to unmarshal full credential: %w", err)
	}

	return fullCredential, nil
}

func (e *executorCredentialManager) UpdateOAuthAccountMetadata(ctx context.Context, oauthAccountID string, metadata map[string]interface{}) error {
	workflowExecutionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return fmt.Errorf("workflow execution context is required")
	}

	req := &flowbaker.UpdateOAuthAccountMetadataRequest{
		Metadata: metadata,
	}

	_, err := e.client.UpdateOAuthAccountMetadata(ctx, workflowExecutionContext.WorkspaceID, oauthAccountID, req)
	if err != nil {
		return fmt.Errorf("failed to update OAuth account metadata: %w", err)
	}

	return nil
}

// ExecutorCredentialGetter retrieves and decrypts credentials from the API for executors
type ExecutorCredentialGetter[T any] struct {
	manager domain.ExecutorCredentialManager
}

func NewExecutorCredentialGetter[T any](
	manager domain.ExecutorCredentialManager,
) *ExecutorCredentialGetter[T] {
	return &ExecutorCredentialGetter[T]{
		manager: manager,
	}
}

func (e *ExecutorCredentialGetter[T]) GetDecryptedCredential(ctx context.Context, credentialID string) (T, error) {
	var zero T

	decryptedBytes, err := e.manager.GetDecryptedCredential(ctx, credentialID)
	if err != nil {
		return zero, fmt.Errorf("failed to get encrypted credential: %w", err)
	}

	var result T
	if err := json.Unmarshal(decryptedBytes, &result); err != nil {
		return zero, fmt.Errorf("failed to unmarshal credential: %w", err)
	}

	return result, nil
}
