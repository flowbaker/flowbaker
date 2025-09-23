package base64

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type Base64IntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewBase64IntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &Base64IntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *Base64IntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewBase64Integration(Base64IntegrationDependencies{
		ParameterBinder: c.binder,
	})
}

type Base64Integration struct {
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
}

type Base64IntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewBase64Integration(deps Base64IntegrationDependencies) (*Base64Integration, error) {
	integration := &Base64Integration{
		binder: deps.ParameterBinder,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_EncodeToBase64, integration.EncodeToBase64).
		AddPerItem(IntegrationActionType_DecodeFromBase64, integration.DecodeFromBase64)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *Base64Integration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type EncodeToBase64Params struct {
	Text string `json:"text"`
}

type DecodeFromBase64Params struct {
	EncodedText string `json:"encoded_text"`
}

func (i *Base64Integration) EncodeToBase64(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := EncodeToBase64Params{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	encodedText := base64.StdEncoding.EncodeToString([]byte(p.Text))

	result := map[string]interface{}{
		"encoded_text": encodedText,
	}

	return result, nil
}

func (i *Base64Integration) DecodeFromBase64(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DecodeFromBase64Params{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.EncodedText == "" {
		return nil, fmt.Errorf("encoded_text cannot be empty")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(p.EncodedText)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	result := map[string]interface{}{
		"decoded_text": string(decodedBytes),
	}

	return result, nil
}
