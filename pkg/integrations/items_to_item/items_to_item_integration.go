package items_to_item

import (
	"context"
	"encoding/json"

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
		Add(IntegrationActionType_ItemsToItem, integration.ItemsToItem)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *ItemsToItemIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type ItemsToItemParams struct {
	FieldName string `json:"field_name"`
}

type itemWithFieldName struct {
	Item      domain.Item
	FieldName string
}

func (i *ItemsToItemIntegration) ItemsToItem(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	p := ItemsToItemParams{}

	items, err := params.GetAllItems()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	itemWithFieldNames := make([]itemWithFieldName, 0)

	for _, item := range items {
		err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		itemWithFieldName := itemWithFieldName{
			Item:      item,
			FieldName: p.FieldName,
		}

		itemWithFieldNames = append(itemWithFieldNames, itemWithFieldName)
	}

	outputItem := make(map[string][]any)

	for _, itemWithFieldName := range itemWithFieldNames {
		if itemWithFieldName.FieldName == "" {
			itemWithFieldName.FieldName = "items"
		}

		outputItem[itemWithFieldName.FieldName] = append(outputItem[itemWithFieldName.FieldName], itemWithFieldName.Item)
	}

	outputItems := []domain.Item{
		outputItem,
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}
