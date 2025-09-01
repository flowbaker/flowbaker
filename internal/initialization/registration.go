package initialization

import (
	"context"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
)

// RegisterExecutor registers the executor with the API and returns verification code
func RegisterExecutor(executorID string, keys CryptoKeys, apiBaseURL string) (string, error) {
	client := flowbaker.NewClient(
		flowbaker.WithBaseURL(apiBaseURL),
	)

	req := &flowbaker.CreateExecutorRegistrationRequest{
		ExecutorKey:      executorID,
		Address:          "localhost:8081", // Default address
		X25519PublicKey:  keys.X25519Public,
		Ed25519PublicKey: keys.Ed25519Public,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.CreateExecutorRegistration(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to register executor: %w", err)
	}

	return resp.VerificationCode, nil
}

// WaitForVerification waits for the executor to be verified via the frontend
func WaitForVerification(executorID, verificationCode string, keys CryptoKeys, apiBaseURL string) (string, string, error) {
	/* 	client := flowbaker.NewClient(
		flowbaker.WithBaseURL(apiBaseURL),
		flowbaker.WithExecutorID(executorID),
		flowbaker.WithEd25519PrivateKey(keys.Ed25519Private),
	) */

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", fmt.Errorf("verification timeout after 10 minutes")
		case <-ticker.C:
			// FIXME: Add polling logic here
		}
	}
}
