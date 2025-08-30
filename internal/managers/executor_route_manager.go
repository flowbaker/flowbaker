package managers

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/internal/domain"
)

type executorRouteManager struct {
	client flowbaker.ClientInterface
}

type ExecutorRouteManagerDependencies struct {
	Client flowbaker.ClientInterface
}

func NewExecutorRouteManager(deps ExecutorRouteManagerDependencies) domain.ExecutorRouteManager {
	return &executorRouteManager{
		client: deps.Client,
	}
}

func (m *executorRouteManager) GetRoutes(ctx context.Context, params domain.GetRoutesParams) ([]domain.WorkflowRoute, error) {
	response, err := m.client.GetRoutes(ctx, flowbaker.GetRoutesRequest{
		WorkspaceID:      params.WorkspaceID,
		RouteID:          params.RouteID,
		TriggerEventType: string(params.TriggerEventType),
		WorkflowType:     string(params.WorkflowType),
		IsWebhook:        params.IsWebhook,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get routes: %w", err)
	}

	routes := make([]domain.WorkflowRoute, 0, len(response.Routes))

	for i, route := range response.Routes {
		routes[i] = domain.WorkflowRoute{
			ID:                     route.ID,
			UserID:                 route.UserID,
			WorkflowID:             route.WorkflowID,
			RouteType:              route.RouteType,
			RouteID:                route.RouteID,
			TriggerID:              route.TriggerID,
			TriggerEventType:       domain.IntegrationTriggerEventType(route.TriggerEventType),
			TriggerIntegrationType: domain.IntegrationType(route.TriggerIntegrationType),
			WorkflowType:           domain.WorkflowType(route.WorkflowType),
			IsWebhook:              route.IsWebhook,
			Metadata:               route.Metadata,
			WorkspaceID:            route.WorkspaceID,
		}
	}

	return routes, nil
}
