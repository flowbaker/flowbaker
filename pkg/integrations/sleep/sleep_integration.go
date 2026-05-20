package sleep

import (
	"context"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	MinSleep = 1 * time.Second
	MaxSleep = 30 * 24 * time.Hour
)

type SleepIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewSleepIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &SleepIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *SleepIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewSleepIntegration(SleepIntegrationDependencies{
		ParameterBinder: c.binder,
	})
}

type SleepIntegration struct {
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
}

type SleepIntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewSleepIntegration(deps SleepIntegrationDependencies) (*SleepIntegration, error) {
	integration := &SleepIntegration{
		binder: deps.ParameterBinder,
	}

	integration.actionManager = domain.NewIntegrationActionManager().
		Add(SleepActionType_Sleep, integration.Sleep)

	return integration, nil
}

func (i *SleepIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type SleepParams struct {
	DurationValue int    `json:"duration_value"`
	DurationUnit  string `json:"duration_unit"`
}

func (i *SleepIntegration) Sleep(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	p := SleepParams{}

	bindItem := domain.Item(map[string]any{})
	allItems := params.GetAllItems()
	if len(allItems) > 0 {
		bindItem = allItems[0]
	}

	if err := i.binder.BindToStruct(ctx, bindItem, &p, params.IntegrationParams.Settings); err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to bind sleep parameters: %w", err)
	}

	duration, err := parseDuration(p.DurationValue, p.DurationUnit)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	if duration < MinSleep {
		return domain.IntegrationOutput{}, fmt.Errorf("sleep duration must be at least %s", MinSleep)
	}
	if duration > MaxSleep {
		return domain.IntegrationOutput{}, fmt.Errorf("sleep duration must not exceed %s", MaxSleep)
	}

	execCtx, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return domain.IntegrationOutput{}, fmt.Errorf("sleep: workflow execution context not found")
	}
	execCtx.EmitSignal(domain.PauseSignal{
		WakeAt: time.Now().Add(duration),
	})

	return domain.IntegrationOutput{
		ItemsByOutputIndex: params.ItemsByInputIndex,
	}, nil
}

func parseDuration(value int, unit string) (time.Duration, error) {
	if value <= 0 {
		return 0, fmt.Errorf("duration value must be positive")
	}

	switch unit {
	case DurationUnitSeconds:
		return time.Duration(value) * time.Second, nil
	case DurationUnitMinutes:
		return time.Duration(value) * time.Minute, nil
	case DurationUnitHours:
		return time.Duration(value) * time.Hour, nil
	case DurationUnitDays:
		return time.Duration(value) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}
}
