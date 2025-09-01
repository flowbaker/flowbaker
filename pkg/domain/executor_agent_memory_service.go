package domain

import (
	"context"
	"time"
)

type AgentMemoryService interface {
	SaveConversation(ctx context.Context, conversation *AgentConversation) error
	GetConversations(ctx context.Context, params GetConversationsParams) ([]*AgentConversation, error)
	DeleteOldConversations(ctx context.Context, workspaceID string, sessionID string, keepCount int) error
}

type GetConversationsParams struct {
	WorkspaceID string
	SessionID   string
	Limit       int
	Offset      int
	StartTime   *time.Time
	EndTime     *time.Time
	Status      string
}
