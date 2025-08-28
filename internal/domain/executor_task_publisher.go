package domain

import (
	"context"
)

type ExecutorTaskPublisher interface {
	EnqueueTask(ctx context.Context, workspaceID string, task Task) error
	EnqueueTaskAndWait(ctx context.Context, workspaceID string, task Task) ([]byte, error)
}
