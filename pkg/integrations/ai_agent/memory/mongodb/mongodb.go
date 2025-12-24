package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/flowbaker/flowbaker/pkg/domain"
	mongodb "github.com/flowbaker/flowbaker/pkg/integrations/mongo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store implements memory.Store interface using MongoDB
type Store struct {
	database   *mongo.Database
	collection *mongo.Collection
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

// New creates a new MongoDB memory store with the given database
func New(deps StoreDeps, opts Opts) (*Store, error) {
	credential, err := deps.CredentialGetter.GetDecryptedCredential(deps.Context, opts.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	client, err := mongo.Connect(deps.Context, options.Client().ApplyURI(credential.URI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	database := client.Database(opts.DatabaseName)
	collection := database.Collection(opts.CollectionName)

	store := &Store{
		database:   database,
		collection: collection,
	}

	store.ensureIndexes()

	return store, nil
}

// ensureIndexes creates necessary indexes for performance
func (s *Store) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	_, err := s.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		fmt.Printf("Failed to create indexes for ai_chat_conversations: %v\n", err)
	}
}

// SaveConversation saves a new conversation or updates an existing one
func (s *Store) SaveConversation(ctx context.Context, conversation *types.Conversation) error {
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
	_, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	return nil
}

// GetConversations retrieves conversations based on filter parameters
func (s *Store) GetConversations(ctx context.Context, filter memory.Filter) ([]*types.Conversation, error) {
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
	cursor, err := s.collection.Find(ctx, mongoFilter, findOptions)
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
