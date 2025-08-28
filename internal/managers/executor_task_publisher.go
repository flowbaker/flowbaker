package managers

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/pkg/flowbaker"
	"fmt"
)

type executorTaskPublisher struct {
	client *flowbaker.Client
}

type ExecutorTaskPublisherDependencies struct {
	Client *flowbaker.Client
}

func NewExecutorTaskPublisher(deps ExecutorTaskPublisherDependencies) domain.ExecutorTaskPublisher {
	return &executorTaskPublisher{
		client: deps.Client,
	}
}

func (p *executorTaskPublisher) EnqueueTask(ctx context.Context, workspaceID string, task domain.Task) error {
	if workspaceID == "" {
		return fmt.Errorf("workspaceID cannot be empty")
	}

	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	req := &flowbaker.EnqueueTaskRequest{
		TaskType: string(task.GetType()),
		TaskData: task,
	}

	resp, err := p.client.EnqueueTask(ctx, workspaceID, req)
	if err != nil {
		return fmt.Errorf("task enqueue failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("task enqueue failed: API returned failure")
	}

	return nil
}

func (p *executorTaskPublisher) EnqueueTaskAndWait(ctx context.Context, workspaceID string, task domain.Task) ([]byte, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	req := &flowbaker.EnqueueTaskAndWaitRequest{
		TaskType: string(task.GetType()),
		TaskData: task,
	}

	resp, err := p.client.EnqueueTaskAndWait(ctx, workspaceID, req)
	if err != nil {
		return nil, fmt.Errorf("task enqueue and wait failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("task enqueue and wait failed: API returned failure")
	}

	// Convert result to bytes if it's not already
	var resultBytes []byte
	if resp.Result != nil {
		var err error

		resultBytes, err = json.Marshal(resp.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal task result: %w", err)
		}
	}

	return resultBytes, nil
}
