package mongodb

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const aiChatConversationsCollection = "ai_chat_conversations"

// Store implements memory.Store interface using MongoDB
type Store struct {
	database *mongo.Database
}

// New creates a new MongoDB memory store with the given database
func New(database *mongo.Database) *Store {
	store := &Store{
		database: database,
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
func (s *Store) SaveConversation(ctx context.Context, conversation *types.Conversation) error {
	log.Println("saving conversation status", conversation.Status)

	collection := s.database.Collection(aiChatConversationsCollection)

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
			"id":         conversation.ID,
			"session_id": conversation.SessionID,
			"user_id":    conversation.UserID,
			"messages":   conversation.Messages,
			"created_at": conversation.CreatedAt,
			"updated_at": conversation.UpdatedAt,
			"status":     conversation.Status,
			"metadata":   conversation.Metadata,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	return nil
}

// GetConversations retrieves conversations based on filter parameters
func (s *Store) GetConversations(ctx context.Context, filter memory.Filter) ([]*types.Conversation, error) {
	collection := s.database.Collection(aiChatConversationsCollection)

	// Build MongoDB filter
	mongoFilter := bson.M{}

	if filter.SessionID != "" {
		mongoFilter["session_id"] = filter.SessionID
	}

	if filter.UserID != "" {
		mongoFilter["user_id"] = filter.UserID
	}

	if filter.Status != "" {
		mongoFilter["status"] = filter.Status
	}

	// Set options
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "created_at", Value: -1}}) // Most recent first

	if filter.Limit > 0 {
		findOptions.SetLimit(int64(filter.Limit))
	}

	if filter.Offset > 0 {
		findOptions.SetSkip(int64(filter.Offset))
	}

	// Execute query
	cursor, err := collection.Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to find conversations: %w", err)
	}
	defer cursor.Close(ctx)

	var conversations []*types.Conversation
	if err := cursor.All(ctx, &conversations); err != nil {
		return nil, fmt.Errorf("failed to decode conversations: %w", err)
	}

	return conversations, nil
}

// GetConversation retrieves a single conversation by ID
func (s *Store) GetConversation(ctx context.Context, id string) (*types.Conversation, error) {
	collection := s.database.Collection(aiChatConversationsCollection)

	filter := bson.M{"id": id}

	var conversation types.Conversation
	err := collection.FindOne(ctx, filter).Decode(&conversation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find conversation: %w", err)
	}

	return &conversation, nil
}

// DeleteConversation removes a conversation by ID
func (s *Store) DeleteConversation(ctx context.Context, id string) error {
	collection := s.database.Collection(aiChatConversationsCollection)

	filter := bson.M{"id": id}

	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// DeleteOldConversations removes old conversations keeping only the specified count
func (s *Store) DeleteOldConversations(ctx context.Context, sessionID string, keepCount int) error {
	collection := s.database.Collection(aiChatConversationsCollection)

	// First, get the IDs of conversations to keep
	filter := bson.M{
		"session_id": sessionID,
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "created_at", Value: -1}})
	findOptions.SetLimit(int64(keepCount))
	findOptions.SetProjection(bson.M{"_id": 1})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return fmt.Errorf("failed to find conversations to keep: %w", err)
	}
	defer cursor.Close(ctx)

	var keepIDs []primitive.ObjectID
	for cursor.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err == nil {
			keepIDs = append(keepIDs, doc.ID)
		}
	}

	// Delete all conversations except the ones to keep
	deleteFilter := bson.M{
		"session_id": sessionID,
	}

	if len(keepIDs) > 0 {
		deleteFilter["_id"] = bson.M{"$nin": keepIDs}
	}

	result, err := collection.DeleteMany(ctx, deleteFilter)
	if err != nil {
		return fmt.Errorf("failed to delete old conversations: %w", err)
	}

	if result.DeletedCount > 0 {
		fmt.Printf("Deleted %d old conversations for session %s\n", result.DeletedCount, sessionID)
	}

	return nil
}
