package items_to_item

import (
	"context"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ItemsToItemIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewItemsToItemIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &ItemsToItemIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *ItemsToItemIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewItemsToItemIntegration(ItemsToItemIntegrationDependencies{
		ParameterBinder: c.binder,
	})
}

type ItemsToItemIntegration struct {
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
}

type ItemsToItemIntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewItemsToItemIntegration(deps ItemsToItemIntegrationDependencies) (*ItemsToItemIntegration, error) {
	integration := &ItemsToItemIntegration{
		binder: deps.ParameterBinder,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddItemsToItem(IntegrationActionType_ItemsToItem, integration.ItemsToItem)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *ItemsToItemIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type ItemsToItemResponse struct {
	Items []domain.Item `json:"items"`
	Count int           `json:"count"`
}

func (i *ItemsToItemIntegration) ItemsToItem(ctx context.Context, params domain.IntegrationInput, items []domain.Item) (domain.Item, error) {
	return ItemsToItemResponse{
		Items: items,
		Count: len(items),
	}, nil
}
