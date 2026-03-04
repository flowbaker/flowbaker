package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const aiChatConversationsCollection = "ai_chat_conversations"

// Store implements memory.Store interface using MongoDB
type Store struct {
	database   *mongo.Database
	collection *mongo.Collection
}

// New creates a new MongoDB memory store with the given database
func New(database *mongo.Database) *Store {
	store := &Store{
		database:   database,
		collection: database.Collection(aiChatConversationsCollection),
	}

	store.ensureIndexes()
	return store
}

// ensureIndexes creates necessary indexes for performance
func (s *Store) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := s.database.Collection(aiChatConversationsCollection)

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "session_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		fmt.Printf("Failed to create indexes for ai_chat_conversations: %v\n", err)
	}
}

// SaveConversation saves a new conversation or updates an existing one
func (s *Store) SaveConversation(ctx context.Context, conversation types.Conversation) error {
	if conversation.ID == "" {
		return types.ErrInvalidMessage
	}

	// Set timestamps
	now := time.Now()
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = now
	}
	conversation.UpdatedAt = now

	doc := conversationFromTypes(&conversation)

	// Use upsert to handle both insert and update
	filter := bson.M{
		"id": doc.ID,
	}

	update := bson.M{
		"$set": bson.M{
			"session_id": doc.SessionID,
			"user_id":    doc.UserID,
			"messages":   doc.Messages,
			"created_at": doc.CreatedAt,
			"updated_at": doc.UpdatedAt,
			"status":     doc.Status,
			"metadata":   doc.Metadata,
		},
	}

	_, err := s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	return nil
}

func (s *Store) GetConversation(ctx context.Context, filter memory.Filter) (types.Conversation, error) {
	mongoFilter := bson.M{}

	if filter.SessionID != "" {
		mongoFilter["session_id"] = filter.SessionID
	}

	if filter.UserID != "" {
		mongoFilter["user_id"] = filter.UserID
	}

	result := s.collection.FindOne(ctx, mongoFilter)

	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			newConversation := types.Conversation{
				ID:        uuid.New().String(),
				SessionID: filter.SessionID,
				UserID:    filter.UserID,
				Status:    types.StatusActive,
				Messages:  []types.Message{},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Metadata:  map[string]any{},
			}

			_, err := s.collection.InsertOne(ctx, conversationFromTypes(&newConversation))
			if err != nil {
				return types.Conversation{}, fmt.Errorf("failed to insert new conversation: %w", err)
			}

			return newConversation, nil
		}

		return types.Conversation{}, fmt.Errorf("failed to find conversation: %w", result.Err())
	}

	var doc conversationBson

	if err := result.Decode(&doc); err != nil {
		return types.Conversation{}, fmt.Errorf("failed to decode conversation: %w", err)
	}

	return doc.toTypes(), nil
}

// --- BSON types ---

type conversationBson struct {
	ID        string                 `bson:"id"`
	SessionID string                 `bson:"session_id"`
	UserID    string                 `bson:"user_id,omitempty"`
	Messages  []messageBson          `bson:"messages"`
	CreatedAt time.Time              `bson:"created_at"`
	UpdatedAt time.Time              `bson:"updated_at"`
	Status    string                 `bson:"status"`
	Metadata  map[string]interface{} `bson:"metadata,omitempty"`
}

func (c *conversationBson) toTypes() types.Conversation {
	messages := make([]types.Message, len(c.Messages))
	for i, m := range c.Messages {
		messages[i] = m.toTypes()
	}

	return types.Conversation{
		ID:        c.ID,
		SessionID: c.SessionID,
		UserID:    c.UserID,
		Messages:  messages,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Status:    types.ConversationStatus(c.Status),
		Metadata:  c.Metadata,
	}
}

func conversationFromTypes(c *types.Conversation) conversationBson {
	messages := make([]messageBson, len(c.Messages))
	for i, m := range c.Messages {
		messages[i] = messageFromTypes(&m)
	}

	return conversationBson{
		ID:        c.ID,
		SessionID: c.SessionID,
		UserID:    c.UserID,
		Messages:  messages,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Status:    string(c.Status),
		Metadata:  c.Metadata,
	}
}

type messageBson struct {
	ConversationID string           `bson:"conversation_id,omitempty"`
	Role           string           `bson:"role"`
	Content        string           `bson:"content"`
	ToolCalls      []toolCallBson   `bson:"tool_calls,omitempty"`
	ToolResults    []toolResultBson `bson:"tool_results,omitempty"`
	Timestamp      time.Time        `bson:"timestamp"`
	Metadata       map[string]any   `bson:"metadata,omitempty"`
	CreatedAt      time.Time        `bson:"created_at"`
}

func (m *messageBson) toTypes() types.Message {
	toolCalls := make([]types.ToolCall, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		toolCalls[i] = tc.toTypes()
	}

	toolResults := make([]types.ToolResult, len(m.ToolResults))
	for i, tr := range m.ToolResults {
		toolResults[i] = tr.toTypes()
	}

	return types.Message{
		ConversationID: m.ConversationID,
		Role:           types.MessageRole(m.Role),
		Content:        m.Content,
		ToolCalls:      toolCalls,
		ToolResults:    toolResults,
		Timestamp:      m.Timestamp,
		Metadata:       m.Metadata,
		CreatedAt:      m.CreatedAt,
	}
}

func messageFromTypes(m *types.Message) messageBson {
	toolCalls := make([]toolCallBson, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		toolCalls[i] = toolCallFromTypes(&tc)
	}

	toolResults := make([]toolResultBson, len(m.ToolResults))
	for i, tr := range m.ToolResults {
		toolResults[i] = toolResultFromTypes(&tr)
	}

	return messageBson{
		ConversationID: m.ConversationID,
		Role:           string(m.Role),
		Content:        m.Content,
		ToolCalls:      toolCalls,
		ToolResults:    toolResults,
		Timestamp:      m.Timestamp,
		Metadata:       m.Metadata,
		CreatedAt:      m.CreatedAt,
	}
}

type toolCallBson struct {
	ID        string         `bson:"id"`
	Name      string         `bson:"name"`
	Arguments map[string]any `bson:"arguments"`
	Metadata  map[string]any `bson:"metadata,omitempty"`
}

func (tc *toolCallBson) toTypes() types.ToolCall {
	return types.ToolCall{
		ID:        tc.ID,
		Name:      tc.Name,
		Arguments: tc.Arguments,
		Metadata:  tc.Metadata,
	}
}

func toolCallFromTypes(tc *types.ToolCall) toolCallBson {
	return toolCallBson{
		ID:        tc.ID,
		Name:      tc.Name,
		Arguments: tc.Arguments,
		Metadata:  tc.Metadata,
	}
}

type toolResultBson struct {
	ToolCallID string `bson:"tool_call_id"`
	ToolName   string `bson:"tool_name,omitempty"`
	Content    string `bson:"content"`
	IsError    bool   `bson:"is_error,omitempty"`
}

func (tr *toolResultBson) toTypes() types.ToolResult {
	return types.ToolResult{
		ToolCallID: tr.ToolCallID,
		ToolName:   tr.ToolName,
		Content:    tr.Content,
		IsError:    tr.IsError,
	}
}

func toolResultFromTypes(tr *types.ToolResult) toolResultBson {
	return toolResultBson{
		ToolCallID: tr.ToolCallID,
		ToolName:   tr.ToolName,
		Content:    tr.Content,
		IsError:    tr.IsError,
	}
}
