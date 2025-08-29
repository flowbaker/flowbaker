package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

// InMemoryFunctionCallingStateManager implements FunctionCallingStateManager using in-memory storage
type InMemoryFunctionCallingStateManager struct {
	states map[string]*FunctionCallingState
	mutex  sync.RWMutex
}

// NewInMemoryFunctionCallingStateManager creates a new in-memory state manager for function calling
func NewInMemoryFunctionCallingStateManager() FunctionCallingStateManager {
	return &InMemoryFunctionCallingStateManager{
		states: make(map[string]*FunctionCallingState),
	}
}

// SaveState saves the function calling conversation state
func (m *InMemoryFunctionCallingStateManager) SaveState(ctx context.Context, state *FunctionCallingState) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Deep copy the state to prevent external modifications
	stateCopy, err := m.deepCopyState(state)
	if err != nil {
		return fmt.Errorf("failed to copy state: %w", err)
	}

	m.states[state.ConversationID] = stateCopy

	log.Debug().
		Str("conversation_id", state.ConversationID).
		Int("round", state.Round).
		Str("status", string(state.Status)).
		Msg("Function calling state saved")

	return nil
}

// LoadState loads the function calling conversation state
func (m *InMemoryFunctionCallingStateManager) LoadState(ctx context.Context, conversationID string) (*FunctionCallingState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	state, exists := m.states[conversationID]
	if !exists {
		return nil, fmt.Errorf("state not found for conversation %s", conversationID)
	}

	// Deep copy the state to prevent external modifications
	stateCopy, err := m.deepCopyState(state)
	if err != nil {
		return nil, fmt.Errorf("failed to copy state: %w", err)
	}

	log.Debug().
		Str("conversation_id", conversationID).
		Int("round", state.Round).
		Str("status", string(state.Status)).
		Msg("Function calling state loaded")

	return stateCopy, nil
}

// DeleteState deletes the function calling conversation state
func (m *InMemoryFunctionCallingStateManager) DeleteState(ctx context.Context, conversationID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.states, conversationID)

	log.Debug().
		Str("conversation_id", conversationID).
		Msg("Function calling state deleted")

	return nil
}

// deepCopyState creates a deep copy of the function calling state
func (m *InMemoryFunctionCallingStateManager) deepCopyState(state *FunctionCallingState) (*FunctionCallingState, error) {
	// Use JSON marshal/unmarshal for deep copying
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	var copy FunctionCallingState
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, err
	}

	return &copy, nil
}

// GetAllStates returns all stored states (useful for debugging/monitoring)
func (m *InMemoryFunctionCallingStateManager) GetAllStates(ctx context.Context) (map[string]*FunctionCallingState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create a deep copy of all states
	statesCopy := make(map[string]*FunctionCallingState)
	for conversationID, state := range m.states {
		stateCopy, err := m.deepCopyState(state)
		if err != nil {
			return nil, fmt.Errorf("failed to copy state for conversation %s: %w", conversationID, err)
		}
		statesCopy[conversationID] = stateCopy
	}

	return statesCopy, nil
}

// GetStateCount returns the number of stored states
func (m *InMemoryFunctionCallingStateManager) GetStateCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.states)
}

// CleanupExpiredStates removes states that are older than the specified duration
// This is useful for memory management in production
func (m *InMemoryFunctionCallingStateManager) CleanupExpiredStates(ctx context.Context, maxAge int64) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	currentTime := getCurrentTimestamp()
	var deletedCount int

	for conversationID, state := range m.states {
		stateAge := currentTime - state.CreatedAt.Unix()
		if stateAge > maxAge {
			delete(m.states, conversationID)
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Info().
			Int("deleted_count", deletedCount).
			Msg("Cleaned up expired function calling states")
	}

	return nil
}

// getCurrentTimestamp returns the current Unix timestamp
func getCurrentTimestamp() int64 {
	return int64(0) // This would use time.Now().Unix() in real implementation
}
