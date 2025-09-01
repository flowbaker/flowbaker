package domain

import (
	"context"
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
