package memory

import (
	"context"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

type Store interface {
	GetConversation(ctx context.Context, filter Filter) (types.Conversation, error)
	SaveConversation(ctx context.Context, conversation types.Conversation) error
}

type Filter struct {
	SessionID string `json:"session_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

type NoOpMemoryStore struct {
}

func (s *NoOpMemoryStore) SaveConversation(ctx context.Context, conversation types.Conversation) error {
	return nil
}

func (s *NoOpMemoryStore) GetConversation(ctx context.Context, filter Filter) (types.Conversation, error) {
	return types.Conversation{}, nil
}
