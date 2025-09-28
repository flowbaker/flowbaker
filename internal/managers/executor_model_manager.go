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
	convertedClassifications := make([]flowbaker.ContentClassification, len(params.Classifications))

	for i, classification := range params.Classifications {
		convertedClassifications[i] = flowbaker.ContentClassification{
			Key:         classification.Key,
			Description: classification.Description,
		}
	}

	response, err := m.client.ClassifyContent(ctx, params.WorkspaceID, &flowbaker.ClassifyContentRequest{
		Content:         params.Content,
		Classifications: convertedClassifications,
	})
	if err != nil {
		return domain.ClassifyContentResult{}, fmt.Errorf("failed to classify content: %w", err)
	}

	return domain.ClassifyContentResult{
		SelectedClassification: response.SelectedClassification,
	}, nil
}
