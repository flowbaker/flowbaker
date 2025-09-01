package initialization

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

func RunFirstTimeSetup() (*SetupResult, error) {
	fmt.Println("🚀 Welcome to Flowbaker")
	fmt.Println()
	fmt.Println("Setting up your executor...")

	executorID := GenerateExecutorID()
	log.Info().Str("executor_id", executorID).Msg("Generated executor ID")

	keys, err := GenerateAllKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	apiURL := getAPIURL()

	fmt.Println("📡 Registering with Flowbaker...")
	verificationCode, err := RegisterExecutor(executorID, keys, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to register executor: %w", err)
	}

	frontendURL := GetVerificationURL(apiURL)
	connectionURL := fmt.Sprintf("%s/executors/%s/verify?code=%s", frontendURL, executorID, verificationCode)
	fmt.Println()
	fmt.Println("🔗 Connect your executor:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for connection...")

	workspaceID, workspaceName, err := WaitForVerification(executorID, verificationCode, keys, apiURL)
	if err != nil {
		// Save partial config so user can resume later
		partialConfig := &ExecutorConfig{
			ExecutorID:       executorID,
			Keys:             keys,
			APIBaseURL:       apiURL,
			SetupComplete:    false,
			VerificationCode: verificationCode, // Save the code for resume
		}
		SaveConfig(partialConfig)
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	config := &ExecutorConfig{
		ExecutorID:    executorID,
		WorkspaceID:   workspaceID,
		Keys:          keys,
		APIBaseURL:    apiURL,
		SetupComplete: true,
		LastConnected: time.Now(),
	}

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Connected to \"%s\" workspace\n", workspaceName)
	fmt.Println("💾 Configuration saved")

	return &SetupResult{
		ExecutorID:    executorID,
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
	}, nil
}

func getAPIURL() string {
	if url := os.Getenv("FLOWBAKER_API_URL"); url != "" {
		return url
	}
	return GetDefaultAPIURL()
}

func ResumeSetup() (*SetupResult, error) {
	config, err := LoadConfig()
	if err != nil || config == nil {
		return nil, fmt.Errorf("no setup to resume")
	}

	if config.SetupComplete {
		return &SetupResult{
			ExecutorID:    config.ExecutorID,
			WorkspaceID:   config.WorkspaceID,
			WorkspaceName: "Unknown", // We don't store workspace name
		}, nil
	}

	fmt.Println("🔄 Resuming setup...")
	fmt.Println()

	frontendURL := GetVerificationURL(config.APIBaseURL)
	connectionURL := fmt.Sprintf("%s/executors/%s/verify?code=%s", frontendURL, config.ExecutorID, config.VerificationCode)
	fmt.Println("🔗 Connect your executor:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for connection...")

	workspaceID, workspaceName, err := WaitForVerification(config.ExecutorID, config.VerificationCode, config.Keys, config.APIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	config.WorkspaceID = workspaceID
	config.SetupComplete = true
	config.LastConnected = time.Now()

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Connected to \"%s\" workspace\n", workspaceName)

	return &SetupResult{
		ExecutorID:    config.ExecutorID,
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
	}, nil
}
