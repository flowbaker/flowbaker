package managers

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

type executorModelManager struct {
	client flowbaker.ClientInterface
}

type ExecutorModelManagerDependencies struct {
	Client flowbaker.ClientInterface
}

func NewExecutorModelManager(deps ExecutorModelManagerDependencies) domain.ExecutorModelManager {
	return &executorModelManager{
		client: deps.Client,
	}
}

func (m *executorModelManager) ClassifyContent(ctx context.Context, params domain.ClassifyContentParams) (domain.ClassifyContentResult, error) {
	convertedCategories := make([]flowbaker.ContentClassificationCategory, len(params.Categories))

	for i, classification := range params.Categories {
		convertedCategories[i] = flowbaker.ContentClassificationCategory{
			Key:         classification.Key,
			Description: classification.Description,
		}
	}

	response, err := m.client.ClassifyContent(ctx, params.WorkspaceID, &flowbaker.ClassifyContentRequest{
		Content:    params.Content,
		Categories: convertedCategories,
	})
	if err != nil {
		return domain.ClassifyContentResult{}, fmt.Errorf("failed to classify content: %w", err)
	}

	return domain.ClassifyContentResult{
		SelectedClassificationCategory: response.SelectedCategory,
	}, nil
}
