package manipulation

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_SetField    domain.IntegrationActionType = "set_field"
	IntegrationActionType_DeleteField domain.IntegrationActionType = "delete_field"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Manipulation,
		Name:                 "Manipulation",
		Description:          "Set or delete fields in items with support for nested paths and dynamic expressions",
		IsCredentialOptional: true,
		Actions: []domain.IntegrationAction{
			{
				ID:          string(IntegrationActionType_SetField),
				Name:        "Set Field",
				ActionType:  IntegrationActionType_SetField,
				Description: "Set or update a field value in the item. Supports nested paths (e.g., 'user.profile.name') and dynamic expressions (e.g., '{{item.original_field}}')",
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
						Key:         "field_path",
						Name:        "Field Path",
						Description: "The path to the field to set. Supports nested paths using dot notation (e.g., 'user.profile.name' or just 'name' for top-level fields)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Placeholder: "field_name or nested.field.path",
					},
					{
						Key:         "value",
						Name:        "Value",
						Description: "The value to set. Supports dynamic expressions like {{item.field_name}} to reference other fields",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Placeholder: "Enter value or {{item.field}}",
					},
				},
			},
			{
				ID:          string(IntegrationActionType_DeleteField),
				Name:        "Delete Field",
				ActionType:  IntegrationActionType_DeleteField,
				Description: "Remove a field from the item. Supports nested paths (e.g., 'user.profile.name')",
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
						Key:         "field_path",
						Name:        "Field Path",
						Description: "The path to the field to delete. Supports nested paths using dot notation (e.g., 'user.profile.name' or just 'name' for top-level fields)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Placeholder: "field_name or nested.field.path",
					},
				},
			},
		},
	}
)
