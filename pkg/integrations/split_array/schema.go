package split_array

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_SplitArray domain.IntegrationActionType = "split_array"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_SplitArray,
		Name:                 "Split Array",
		Description:          "Extract an array field and convert each element into a separate output item",
		IsCredentialOptional: true,
		Actions: []domain.IntegrationAction{
			{
				ID:          string(IntegrationActionType_SplitArray),
				Name:        "Split Array",
				ActionType:  IntegrationActionType_SplitArray,
				Description: "Extract an array from a field path and output each array element as a separate item. Supports nested paths (e.g., 'data.results.items')",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionBottom, Text: "Output"},
						},
					},
				},
				Properties: []domain.NodeProperty{
					{
						Key:                 "field_path",
						Name:                "Array Field Path",
						Description:         "The path to the array field to split. Supports nested paths using dot notation (e.g., 'items' or 'data.results.items')",
						Required:            true,
						Type:                domain.NodePropertyType_String,
						DragAndDropBehavior: domain.DragAndDropBehavior_BasicPath,
						Placeholder:         "items or data.results.items",
						Help:                "Each element in the array will become a separate output item. The field must exist and must be an array.",
					},
				},
			},
		},
	}
)
