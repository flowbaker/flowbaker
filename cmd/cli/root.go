package cli

import (
	"fmt"
	"os"

	"github.com/flowbaker/flowbaker/internal/initialization"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "flowbaker",
		Short: "Flowbaker Executor CLI",
		Long: `Flowbaker Executor is a workflow automation engine that connects to the Flowbaker platform
to execute workflows and manage integrations.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().String("api-url", "", "Override API URL")

	executorContainer, err := initialization.NewExecutorContainer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize executor container: %v\n", err)
		os.Exit(1)
	}

	rootCmd.AddCommand(NewStartCommand(executorContainer))
	rootCmd.AddCommand(NewResetCommand(executorContainer))
	rootCmd.AddCommand(NewStatusCommand(executorContainer))
	rootCmd.AddCommand(NewWorkspacesCommand(executorContainer))

	return rootCmd
}

// Execute runs the root command
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
