package domain

import "context"

// TaskHandler can handle both async tasks (returns error only) and sync tasks (returns data and error)
type TaskHandler func(context.Context, Task) ([]byte, error)

type TaskQueueListener interface {
	Listen(ctx context.Context, taskType TaskType, handler TaskHandler) error
}

type TaskPublisher interface {
	EnqueueTask(ctx context.Context, task Task) error
	EnqueueTaskAndWait(ctx context.Context, task Task) ([]byte, error)
}

type Task interface {
	GetType() TaskType
}

type TaskType string

var (
	ExecuteWorkflow  TaskType = "execute_workflow"
	ProcessEmbedding TaskType = "process_embedding"
)

type ExecuteWorkflowTask struct {
	WorkspaceID  string       `json:"workspace_id"`
	WorkflowID   string       `json:"workflow_id"`
	UserID       string       `json:"user_id"`
	WorkflowType WorkflowType `json:"workflow_type"`
	FromNodeID   string       `json:"from_node_id"`
	Payload      any          `json:"payload"`
}

func (t ExecuteWorkflowTask) GetType() TaskType {
	return ExecuteWorkflow
}

type ProcessEmbeddingTask struct {
	KnowledgeFileID string `json:"knowledge_file_id"`
	WorkspaceID     string `json:"workspace_id"`
}

func (t ProcessEmbeddingTask) GetType() TaskType {
	return ProcessEmbedding
}
