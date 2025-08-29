package managers

import (
	"context"
	"encoding/json"
	"fmt"

	"flowbaker/internal/domain"
	"flowbaker/pkg/clients/flowbaker"
)

type executorIntegrationManager struct {
	client flowbaker.ClientInterface
}

type ExecutorIntegrationManagerDependencies struct {
	Client flowbaker.ClientInterface
}

func NewExecutorIntegrationManager(deps ExecutorIntegrationManagerDependencies) domain.ExecutorIntegrationManager {
	return &executorIntegrationManager{
		client: deps.Client,
	}
}

func (m *executorIntegrationManager) GetIntegrations(ctx context.Context) ([]domain.Integration, error) {
	responseJSON, err := m.client.GetIntegrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get integrations: %w", err)
	}

	var response struct {
		Integrations []domain.Integration `json:"integrations"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal integrations response: %w", err)
	}

	return response.Integrations, nil
}

func (m *executorIntegrationManager) GetIntegration(ctx context.Context, integrationType domain.IntegrationType) (domain.Integration, error) {
	responseJSON, err := m.client.GetIntegration(ctx, string(integrationType))
	if err != nil {
		return domain.Integration{}, fmt.Errorf("failed to get integration %s: %w", integrationType, err)
	}

	var domainIntegration domain.Integration

	if err := json.Unmarshal(responseJSON, &domainIntegration); err != nil {
		return domain.Integration{}, fmt.Errorf("failed to unmarshal integration response: %w", err)
	}

	return domainIntegration, nil
}
