package inmemory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

type Store struct {
	mu            sync.RWMutex
	conversations map[string]*types.Conversation
	sessionIndex  map[string][]string
	userIndex     map[string][]string
}

func New() *Store {
	return &Store{
		conversations: make(map[string]*types.Conversation),
		sessionIndex:  make(map[string][]string),
		userIndex:     make(map[string][]string),
	}
}

func (s *Store) SaveConversation(ctx context.Context, conversation *types.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conversation.ID == "" {
		return types.ErrInvalidMessage
	}

	conversation.UpdatedAt = time.Now()
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = time.Now()
	}

	s.conversations[conversation.ID] = conversation

	if conversation.SessionID != "" {
		if !contains(s.sessionIndex[conversation.SessionID], conversation.ID) {
			s.sessionIndex[conversation.SessionID] = append(s.sessionIndex[conversation.SessionID], conversation.ID)
		}
	}

	if conversation.UserID != "" {
		if !contains(s.userIndex[conversation.UserID], conversation.ID) {
			s.userIndex[conversation.UserID] = append(s.userIndex[conversation.UserID], conversation.ID)
		}
	}

	return nil
}

func (s *Store) GetConversations(ctx context.Context, filter memory.Filter) ([]*types.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ids []string

	if filter.SessionID != "" {
		ids = s.sessionIndex[filter.SessionID]
	} else if filter.UserID != "" {
		ids = s.userIndex[filter.UserID]
	} else {
		for id := range s.conversations {
			ids = append(ids, id)
		}
	}

	var conversations []*types.Conversation
	for _, id := range ids {
		if conv, ok := s.conversations[id]; ok {
			if filter.Status != "" && conv.Status != filter.Status {
				continue
			}
			conversations = append(conversations, conv)
		}
	}

	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].CreatedAt.After(conversations[j].CreatedAt)
	})

	if filter.Offset > 0 && filter.Offset < len(conversations) {
		conversations = conversations[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(conversations) {
		conversations = conversations[:filter.Limit]
	}

	return conversations, nil
}

func (s *Store) GetConversation(ctx context.Context, id string) (*types.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if conv, ok := s.conversations[id]; ok {
		return conv, nil
	}

	return nil, nil
}

func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conv, ok := s.conversations[id]; ok {
		if conv.SessionID != "" {
			s.sessionIndex[conv.SessionID] = removeString(s.sessionIndex[conv.SessionID], id)
		}
		if conv.UserID != "" {
			s.userIndex[conv.UserID] = removeString(s.userIndex[conv.UserID], id)
		}
		delete(s.conversations, id)
	}

	return nil
}

func (s *Store) DeleteOldConversations(ctx context.Context, sessionID string, keepCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.sessionIndex[sessionID]
	if len(ids) <= keepCount {
		return nil
	}

	var conversations []*types.Conversation
	for _, id := range ids {
		if conv, ok := s.conversations[id]; ok {
			conversations = append(conversations, conv)
		}
	}

	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].CreatedAt.After(conversations[j].CreatedAt)
	})

	toDelete := conversations[keepCount:]
	for _, conv := range toDelete {
		delete(s.conversations, conv.ID)
		s.sessionIndex[sessionID] = removeString(s.sessionIndex[sessionID], conv.ID)
		if conv.UserID != "" {
			s.userIndex[conv.UserID] = removeString(s.userIndex[conv.UserID], conv.ID)
		}
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
