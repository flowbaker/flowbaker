package split_array

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/flowbaker/flowbaker/pkg/integrations/transform"
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

func (i *SplitArrayIntegration) SplitArray(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := SplitArrayParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.FieldPath == "" {
		arr, ok := item.([]any)
		if !ok {
			return nil, fmt.Errorf("item is not an array, got %T", item)
		}

		items := make([]domain.Item, 0, len(arr))

		for _, element := range arr {
			items = append(items, element)
		}

		return items, nil
	}

	value, err := i.fieldParser.GetValue(item, p.FieldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get field '%s': %w", p.FieldPath, err)
	}

	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("field '%s' is not an array, got %T", p.FieldPath, value)
	}

	if len(arr) == 0 {
		return nil, fmt.Errorf("field '%s' is an empty array", p.FieldPath)
	}

	items := make([]domain.Item, 0, len(arr))

	for _, element := range arr {
		items = append(items, element)
	}

	return items, nil
}
