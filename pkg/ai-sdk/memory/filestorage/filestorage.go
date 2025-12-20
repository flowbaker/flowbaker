package filestorage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

type Store struct {
	mu      sync.RWMutex
	baseDir string
	index   *index
}

type index struct {
	SessionIndex map[string][]string `json:"session_index"`
	UserIndex    map[string][]string `json:"user_index"`
}

// New creates a new file storage with the given base directory
// If baseDir is empty, it defaults to "./conversations"
func New(baseDir string) (*Store, error) {
	if baseDir == "" {
		baseDir = "./conversations"
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	store := &Store{
		baseDir: baseDir,
		index: &index{
			SessionIndex: make(map[string][]string),
			UserIndex:    make(map[string][]string),
		},
	}

	// Load existing index if it exists
	if err := store.loadIndex(); err != nil {
		// If index doesn't exist, rebuild it from existing files
		if os.IsNotExist(err) {
			if err := store.rebuildIndex(); err != nil {
				return nil, fmt.Errorf("failed to rebuild index: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load index: %w", err)
		}
	}

	return store, nil
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

	// Write conversation to file
	filePath := s.conversationPath(conversation.ID)
	data, err := json.MarshalIndent(conversation, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write conversation file: %w", err)
	}

	// Update indexes
	if conversation.SessionID != "" {
		if !contains(s.index.SessionIndex[conversation.SessionID], conversation.ID) {
			s.index.SessionIndex[conversation.SessionID] = append(s.index.SessionIndex[conversation.SessionID], conversation.ID)
		}
	}

	if conversation.UserID != "" {
		if !contains(s.index.UserIndex[conversation.UserID], conversation.ID) {
			s.index.UserIndex[conversation.UserID] = append(s.index.UserIndex[conversation.UserID], conversation.ID)
		}
	}

	// Save index
	if err := s.saveIndex(); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

func (s *Store) GetConversations(ctx context.Context, filter memory.Filter) ([]*types.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ids []string

	if filter.SessionID != "" {
		ids = s.index.SessionIndex[filter.SessionID]
	} else if filter.UserID != "" {
		ids = s.index.UserIndex[filter.UserID]
	} else {
		// Get all conversation IDs
		entries, err := os.ReadDir(s.baseDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read conversations directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
				if id != "index" {                       // Skip index file
					ids = append(ids, id)
				}
			}
		}
	}

	var conversations []*types.Conversation
	for _, id := range ids {
		conv, err := s.loadConversation(id)
		if err != nil {
			// Skip conversations that can't be loaded
			continue
		}
		if conv == nil {
			continue
		}

		if filter.Status != "" && conv.Status != filter.Status {
			continue
		}
		conversations = append(conversations, conv)
	}

	// Sort by creation time (newest first)
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].CreatedAt.After(conversations[j].CreatedAt)
	})

	// Apply pagination
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

	return s.loadConversation(id)
}

func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load conversation to get session and user IDs
	conv, err := s.loadConversation(id)
	if err != nil || conv == nil {
		// If conversation doesn't exist, nothing to delete
		return nil
	}

	// Remove from indexes
	if conv.SessionID != "" {
		s.index.SessionIndex[conv.SessionID] = removeString(s.index.SessionIndex[conv.SessionID], id)
	}
	if conv.UserID != "" {
		s.index.UserIndex[conv.UserID] = removeString(s.index.UserIndex[conv.UserID], id)
	}

	// Delete file
	filePath := s.conversationPath(id)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete conversation file: %w", err)
	}

	// Save updated index
	if err := s.saveIndex(); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

func (s *Store) DeleteOldConversations(ctx context.Context, sessionID string, keepCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.index.SessionIndex[sessionID]
	if len(ids) <= keepCount {
		return nil
	}

	// Load all conversations for this session
	var conversations []*types.Conversation
	for _, id := range ids {
		conv, err := s.loadConversation(id)
		if err != nil || conv == nil {
			continue
		}
		conversations = append(conversations, conv)
	}

	// Sort by creation time (newest first)
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].CreatedAt.After(conversations[j].CreatedAt)
	})

	// Delete old conversations
	toDelete := conversations[keepCount:]
	for _, conv := range toDelete {
		// Remove from indexes
		s.index.SessionIndex[sessionID] = removeString(s.index.SessionIndex[sessionID], conv.ID)
		if conv.UserID != "" {
			s.index.UserIndex[conv.UserID] = removeString(s.index.UserIndex[conv.UserID], conv.ID)
		}

		// Delete file
		filePath := s.conversationPath(conv.ID)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete conversation file: %w", err)
		}
	}

	// Save updated index
	if err := s.saveIndex(); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

// Helper methods

func (s *Store) conversationPath(id string) string {
	return filepath.Join(s.baseDir, id+".json")
}

func (s *Store) indexPath() string {
	return filepath.Join(s.baseDir, "index.json")
}

func (s *Store) loadConversation(id string) (*types.Conversation, error) {
	filePath := s.conversationPath(id)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read conversation file: %w", err)
	}

	var conv types.Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
	}

	return &conv, nil
}

func (s *Store) loadIndex() error {
	filePath := s.indexPath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, s.index)
}

func (s *Store) saveIndex() error {
	data, err := json.MarshalIndent(s.index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.indexPath(), data, 0644)
}

func (s *Store) rebuildIndex() error {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		if id == "index" {
			continue
		}

		conv, err := s.loadConversation(id)
		if err != nil || conv == nil {
			continue
		}

		// Update indexes
		if conv.SessionID != "" {
			if !contains(s.index.SessionIndex[conv.SessionID], conv.ID) {
				s.index.SessionIndex[conv.SessionID] = append(s.index.SessionIndex[conv.SessionID], conv.ID)
			}
		}

		if conv.UserID != "" {
			if !contains(s.index.UserIndex[conv.UserID], conv.ID) {
				s.index.UserIndex[conv.UserID] = append(s.index.UserIndex[conv.UserID], conv.ID)
			}
		}
	}

	return s.saveIndex()
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
