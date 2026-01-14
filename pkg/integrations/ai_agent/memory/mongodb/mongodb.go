package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/flowbaker/flowbaker/pkg/domain"
	mongodb "github.com/flowbaker/flowbaker/pkg/integrations/mongo"
	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	database          *mongo.Database
	conversationsColl *mongo.Collection
	messagesColl      *mongo.Collection
}

type Opts struct {
	CredentialID   string
	DatabaseName   string
	CollectionName string
}

type StoreDeps struct {
	Context          context.Context
	CredentialGetter domain.CredentialGetter[mongodb.MongoDBCredential]
}

type conversation struct {
	ID        string         `bson:"id"`
	SessionID string         `bson:"session_id"`
	UserID    string         `bson:"user_id,omitempty"`
	CreatedAt time.Time      `bson:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at"`
	Status    string         `bson:"status"`
	Metadata  map[string]any `bson:"metadata,omitempty"`
}

type message struct {
	ConversationID string         `bson:"conversation_id"`
	Role           string         `bson:"role"`
	Order          int            `bson:"order"`
	Content        string         `bson:"content"`
	ToolCalls      []toolCall     `bson:"tool_calls,omitempty"`
	ToolResults    []toolResult   `bson:"tool_results,omitempty"`
	Timestamp      time.Time      `bson:"timestamp"`
	Metadata       map[string]any `bson:"metadata,omitempty"`
	CreatedAt      time.Time      `bson:"created_at"`
}

type toolCall struct {
	ID        string         `bson:"id"`
	Name      string         `bson:"name"`
	Arguments map[string]any `bson:"arguments"`
}

type toolResult struct {
	ToolCallID string `bson:"tool_call_id"`
	Content    string `bson:"content"`
	IsError    bool   `bson:"is_error,omitempty"`
}

func conversationFromTypes(c types.Conversation) conversation {
	return conversation{
		ID:        c.ID,
		SessionID: c.SessionID,
		UserID:    c.UserID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Status:    string(c.Status),
		Metadata:  c.Metadata,
	}
}

func (c conversation) toTypes(messages []types.Message) types.Conversation {
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

func messageFromTypes(m types.Message, order int) message {
	toolCalls := make([]toolCall, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		toolCalls[i] = toolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		}
	}

	toolResults := make([]toolResult, len(m.ToolResults))
	for i, tr := range m.ToolResults {
		toolResults[i] = toolResult{
			ToolCallID: tr.ToolCallID,
			Content:    tr.Content,
			IsError:    tr.IsError,
		}
	}

	return message{
		ConversationID: m.ConversationID,
		Role:           string(m.Role),
		Order:          order,
		Content:        m.Content,
		ToolCalls:      toolCalls,
		ToolResults:    toolResults,
		Timestamp:      m.Timestamp,
		Metadata:       m.Metadata,
		CreatedAt:      m.CreatedAt,
	}
}

func (m message) toTypes() types.Message {
	toolCalls := make([]types.ToolCall, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		toolCalls[i] = types.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		}
	}

	toolResults := make([]types.ToolResult, len(m.ToolResults))
	for i, tr := range m.ToolResults {
		toolResults[i] = types.ToolResult{
			ToolCallID: tr.ToolCallID,
			Content:    tr.Content,
			IsError:    tr.IsError,
		}
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

func New(deps StoreDeps, opts Opts) (*Store, error) {
	if opts.DatabaseName == "" {
		return nil, fmt.Errorf("database name is required for mongodb memory store")
	}

	if opts.CollectionName == "" {
		return nil, fmt.Errorf("collection name is required for mongodb memory store")
	}

	credential, err := deps.CredentialGetter.GetDecryptedCredential(deps.Context, opts.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	client, err := mongo.Connect(deps.Context, options.Client().ApplyURI(credential.URI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	database := client.Database(opts.DatabaseName)

	store := &Store{
		database:          database,
		conversationsColl: database.Collection(opts.CollectionName),
		messagesColl:      database.Collection(opts.CollectionName + "_messages"),
	}

	store.ensureIndexes()

	return store, nil
}

func (s *Store) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	convIndexes := []mongo.IndexModel{
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
	}

	_, err := s.conversationsColl.Indexes().CreateMany(ctx, convIndexes)
	if err != nil {
		fmt.Printf("Failed to create indexes for conversations: %v\n", err)
	}

	msgIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "conversation_id", Value: 1},
				{Key: "order", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "conversation_id", Value: 1},
				{Key: "created_at", Value: 1},
			},
		},
	}

	_, err = s.messagesColl.Indexes().CreateMany(ctx, msgIndexes)
	if err != nil {
		fmt.Printf("Failed to create indexes for messages: %v\n", err)
	}
}

