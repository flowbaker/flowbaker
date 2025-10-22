package split_array

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/flowbaker/flowbaker/pkg/integrations/transform"
	"github.com/rs/zerolog/log"
)

type SplitArrayIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewSplitArrayIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &SplitArrayIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *SplitArrayIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewSplitArrayIntegration(SplitArrayIntegrationDependencies{
		ParameterBinder: c.binder,
	})
}

type SplitArrayIntegration struct {
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
	fieldParser   *transform.FieldPathParser
}

type SplitArrayIntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewSplitArrayIntegration(deps SplitArrayIntegrationDependencies) (*SplitArrayIntegration, error) {
	integration := &SplitArrayIntegration{
		binder:      deps.ParameterBinder,
		fieldParser: transform.NewFieldPathParser(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemMulti(IntegrationActionType_SplitArray, integration.SplitArray)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *SplitArrayIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type SplitArrayParams struct {
	FieldPath string `json:"field_path"`
}

type SplitArrayResultItem struct {
	Index int `json:"index"`
	Value any `json:"value"`
}

func (i *SplitArrayIntegration) SplitArray(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := SplitArrayParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.FieldPath == "" {
		return nil, fmt.Errorf("field_path cannot be empty")
	}

	log.Debug().Interface("field_path", p.FieldPath).Msg("field_path")

	// Get the value at the field path
	value, err := i.fieldParser.GetValue(item, p.FieldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get field '%s': %w", p.FieldPath, err)
	}

	log.Debug().Interface("value", value).Msg("value")

	// Type assert to array
	arr, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field '%s' is not an array, got %T", p.FieldPath, value)
	}

	// Check if array is empty
	if len(arr) == 0 {
		return nil, fmt.Errorf("field '%s' is an empty array", p.FieldPath)
	}

	// Convert each array element to an Item
	items := make([]domain.Item, len(arr))
	for i, element := range arr {
		items[i] = SplitArrayResultItem{
			Index: i,
			Value: element,
		}
	}

	return items, nil
}
