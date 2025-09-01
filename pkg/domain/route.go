package domain

import "context"

type WorkflowRoute struct {
	ID                     string
	UserID                 string
	WorkflowID             string
	RouteType              string
	RouteID                string
	TriggerID              string
	TriggerEventType       IntegrationTriggerEventType
	TriggerIntegrationType IntegrationType
	Metadata               map[string]any
	WorkflowType           WorkflowType
	IsWebhook              bool
	WorkspaceID            string
}

type ExecutorRouteManager interface {
	GetRoutes(ctx context.Context, params GetRoutesParams) ([]WorkflowRoute, error)
}

type GetRoutesParams struct {
	WorkspaceID      string
	RouteID          string
	TriggerEventType IntegrationTriggerEventType

	WorkflowType WorkflowType
	IsWebhook    bool
}
