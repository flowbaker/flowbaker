package mongodb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	IntegrationActionType_InsertOne  domain.IntegrationActionType = "insert_one"
	IntegrationActionType_FindOne    domain.IntegrationActionType = "find_one"
	IntegrationActionType_UpdateOne  domain.IntegrationActionType = "update_one"
	IntegrationActionType_DeleteOne  domain.IntegrationActionType = "delete_one"
	IntegrationActionType_Aggregate  domain.IntegrationActionType = "aggregate"
	IntegrationActionType_InsertMany domain.IntegrationActionType = "insert_many"
	IntegrationActionType_FindMany   domain.IntegrationActionType = "find_many"
	IntegrationActionType_UpdateMany domain.IntegrationActionType = "update_many"
	IntegrationActionType_DeleteMany domain.IntegrationActionType = "delete_many"
)

const (
	MongoDBIntegrationPeekable_Databases   domain.IntegrationPeekableType = "databases"
	MongoDBIntegrationPeekable_Collections domain.IntegrationPeekableType = "collections"
)

type MongoDBIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[MongoDBCredential]
	binder           domain.IntegrationParameterBinder
}

func NewMongoDBIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &MongoDBIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[MongoDBCredential](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

type MongoDBCredential struct {
	URI string `json:"uri"`
}

func (c *MongoDBIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewMongoDBIntegration(ctx, MongoDBIntegrationDependencies{
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
	})
}

type MongoDBIntegration struct {
	credentialGetter domain.CredentialGetter[MongoDBCredential]
	binder           domain.IntegrationParameterBinder
	client           *mongo.Client

	actionManager *domain.IntegrationActionManager

	peekFuncs map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type MongoDBIntegrationDependencies struct {
	CredentialID     string
	CredentialGetter domain.CredentialGetter[MongoDBCredential]
	ParameterBinder  domain.IntegrationParameterBinder
}

func NewMongoDBIntegration(ctx context.Context, deps MongoDBIntegrationDependencies) (*MongoDBIntegration, error) {
	integration := &MongoDBIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_InsertOne, integration.InsertOne).
		AddPerItem(IntegrationActionType_FindOne, integration.FindOne).
		AddPerItem(IntegrationActionType_UpdateOne, integration.UpdateOne).
		AddPerItem(IntegrationActionType_DeleteOne, integration.DeleteOne).
		AddPerItem(IntegrationActionType_InsertMany, integration.InsertMany).
		AddPerItemMulti(IntegrationActionType_FindMany, integration.FindMany).
		AddPerItem(IntegrationActionType_UpdateMany, integration.UpdateMany).
		AddPerItem(IntegrationActionType_DeleteMany, integration.DeleteMany)

	integration.actionManager = actionManager

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		MongoDBIntegrationPeekable_Databases:   integration.PeekDatabases,
		MongoDBIntegrationPeekable_Collections: integration.PeekCollections,
	}

	integration.peekFuncs = peekFuncs

	if integration.client == nil {
		credential, err := integration.credentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
		if err != nil {
			return nil, err
		}

		client, err := mongo.Connect(ctx, options.Client().ApplyURI(credential.URI))
		if err != nil {
			return nil, err
		}

		integration.client = client
	}

	return integration, nil
}

