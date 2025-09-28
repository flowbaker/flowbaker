package domain

import "context"

type ExecutorModelManager interface {
	ClassifyContent(ctx context.Context, params ClassifyContentParams) (ClassifyContentResult, error)
}

type ClassifyContentParams struct {
	WorkspaceID     string
	Content         string
	Classifications []Classification
}

type Classification struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type ClassifyContentResult struct {
	SelectedClassification string
}
