package items_to_item

import "github.com/flowbaker/flowbaker/pkg/domain"

const (
	IntegrationActionType_ItemsToItem domain.IntegrationActionType = "items_to_item"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_ItemsToItem,
		Name:                 "Items To Item",
		Description:          "Convert items to a single item",
		IsCredentialOptional: true,
		Actions: []domain.IntegrationAction{
			{
				ID:          string(IntegrationActionType_ItemsToItem),
				Name:        "Items To Item",
				ActionType:  IntegrationActionType_ItemsToItem,
				Description: "Convert an array of items to a single item",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				Properties: []domain.NodeProperty{
					{
						Key:                 "field_path",
						Name:                "Field Path",
						Description:         "The path to the field to convert to a single item. If not provided, it will be named as 'items' by default",
						Required:            false,
						Type:                domain.NodePropertyType_String,
						DragAndDropBehavior: domain.DragAndDropBehavior_BasicPath,
					},
				},
			},
		},
	}
)
