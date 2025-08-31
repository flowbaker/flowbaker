package domain

import (
	"context"
)

type ExecutorIntegrationManager interface {
	GetIntegrations(ctx context.Context) ([]Integration, error)
	GetIntegration(ctx context.Context, integrationType IntegrationType) (Integration, error)
}
