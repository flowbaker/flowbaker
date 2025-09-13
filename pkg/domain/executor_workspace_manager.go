package domain

import "context"

type Workspace struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type ExecutorWorkspaceManager interface {
	GetWorkspace(ctx context.Context, workspaceID string) (Workspace, error)
	GetWorkspaces(ctx context.Context) ([]Workspace, error)
}
