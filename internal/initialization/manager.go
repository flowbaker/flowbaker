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

	executorName := GenerateExecutorName()
	log.Info().Str("executor_name", executorName).Msg("Generated executor name")

	keys, err := GenerateAllKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	apiURL := getAPIURL()

	fmt.Println("📡 Registering with Flowbaker...")
	verificationCode, err := RegisterExecutor(executorName, keys, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to register executor: %w", err)
	}

	frontendURL := GetVerificationURL(apiURL)
	connectionURL := fmt.Sprintf("%s/executors/verify?code=%s", frontendURL, verificationCode)
	fmt.Println()
	fmt.Println("🔗 Connect your executor:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for connection...")

	executorID, workspaceID, workspaceName, err := WaitForVerification(executorName, verificationCode, keys, apiURL)
	if err != nil {
		partialConfig := &ExecutorConfig{
			ExecutorName:     executorName,
			Keys:             keys,
			APIBaseURL:       apiURL,
			SetupComplete:    false,
			VerificationCode: verificationCode,
		}
		SaveConfig(partialConfig)
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	config := &ExecutorConfig{
		ExecutorID:    executorID,
		ExecutorName:  executorName,
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
	fmt.Printf("✅ Connected to workspace \"%s\"\n", workspaceName)
	fmt.Println("💾 Configuration saved")

	return &SetupResult{
		ExecutorID:    executorID,
		ExecutorName:  executorName,
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
			ExecutorName:  config.ExecutorName,
			WorkspaceID:   config.WorkspaceID,
			WorkspaceName: "Unknown", // We don't store workspace name in config, would need API call to fetch
		}, nil
	}

	fmt.Println("🔄 Resuming setup...")
	fmt.Println()

	frontendURL := GetVerificationURL(config.APIBaseURL)
	connectionURL := fmt.Sprintf("%s/executors/verify?code=%s", frontendURL, config.VerificationCode)
	fmt.Println("🔗 Connect your executor:")
	fmt.Println()
	fmt.Printf("   %s\n", connectionURL)
	fmt.Println()
	fmt.Println("⏳ Waiting for connection...")

	executorID, workspaceID, workspaceName, err := WaitForVerification(config.ExecutorName, config.VerificationCode, config.Keys, config.APIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	config.ExecutorID = executorID
	config.WorkspaceID = workspaceID
	config.SetupComplete = true
	config.LastConnected = time.Now()

	if err := SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Connected to workspace \"%s\"\n", workspaceName)

	return &SetupResult{
		ExecutorID:    executorID,
		ExecutorName:  config.ExecutorName,
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
	}, nil
}