func (s *Store) SaveConversation(ctx context.Context, conv types.Conversation) error {
	if conv.ID == "" {
		return types.ErrInvalidMessage
	}

	now := time.Now()
	conv.UpdatedAt = now

	doc := conversationFromTypes(conv)

	filter := bson.M{"id": conv.ID}

	update := bson.M{
		"$set": bson.M{
			"updated_at": doc.UpdatedAt,
			"status":     doc.Status,
			"metadata":   doc.Metadata,
		},
	}

	_, err := s.conversationsColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	var newMessages []any

	for i := range conv.Messages {
		msg := &conv.Messages[i]
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = now
			msg.ConversationID = conv.ID
			newMessages = append(newMessages, messageFromTypes(*msg, i))
		}
	}

	if len(newMessages) > 0 {
		_, err = s.messagesColl.InsertMany(ctx, newMessages)
		if err != nil {
			return fmt.Errorf("failed to insert messages: %w", err)
		}
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

	result := s.conversationsColl.FindOne(ctx, mongoFilter)

	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			now := time.Now()
			newConv := conversation{
				ID:        uuid.New().String(),
				SessionID: filter.SessionID,
				UserID:    filter.UserID,
				Status:    string(types.StatusActive),
				CreatedAt: now,
				UpdatedAt: now,
				Metadata:  map[string]any{},
			}

			_, err := s.conversationsColl.InsertOne(ctx, newConv)
			if err != nil {
				return types.Conversation{}, fmt.Errorf("failed to insert new conversation: %w", err)
			}

			return newConv.toTypes([]types.Message{}), nil
		}

		return types.Conversation{}, fmt.Errorf("failed to find conversation: %w", result.Err())
	}

	var conv conversation
	if err := result.Decode(&conv); err != nil {
		return types.Conversation{}, fmt.Errorf("failed to decode conversation: %w", err)
	}

	// Build message filter
	messageFilter := bson.M{"conversation_id": conv.ID}

	// Add cursor filter if "before" is specified
	if filter.Before != nil {
		messageFilter["order"] = bson.M{"$lt": *filter.Before}
	}

	// Build find options
	opts := options.Find()

	if filter.Limit > 0 {
		// Sort DESC to get newest messages first when paginating
		opts.SetSort(bson.D{{Key: "order", Value: -1}})
		opts.SetLimit(int64(filter.Limit))
	} else {
		// No pagination - get all messages in chronological order
		opts.SetSort(bson.D{{Key: "order", Value: 1}})
	}

	cursor, err := s.messagesColl.Find(ctx, messageFilter, opts)
	if err != nil {
		return types.Conversation{}, fmt.Errorf("failed to find messages: %w", err)
	}
	defer cursor.Close(ctx)

	var msgs []message

	if err := cursor.All(ctx, &msgs); err != nil {
		return types.Conversation{}, fmt.Errorf("failed to decode messages: %w", err)
	}

	// If we sorted DESC for pagination, reverse to chronological order
	if filter.Limit > 0 {
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
	}

	messages := make([]types.Message, len(msgs))

	for i, m := range msgs {
		messages[i] = m.toTypes()
	}

	return conv.toTypes(messages), nil
}
