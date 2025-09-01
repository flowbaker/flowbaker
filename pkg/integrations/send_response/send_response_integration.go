package sendresponse

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type SendResponseIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewSendResponseIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &SendResponseIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *SendResponseIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	i := NewSendResponseIntegration(SendResponseIntegrationDependencies{
		Binder: c.binder,
	})

	return i, nil
}

type SendResponseIntegration struct {
	binder domain.IntegrationParameterBinder
}

type SendResponseIntegrationDependencies struct {
	Binder domain.IntegrationParameterBinder
}

func NewSendResponseIntegration(deps SendResponseIntegrationDependencies) *SendResponseIntegration {
	return &SendResponseIntegration{
		binder: deps.Binder,
	}
}

type SendResponseParams struct {
	ResponseType string `json:"response_type"`

	// Optional fields
	Text string `json:"text"`
	JSON string `json:"json"`
	HTML string `json:"html"`
}

func (i *SendResponseIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	workflowExecutionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return domain.IntegrationOutput{}, errors.New("workflow execution context not found")
	}

	allItems, err := params.GetAllItems()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	outputItems := []domain.Item{}

	if len(allItems) > 0 {
		sendResponseParams := SendResponseParams{}

		item := allItems[0]

		err := i.binder.BindToStruct(ctx, item, &sendResponseParams, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		handleResponseParams := HandleResponseParams{
			Item:               item,
			IntegrationParams:  params.IntegrationParams,
			SendResponseParams: sendResponseParams,
		}

		var handleResponseOutput HandleResponseOutput

		switch sendResponseParams.ResponseType {
		case "text":
			handleResponseOutput, err = i.handleText(handleResponseParams)
			if err != nil {
				return domain.IntegrationOutput{}, err
			}
		case "json":
			handleResponseOutput, err = i.handleJSON(handleResponseParams)
			if err != nil {
				return domain.IntegrationOutput{}, err
			}
		case "html":
			handleResponseOutput, err = i.handleHTML(handleResponseParams)
			if err != nil {
				return domain.IntegrationOutput{}, err
			}
		case "empty":
			handleResponseOutput = HandleResponseOutput{
				ResponsePayload: []byte{},
				ResponseHeaders: map[string][]string{},
				OutputItem:      map[string]any{},
			}
		default:
			allItemsJSON, err := json.Marshal(allItems)
			if err != nil {
				return domain.IntegrationOutput{}, err
			}

			handleResponseOutput = HandleResponseOutput{
				ResponsePayload: allItemsJSON,
				ResponseHeaders: map[string][]string{
					"Content-Type": {"application/json"},
				},
				OutputItem: map[string]any{
					"body": allItemsJSON,
					"headers": map[string][]string{
						"Content-Type": {"application/json"},
					},
				},
			}
		}

		outputItems = append(outputItems, handleResponseOutput.OutputItem)

		workflowExecutionContext.SetResponsePayload(handleResponseOutput.ResponsePayload)
		workflowExecutionContext.SetResponseHeaders(handleResponseOutput.ResponseHeaders)
		workflowExecutionContext.SetResponseStatusCode(200)
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

type HandleResponseParams struct {
	Item               domain.Item              `json:"item"`
	IntegrationParams  domain.IntegrationParams `json:"integration_params"`
	SendResponseParams SendResponseParams       `json:"send_response_params"`
}

type HandleResponseOutput struct {
	ResponseHeaders    map[string][]string `json:"response_headers"`
	ResponsePayload    []byte              `json:"response_payload"`
	ResponseStatusCode int                 `json:"response_status_code"`
	OutputItem         domain.Item         `json:"output_item"`
}

type TextResponseParams struct {
	Text string `json:"text"`
}

func (i *SendResponseIntegration) handleText(p HandleResponseParams) (HandleResponseOutput, error) {
	responseHeaders := map[string][]string{
		"Content-Type": {"text/plain"},
	}

	outputItem := map[string]any{
		"body":    p.SendResponseParams.Text,
		"headers": responseHeaders,
		"status":  200,
	}

	return HandleResponseOutput{
		OutputItem:         outputItem,
		ResponsePayload:    []byte(p.SendResponseParams.Text),
		ResponseHeaders:    responseHeaders,
		ResponseStatusCode: 200,
	}, nil
}

func (i *SendResponseIntegration) handleJSON(p HandleResponseParams) (HandleResponseOutput, error) {
	responseHeaders := map[string][]string{
		"Content-Type": {"application/json"},
	}

	outputItem := map[string]any{
		"body":    p.SendResponseParams.JSON,
		"headers": responseHeaders,
		"status":  200,
	}

	return HandleResponseOutput{
		OutputItem:         outputItem,
		ResponsePayload:    []byte(p.SendResponseParams.JSON),
		ResponseHeaders:    responseHeaders,
		ResponseStatusCode: 200,
	}, nil
}

func (i *SendResponseIntegration) handleHTML(p HandleResponseParams) (HandleResponseOutput, error) {
	responseHeaders := map[string][]string{
		"Content-Type": {"text/html"},
	}

	outputItem := map[string]any{
		"body":    p.SendResponseParams.HTML,
		"headers": responseHeaders,
		"status":  200,
	}

	return HandleResponseOutput{
		OutputItem:         outputItem,
		ResponsePayload:    []byte(p.SendResponseParams.HTML),
		ResponseHeaders:    responseHeaders,
		ResponseStatusCode: 200,
	}, nil
}
