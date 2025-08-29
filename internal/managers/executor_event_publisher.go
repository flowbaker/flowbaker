package managers

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/pkg/clients/flowbaker"
	"fmt"
)

type ExecutorEventPublisher struct {
	api *flowbaker.Client
}

func NewExecutorEventPublisher(api *flowbaker.Client) *ExecutorEventPublisher {
	return &ExecutorEventPublisher{
		api: api,
	}
}

func (p *ExecutorEventPublisher) PublishEvent(ctx context.Context, event domain.Event) error {
	workflowExecutionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return fmt.Errorf("workflow execution context is required")
	}

	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = p.api.PublishExecutionEvent(ctx, workflowExecutionContext.WorkspaceID, &flowbaker.PublishEventRequest{
		EventType: flowbaker.EventType(event.GetType()),
		EventData: payloadJSON,
	})
	if err != nil {
		return err
	}

	return nil
}
