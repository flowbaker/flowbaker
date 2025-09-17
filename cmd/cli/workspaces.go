package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/flowbaker/flowbaker/internal/initialization"
	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func NewWorkspacesCommand(executorContainer *initialization.ExecutorContainer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspaces",
		Short: "Manage workspace assignments",
		Long:  `Manage workspace assignments for the executor. View, add, or remove workspace assignments.`,
	}

	cmd.AddCommand(NewWorkspacesListCommand(executorContainer))

	return cmd
}

func NewWorkspacesListCommand(executorContainer *initialization.ExecutorContainer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all assigned workspaces",
		Long:  `List all workspaces that this executor is assigned to, showing workspace names and details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkspacesList(executorContainer)
		},
	}

	return cmd
}

func runWorkspacesList(executorContainer *initialization.ExecutorContainer) error {
	configManager := executorContainer.GetConfigManager()

	if !configManager.IsSetupComplete(context.Background()) {
		fmt.Println("❌ Executor is not set up. Run 'start' to begin setup.")
		os.Exit(1)
	}

	config, err := configManager.GetConfig(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
		return err
	}

	fmt.Println("📋 Assigned Workspaces:")
	if len(config.WorkspaceAssignments) == 0 {
		fmt.Println("   No workspaces assigned")
		return nil
	}

	flowbakerClient := flowbaker.NewClient(
		flowbaker.WithBaseURL(config.APIBaseURL),
		flowbaker.WithExecutorID(config.ExecutorID),
		flowbaker.WithEd25519PrivateKey(config.Ed25519PrivateKey),
	)

	workspaceManager := managers.NewExecutorWorkspaceManager(managers.ExecutorWorkspaceManagerDependencies{
		FlowbakerClient: flowbakerClient,
	})

	ctx := context.Background()
	workspaces, err := workspaceManager.GetWorkspaces(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch workspace details")
		return err
	}

	for i, workspace := range workspaces {
		fmt.Printf("   %d. %s (%s)\n", i+1, workspace.Name, workspace.Slug)
	}
	fmt.Printf("\nTotal: %d workspace(s)\n", len(workspaces))

	return nil
}
