package managers

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

type executorWorkspaceManager struct {
	client *flowbaker.Client
}

type ExecutorWorkspaceManagerDependencies struct {
	FlowbakerClient *flowbaker.Client
}

func NewExecutorWorkspaceManager(deps ExecutorWorkspaceManagerDependencies) domain.ExecutorWorkspaceManager {
	return &executorWorkspaceManager{
		client: deps.FlowbakerClient,
	}
}

// GetWorkspace retrieves workspace information by ID
func (m *executorWorkspaceManager) GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	workspace, err := m.client.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("failed to get workspace from API: %w", err)
	}

	return domain.Workspace{
		ID:   workspace.ID,
		Name: workspace.Name,
		Slug: workspace.Slug,
	}, nil
}

// GetWorkspaces retrieves all workspaces assigned to this executor
func (m *executorWorkspaceManager) GetWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	workspaces, err := m.client.GetWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces from API: %w", err)
	}

	result := make([]domain.Workspace, len(workspaces))
	for i, workspace := range workspaces {
		result[i] = domain.Workspace{
			ID:   workspace.ID,
			Name: workspace.Name,
			Slug: workspace.Slug,
		}
	}

	return result, nil
}