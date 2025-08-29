package knowledge

import (
	"context"
	"flowbaker/internal/domain"
	"fmt"

	"github.com/rs/zerolog/log"
)

const (
	KnowledgeIntegrationPeekable_Knowledges domain.IntegrationPeekableType = "knowledges"
)

type KnowledgeIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	knowledgeManager domain.ExecutorKnowledgeManager
}

func NewKnowledgeIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &KnowledgeIntegrationCreator{
		binder:           deps.ParameterBinder,
		knowledgeManager: deps.ExecutorKnowledgeManager,
	}
}

func (c *KnowledgeIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewKnowledgeIntegration(KnowledgeIntegrationDependencies{
		ParameterBinder:  c.binder,
		KnowledgeManager: c.knowledgeManager,
		WorkspaceID:      p.WorkspaceID,
	})
}

type KnowledgeIntegration struct {
	binder           domain.IntegrationParameterBinder
	knowledgeManager domain.ExecutorKnowledgeManager
	workspaceID      string

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context) (domain.PeekResult, error)
}

type KnowledgeIntegrationDependencies struct {
	ParameterBinder  domain.IntegrationParameterBinder
	KnowledgeManager domain.ExecutorKnowledgeManager
	WorkspaceID      string
}

func NewKnowledgeIntegration(deps KnowledgeIntegrationDependencies) (*KnowledgeIntegration, error) {
	integration := &KnowledgeIntegration{
		binder:           deps.ParameterBinder,
		knowledgeManager: deps.KnowledgeManager,
		workspaceID:      deps.WorkspaceID,
	}

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context) (domain.PeekResult, error){
		KnowledgeIntegrationPeekable_Knowledges: integration.PeekKnowledges,
	}

	integration.peekFuncs = peekFuncs

	integration.actionManager = domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_Search, integration.SearchKnowledge)

	return integration, nil
}

func (i *KnowledgeIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msgf("Executing Knowledge integration")

	return i.actionManager.Run(ctx, params.ActionType, params)
}

type SearchKnowledgeParams struct {
	KnowledgeBaseID     string  `json:"knowledge_base_id"`
	Query               string  `json:"query"`
	Limit               int     `json:"limit"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
}

func (i *KnowledgeIntegration) SearchKnowledge(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SearchKnowledgeParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Validate required parameters
	if p.KnowledgeBaseID == "" {
		return nil, fmt.Errorf("knowledge_base_id is required")
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Set defaults if not provided
	if p.Limit <= 0 {
		p.Limit = 10
	}
	if p.SimilarityThreshold <= 0 {
		p.SimilarityThreshold = 0.7
	}

	// Use ExecutorKnowledgeManager to perform search
	searchResult, err := i.knowledgeManager.SearchKnowledge(ctx, i.workspaceID, p.KnowledgeBaseID, domain.SearchKnowledgeParams{
		WorkspaceID:         i.workspaceID,
		KnowledgeID:         p.KnowledgeBaseID,
		Query:               p.Query,
		Limit:               p.Limit,
		SimilarityThreshold: p.SimilarityThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge: %w", err)
	}

	// Convert search result to integration output format
	result := map[string]interface{}{
		"query":        searchResult.Query,
		"knowledge_id": p.KnowledgeBaseID,
		"results":      searchResult.Results,
		"total_found":  searchResult.TotalFound,
	}

	return result, nil
}

func (i *KnowledgeIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	log.Info().Interface("params", params).Msg("Peeking knowledge")

	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx)
}

func (i *KnowledgeIntegration) PeekKnowledges(ctx context.Context) (domain.PeekResult, error) {
	knowledges, err := i.knowledgeManager.GetWorkspaceKnowledges(ctx, i.workspaceID)
	if err != nil {
		return domain.PeekResult{}, err
	}

	var peekResultItems []domain.PeekResultItem
	for _, knowledge := range knowledges {
		peekResultItems = append(peekResultItems, domain.PeekResultItem{
			Key:     knowledge.ID,
			Value:   knowledge.ID,
			Content: knowledge.Name,
		})
	}

	return domain.PeekResult{
		Result: peekResultItems,
	}, nil
}
