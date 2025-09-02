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
		ExecutorName:     executorID,
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
func WaitForVerification(executorName, verificationCode string, keys CryptoKeys, apiBaseURL string) (string, string, string, error) {
	client := flowbaker.NewClient(
		flowbaker.WithBaseURL(apiBaseURL),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Printf("Waiting for executor verification (code: %s)...\n", verificationCode)

	for {
		select {
		case <-ctx.Done():
			return "", "", "", fmt.Errorf("verification timeout after 10 minutes")
		case <-ticker.C:
			status, err := client.GetExecutorRegistrationStatus(ctx, verificationCode)
			if err != nil {
				// Log error but continue polling
				fmt.Printf("Error checking registration status: %v\n", err)
				continue
			}

			switch status.Status {
			case "verified":
				fmt.Println("Executor registration verified!")
				if status.Executor != nil {
					return status.Executor.ID, status.Executor.WorkspaceID, status.WorkspaceName, nil
				}
				// Fallback if executor data is not available
				return "", "", "", fmt.Errorf("executor data not available in verification response")
			case "not_found":
				return "", "", "", fmt.Errorf("registration not found or expired: %s", status.Message)
			case "pending":
				// Continue polling
				fmt.Printf("Registration still pending, expires at: %s\n", status.ExpiresAt)
				continue
			default:
				fmt.Printf("Unknown status: %s\n", status.Status)
				continue
			}
		}
	}
}
