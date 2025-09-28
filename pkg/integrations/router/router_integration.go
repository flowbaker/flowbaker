package router

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/gosimple/slug"
	"github.com/rs/zerolog/log"
)

type RouterIntegrationCreator struct {
	binder               domain.IntegrationParameterBinder
	executorModelManager domain.ExecutorModelManager
}

func NewRouterIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &RouterIntegrationCreator{
		binder:               deps.ParameterBinder,
		executorModelManager: deps.ExecutorModelManager,
	}
}

func (c *RouterIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewRouterIntegration(ctx, RouterIntegrationDependencies{
		WorkspaceID:          p.WorkspaceID,
		ParameterBinder:      c.binder,
		ExecutorModelManager: c.executorModelManager,
	})
}

type RouterIntegration struct {
	workspaceID string

	binder               domain.IntegrationParameterBinder
	executorModelManager domain.ExecutorModelManager
	actionManager        *domain.IntegrationActionManager
}

type RouterIntegrationDependencies struct {
	WorkspaceID          string
	ParameterBinder      domain.IntegrationParameterBinder
	ExecutorModelManager domain.ExecutorModelManager
}

func NewRouterIntegration(ctx context.Context, deps RouterIntegrationDependencies) (*RouterIntegration, error) {
	integration := &RouterIntegration{
		workspaceID:          deps.WorkspaceID,
		binder:               deps.ParameterBinder,
		executorModelManager: deps.ExecutorModelManager,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemRoutable(IntegrationActionType_Route, integration.Route)

	integration.actionManager = actionManager

	return integration, nil
}

func (r *RouterIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return r.actionManager.Run(ctx, params.ActionType, params)
}

type RouteParams struct {
	Content         string           `json:"content"`
	Classifications []Classification `json:"classifications"`
}

type Classification struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r *RouterIntegration) Route(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.RoutableOutput, error) {
	p := RouteParams{}

	err := r.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.RoutableOutput{}, err
	}

	log.Debug().Interface("p", p).Msg("Route params")

	convertedClassifications := make([]domain.Classification, len(p.Classifications))

	slugsToIndex := make(map[string]int)

	for i, classification := range p.Classifications {
		keySlug := slug.Make(classification.Name)

		slugsToIndex[keySlug] = i

		convertedClassifications[i] = domain.Classification{
			Key:         keySlug,
			Description: classification.Description,
		}
	}

	classification, err := r.executorModelManager.ClassifyContent(ctx, domain.ClassifyContentParams{
		WorkspaceID:     r.workspaceID,
		Content:         p.Content,
		Classifications: convertedClassifications,
	})
	if err != nil {
		return domain.RoutableOutput{}, err
	}

	selectedClassificationIndex, ok := slugsToIndex[classification.SelectedClassification]
	if !ok {
		return domain.RoutableOutput{}, fmt.Errorf("no selected classification")
	}

	if selectedClassificationIndex < 0 || selectedClassificationIndex >= len(convertedClassifications) {
		return domain.RoutableOutput{}, fmt.Errorf("selected classification index out of bounds")
	}

	return domain.RoutableOutput{
		Item:        item,
		OutputIndex: selectedClassificationIndex,
	}, nil
}
