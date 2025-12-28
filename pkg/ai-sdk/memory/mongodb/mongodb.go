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

	// Use upsert to handle both insert and update
	filter := bson.M{
		"id": conversation.ID,
	}

	update := bson.M{
		"$set": bson.M{
			"session_id": conversation.SessionID,
			"user_id":    conversation.UserID,
			"messages":   conversation.Messages,
			"created_at": conversation.CreatedAt,
			"updated_at": conversation.UpdatedAt,
			"status":     conversation.Status,
			"metadata":   conversation.Metadata,
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

			_, err := s.collection.InsertOne(ctx, newConversation)
			if err != nil {
				return types.Conversation{}, fmt.Errorf("failed to insert new conversation: %w", err)
			}

			return newConversation, nil
		}

		return types.Conversation{}, fmt.Errorf("failed to find conversation: %w", result.Err())
	}

	var conversation types.Conversation

	if err := result.Decode(&conversation); err != nil {
		return types.Conversation{}, fmt.Errorf("failed to decode conversation: %w", err)
	}

	return conversation, nil
}
