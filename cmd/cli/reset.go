package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/flowbaker/flowbaker/internal/initialization"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func NewResetCommand(executorContainer *initialization.ExecutorContainer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration and start fresh",
		Long:  `Reset the executor configuration and start fresh. This will remove all existing configuration and require you to set up the executor again.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReset(executorContainer)
		},
	}

	return cmd
}

func runReset(executorContainer *initialization.ExecutorContainer) error {
	configManager := executorContainer.GetConfigManager()

	if err := configManager.ResetConfig(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("Failed to reset configuration")
		return err
	}

	fmt.Println("✅ Configuration reset successfully")
	fmt.Printf("Run '%s start' to begin setup\n", os.Args[0])
	return nil
}
