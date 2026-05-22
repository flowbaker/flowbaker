package managers

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

type executorAPIDefinitionManager struct {
	client flowbaker.ClientInterface
}

func NewExecutorAPIDefinitionManager(client flowbaker.ClientInterface) domain.ExecutorAPIDefinitionManager {
	return &executorAPIDefinitionManager{client: client}
}

func (l *executorAPIDefinitionManager) Load(ctx context.Context, apiDefinitionID string) (domain.ResolvedAPIDefinition, error) {
	wfCtx, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return domain.ResolvedAPIDefinition{}, fmt.Errorf("workflow execution context is required")
	}

	resp, err := l.client.GetResolvedAPIDefinition(ctx, wfCtx.WorkspaceID, apiDefinitionID)
	if err != nil {
		return domain.ResolvedAPIDefinition{}, err
	}

	endpoints := make([]domain.ResolvedAPIEndpoint, 0, len(resp.Endpoints))
	for _, e := range resp.Endpoints {
		endpoints = append(endpoints, domain.ResolvedAPIEndpoint{
			Path:    e.Path,
			Method:  e.Method,
			Summary: e.Summary,
		})
	}

	return domain.ResolvedAPIDefinition{
		ID:          resp.ID,
		WorkspaceID: resp.WorkspaceID,
		Name:        resp.Name,
		Type:        resp.Type,
		BaseURL:     resp.BaseURL,
		Endpoints:   endpoints,
		Auth: domain.ResolvedAPIAuth{
			Type:          resp.Auth.Type,
			KeyLocation:   resp.Auth.KeyLocation,
			KeyName:       resp.Auth.KeyName,
			HMACAlgorithm: resp.Auth.HMACAlgorithm,
			HMACHeader:    resp.Auth.HMACHeader,
			HMACPrefix:    resp.Auth.HMACPrefix,
			Secret:        resp.Auth.Secret,
		},
	}, nil
}
