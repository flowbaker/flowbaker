package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"flowbaker/pkg/flowbaker"
)

type ExecutorKnowledgeManager interface {
	GetKnowledge(ctx context.Context, workspaceID string, knowledgeID string) (Knowledge, error)
	GetWorkspaceKnowledges(ctx context.Context, workspaceID string) ([]Knowledge, error)
	GetKnowledgeFiles(ctx context.Context, workspaceID string, knowledgeID string) ([]KnowledgeFile, error)
	GetKnowledgeFile(ctx context.Context, workspaceID string, knowledgeID string, fileID string) (KnowledgeFile, error)
	SearchKnowledge(ctx context.Context, workspaceID, knowledgeID string, params SearchKnowledgeParams) (SearchKnowledgeResult, error)
}

type SearchKnowledgeParams struct {
	WorkspaceID         string  `json:"workspace_id"`
	KnowledgeID         string  `json:"knowledge_id"`
	Query               string  `json:"query"`
	Limit               int     `json:"limit"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
}

type SearchKnowledgeResult struct {
	Query      string                      `json:"query"`
	Results    []SearchKnowledgeResultItem `json:"results"`
	TotalFound int                         `json:"total_found"`
}

type SearchKnowledgeResultItem struct {
	FileID   string  `json:"file_id"`
	FileName string  `json:"file_name"`
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
}

type executorKnowledgeManager struct {
	client flowbaker.ClientInterface
}

type ExecutorKnowledgeManagerDependencies struct {
	Client flowbaker.ClientInterface
}

func NewExecutorKnowledgeManager(deps ExecutorKnowledgeManagerDependencies) ExecutorKnowledgeManager {
	return &executorKnowledgeManager{
		client: deps.Client,
	}
}

func (m *executorKnowledgeManager) GetKnowledge(ctx context.Context, workspaceID string, knowledgeID string) (Knowledge, error) {
	responseJSON, err := m.client.GetKnowledge(ctx, workspaceID, knowledgeID)
	if err != nil {
		return Knowledge{}, fmt.Errorf("failed to get knowledge %s: %w", knowledgeID, err)
	}

	var domainKnowledge Knowledge

	if err := json.Unmarshal(responseJSON, &domainKnowledge); err != nil {
		return Knowledge{}, fmt.Errorf("failed to unmarshal knowledge response: %w", err)
	}

	return domainKnowledge, nil
}

func (m *executorKnowledgeManager) GetWorkspaceKnowledges(ctx context.Context, workspaceID string) ([]Knowledge, error) {
	responseJSON, err := m.client.GetWorkspaceKnowledges(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace knowledges: %w", err)
	}

	var response struct {
		Knowledges []Knowledge `json:"knowledges"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace knowledges response: %w", err)
	}

	return response.Knowledges, nil
}

func (m *executorKnowledgeManager) GetKnowledgeFiles(ctx context.Context, workspaceID string, knowledgeID string) ([]KnowledgeFile, error) {
	responseJSON, err := m.client.GetKnowledgeFiles(ctx, workspaceID, knowledgeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge files for %s: %w", knowledgeID, err)
	}

	var response struct {
		Files []KnowledgeFile `json:"files"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal knowledge files response: %w", err)
	}

	return response.Files, nil
}

func (m *executorKnowledgeManager) GetKnowledgeFile(ctx context.Context, workspaceID string, knowledgeID string, fileID string) (KnowledgeFile, error) {
	responseJSON, err := m.client.GetKnowledgeFile(ctx, workspaceID, knowledgeID, fileID)
	if err != nil {
		return KnowledgeFile{}, fmt.Errorf("failed to get knowledge file %s: %w", fileID, err)
	}

	var domainKnowledgeFile KnowledgeFile

	if err := json.Unmarshal(responseJSON, &domainKnowledgeFile); err != nil {
		return KnowledgeFile{}, fmt.Errorf("failed to unmarshal knowledge file response: %w", err)
	}

	return domainKnowledgeFile, nil
}

func (m *executorKnowledgeManager) SearchKnowledge(ctx context.Context, workspaceID, knowledgeID string, params SearchKnowledgeParams) (SearchKnowledgeResult, error) {
	responseJSON, err := m.client.SearchKnowledge(ctx, workspaceID, knowledgeID, &flowbaker.SearchKnowledgeRequest{
		Query:               params.Query,
		Limit:               params.Limit,
		SimilarityThreshold: params.SimilarityThreshold,
	})
	if err != nil {
		return SearchKnowledgeResult{}, fmt.Errorf("failed to search knowledge %s: %w", params.KnowledgeID, err)
	}

	var searchResult SearchKnowledgeResult

	if err := json.Unmarshal(responseJSON, &searchResult); err != nil {
		return SearchKnowledgeResult{}, fmt.Errorf("failed to unmarshal search knowledge response: %w", err)
	}

	return searchResult, nil
}