func (i *MongoDBIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type MongoDBParams struct {
	Database   string   `json:"database"`
	Collection string   `json:"collection"`
	Filter     string   `json:"filter,omitempty"`
	Document   string   `json:"document,omitempty"`
	Documents  []string `json:"documents,omitempty"`
	Update     string   `json:"update,omitempty"`
}

func (i *MongoDBIntegration) InsertOne(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p MongoDBParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	document := bson.M{}

	err = json.Unmarshal([]byte(p.Document), &document)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	result, err := i.client.Database(p.Database).Collection(p.Collection).InsertOne(ctx, document)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return result, nil
}

func (i *MongoDBIntegration) FindOne(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p MongoDBParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	filter := bson.M{}

	err = json.Unmarshal([]byte(p.Filter), &filter)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	var result map[string]any

	err = i.client.Database(p.Database).Collection(p.Collection).FindOne(ctx, filter).Decode(&result)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return result, nil
}

func (i *MongoDBIntegration) UpdateOne(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p MongoDBParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	filter := bson.M{}

	err = json.Unmarshal([]byte(p.Filter), &filter)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	update := bson.M{}

	err = json.Unmarshal([]byte(p.Update), &update)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	result, err := i.client.Database(p.Database).Collection(p.Collection).UpdateOne(ctx, filter, update)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return result, nil
}

func (i *MongoDBIntegration) DeleteOne(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p MongoDBParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	filter := bson.M{}

	err = json.Unmarshal([]byte(p.Filter), &filter)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	result, err := i.client.Database(p.Database).Collection(p.Collection).DeleteOne(ctx, filter)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return result, nil
}

type FindManyParams struct {
	Database   string `json:"database"`
	Collection string `json:"collection"`
	Filter     string `json:"filter"`
	Limit      int64  `json:"limit"`
	Skip       int64  `json:"skip"`
}

func (i *MongoDBIntegration) FindMany(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var p FindManyParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	filter := bson.M{}

	err = json.Unmarshal([]byte(p.Filter), &filter)
	if err != nil {
		return nil, err
	}

	opts := options.Find()
	opts.SetLimit(p.Limit)
	opts.SetSkip(p.Skip)

	cursor, err := i.client.Database(p.Database).Collection(p.Collection).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	var results []domain.Item

	for cursor.Next(ctx) {
		var result map[string]any

		err := cursor.Decode(&result)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

type InsertManyParams struct {
	Database   string           `json:"database"`
	Collection string           `json:"collection"`
	Documents  []DocumentParams `json:"documents"`
}

type DocumentParams struct {
	Document string `json:"document"`
}

func (i *MongoDBIntegration) InsertMany(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p InsertManyParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	documents := make([]any, len(p.Documents))

	for i, doc := range p.Documents {
		var document bson.M

		err = json.Unmarshal([]byte(doc.Document), &document)
		if err != nil {
			return nil, err
		}

		documents[i] = document
	}

	result, err := i.client.Database(p.Database).Collection(p.Collection).InsertMany(ctx, documents)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (i *MongoDBIntegration) UpdateMany(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p MongoDBParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)

	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	filter := bson.M{}

	err = json.Unmarshal([]byte(p.Filter), &filter)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	update := bson.M{}

	err = json.Unmarshal([]byte(p.Update), &update)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	result, err := i.client.Database(p.Database).Collection(p.Collection).UpdateMany(ctx, filter, update)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return result, nil
}

func (i *MongoDBIntegration) DeleteMany(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p MongoDBParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	filter := bson.M{}

	err = json.Unmarshal([]byte(p.Filter), &filter)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	result, err := i.client.Database(p.Database).Collection(p.Collection).DeleteMany(ctx, filter)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return result, nil
}

/* func (i *MongoDBIntegration) Aggregate(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var p MongoDBParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	cursor, err := i.client.Database(p.Database).Collection(p.Collection).Aggregate(ctx, p.Documents)
	if err != nil {
		return nil, err
	}

	var results []domain.Item

	for cursor.Next(ctx) {
		var result map[string]any
		err := cursor.Decode(&result)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
} */

func (i *MongoDBIntegration) Peek(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[p.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peekable type not found")
	}

	return peekFunc(ctx, p)
}

func (i *MongoDBIntegration) PeekDatabases(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	offset := p.GetOffset()

	databases, err := i.client.ListDatabases(ctx, bson.D{})
	if err != nil {
		return domain.PeekResult{}, err
	}

	var allResults []domain.PeekResultItem
	for _, db := range databases.Databases {
		allResults = append(allResults, domain.PeekResultItem{
			Key:     db.Name,
			Value:   db.Name,
			Content: db.Name,
		})
	}

	totalCount := len(allResults)
	start := offset
	end := offset + limit

	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	var results []domain.PeekResultItem
	if start < end {
		results = allResults[start:end]
	}

	nextOffset := offset + len(results)
	hasMore := nextOffset < totalCount

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			Offset:  nextOffset,
			HasMore: hasMore,
		},
	}

	result.SetOffset(nextOffset)
	result.SetHasMore(hasMore)

	return result, nil
}

func (i *MongoDBIntegration) PeekCollections(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	offset := p.GetOffset()

	var params struct {
		Database string `json:"database"`
	}

	err := json.Unmarshal(p.PayloadJSON, &params)
	if err != nil {
		return domain.PeekResult{}, err
	}

	collections, err := i.client.Database(params.Database).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return domain.PeekResult{}, err
	}

	var allResults []domain.PeekResultItem
	for _, collection := range collections {
		allResults = append(allResults, domain.PeekResultItem{
			Key:     collection,
			Value:   collection,
			Content: collection,
		})
	}

	totalCount := len(allResults)
	start := offset
	end := offset + limit

	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	var results []domain.PeekResultItem
	if start < end {
		results = allResults[start:end]
	}

	nextOffset := offset + len(results)
	hasMore := nextOffset < totalCount

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			Offset:  nextOffset,
			HasMore: hasMore,
		},
	}

	result.SetOffset(nextOffset)
	result.SetHasMore(hasMore)

	return result, nil
}
