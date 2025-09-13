package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/flowbaker/flowbaker/internal/initialization"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func NewStatusCommand(executorContainer *initialization.ExecutorContainer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current executor status",
		Long:  `Display the current status of the executor including configuration details and workspace assignments.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(executorContainer)
		},
	}

	return cmd
}

func runStatus(executorContainer *initialization.ExecutorContainer) error {
	configManager := executorContainer.GetConfigManager()

	if configManager.IsSetupComplete(context.Background()) {
		config, err := configManager.GetConfig(context.Background())
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to load configuration")
			return err
		}

		workspaceNames := make([]string, len(config.Assignments))

		for i, assignment := range config.Assignments {
			workspaceNames[i] = assignment.WorkspaceName
		}

		fmt.Println("✅ Executor is set up and ready")
		fmt.Printf("   Executor ID: %s\n", config.ExecutorID)
		fmt.Printf("   Workspaces (%d): %s\n", len(config.Assignments), strings.Join(workspaceNames, ", "))
		fmt.Printf("   API URL: %s\n", config.APIBaseURL)
		if !config.LastConnected.IsZero() {
			fmt.Printf("   Last connected: %s\n", config.LastConnected.Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Println("❌ Executor is not set up")
		fmt.Printf("Run '%s start' to begin setup\n", os.Args[0])
	}

	return nil
}
