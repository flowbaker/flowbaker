package managers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type executorKnowledgeManager struct {
	client flowbaker.ClientInterface
}

type ExecutorKnowledgeManagerDependencies struct {
	Client flowbaker.ClientInterface
}

func NewExecutorKnowledgeManager(deps ExecutorKnowledgeManagerDependencies) domain.ExecutorKnowledgeManager {
	return &executorKnowledgeManager{
		client: deps.Client,
	}
}

func (m *executorKnowledgeManager) GetKnowledge(ctx context.Context, workspaceID string, knowledgeID string) (domain.Knowledge, error) {
	responseJSON, err := m.client.GetKnowledge(ctx, workspaceID, knowledgeID)
	if err != nil {
		return domain.Knowledge{}, fmt.Errorf("failed to get knowledge %s: %w", knowledgeID, err)
	}

	var domainKnowledge domain.Knowledge

	if err := json.Unmarshal(responseJSON, &domainKnowledge); err != nil {
		return domain.Knowledge{}, fmt.Errorf("failed to unmarshal knowledge response: %w", err)
	}

	return domainKnowledge, nil
}

func (m *executorKnowledgeManager) GetWorkspaceKnowledges(ctx context.Context, workspaceID string) ([]domain.Knowledge, error) {
	responseJSON, err := m.client.GetWorkspaceKnowledges(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace knowledges: %w", err)
	}

	var response struct {
		Knowledges []domain.Knowledge `json:"knowledges"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace knowledges response: %w", err)
	}

	return response.Knowledges, nil
}

func (m *executorKnowledgeManager) GetKnowledgeFiles(ctx context.Context, workspaceID string, knowledgeID string) ([]domain.KnowledgeFile, error) {
	responseJSON, err := m.client.GetKnowledgeFiles(ctx, workspaceID, knowledgeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge files for %s: %w", knowledgeID, err)
	}

	var response struct {
		Files []domain.KnowledgeFile `json:"files"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal knowledge files response: %w", err)
	}

	return response.Files, nil
}

func (m *executorKnowledgeManager) GetKnowledgeFile(ctx context.Context, workspaceID string, knowledgeID string, fileID string) (domain.KnowledgeFile, error) {
	responseJSON, err := m.client.GetKnowledgeFile(ctx, workspaceID, knowledgeID, fileID)
	if err != nil {
		return domain.KnowledgeFile{}, fmt.Errorf("failed to get knowledge file %s: %w", fileID, err)
	}

	var domainKnowledgeFile domain.KnowledgeFile

	if err := json.Unmarshal(responseJSON, &domainKnowledgeFile); err != nil {
		return domain.KnowledgeFile{}, fmt.Errorf("failed to unmarshal knowledge file response: %w", err)
	}

	return domainKnowledgeFile, nil
}

func (m *executorKnowledgeManager) SearchKnowledge(ctx context.Context, workspaceID, knowledgeID string, params domain.SearchKnowledgeParams) (domain.SearchKnowledgeResult, error) {
	responseJSON, err := m.client.SearchKnowledge(ctx, workspaceID, knowledgeID, &flowbaker.SearchKnowledgeRequest{
		Query:               params.Query,
		Limit:               params.Limit,
		SimilarityThreshold: params.SimilarityThreshold,
	})
	if err != nil {
		return domain.SearchKnowledgeResult{}, fmt.Errorf("failed to search knowledge %s: %w", params.KnowledgeID, err)
	}

	var searchResult domain.SearchKnowledgeResult

	if err := json.Unmarshal(responseJSON, &searchResult); err != nil {
		return domain.SearchKnowledgeResult{}, fmt.Errorf("failed to unmarshal search knowledge response: %w", err)
	}

	return searchResult, nil
}
