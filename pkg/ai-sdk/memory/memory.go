package memory

import (
	"context"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

type Store interface {
	SaveConversation(ctx context.Context, conversation *types.Conversation) error
	GetConversations(ctx context.Context, filter Filter) ([]*types.Conversation, error)
	GetConversation(ctx context.Context, id string) (*types.Conversation, error)
	DeleteConversation(ctx context.Context, id string) error
	DeleteOldConversations(ctx context.Context, sessionID string, keepCount int) error
}

type Filter struct {
	SessionID string
	UserID    string
	Limit     int
	Offset    int
	Status    types.ConversationStatus
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
