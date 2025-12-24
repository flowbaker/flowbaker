package memory

import (
	"context"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

type Store interface {
	SaveConversation(ctx context.Context, conversation *types.Conversation) error
	GetConversations(ctx context.Context, filter Filter) ([]*types.Conversation, error)
}

type Filter struct {
	SessionID string                   `json:"session_id,omitempty"`
	UserID    string                   `json:"user_id,omitempty"`
	Limit     int                      `json:"limit,omitempty"`
	Offset    int                      `json:"offset,omitempty"`
	Status    types.ConversationStatus `json:"status,omitempty"`
}

type NoOpMemoryStore struct {
}

func (s *NoOpMemoryStore) SaveConversation(ctx context.Context, conversation *types.Conversation) error {
	return nil
}

func (s *NoOpMemoryStore) GetConversations(ctx context.Context, filter Filter) ([]*types.Conversation, error) {
	return nil, nil
}

func (s *NoOpMemoryStore) GetConversation(ctx context.Context, id string) (*types.Conversation, error) {
	return nil, nil
}

func (s *NoOpMemoryStore) DeleteConversation(ctx context.Context, id string) error {
	return nil
}

func (s *NoOpMemoryStore) DeleteOldConversations(ctx context.Context, sessionID string, keepCount int) error {
	return nil
}
