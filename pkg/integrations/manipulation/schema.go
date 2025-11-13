package manipulation

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_SetField          domain.IntegrationActionType = "set_field"
	IntegrationActionType_SetMultipleFields domain.IntegrationActionType = "set_multiple_fields"
	IntegrationActionType_DeleteField       domain.IntegrationActionType = "delete_field"
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
					{
						Key:         "field_type",
						Name:        "Field Type",
						Description: "The type to convert the value to. Auto will try to infer the type from the existing field value",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Auto", Value: "auto", Description: "Automatically detect type from existing field (default)"},
							{Label: "String", Value: "string", Description: "Text value"},
							{Label: "Number", Value: "number", Description: "Numeric value (integer or float)"},
							{Label: "Boolean", Value: "boolean", Description: "True or false value"},
							{Label: "Array", Value: "array", Description: "Array/list parsed from JSON"},
							{Label: "Object", Value: "object", Description: "Object/map parsed from JSON"},
						},
					},
				},
			},
			{
				ID:          string(IntegrationActionType_SetMultipleFields),
				Name:        "Set Multiple Fields",
				ActionType:  IntegrationActionType_SetMultipleFields,
				Description: "Set or update multiple field values at once. Supports nested paths and dynamic expressions for each field",
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
						Key:         "fields",
						Name:        "Fields",
						Description: "List of fields to set with their values and types",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 50,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "field_path",
									Name:        "Field Path",
									Description: "The path to the field to set. Supports nested paths using dot notation",
									Required:    true,
									Type:        domain.NodePropertyType_String,
									Placeholder: "field_name or nested.field.path",
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value to set. Supports dynamic expressions like {{item.field_name}}",
									Required:    true,
									Type:        domain.NodePropertyType_Text,
									Placeholder: "Enter value or {{item.field}}",
								},
								{
									Key:         "field_type",
									Name:        "Field Type",
									Description: "The type to convert the value to. Auto will try to infer from existing field",
									Required:    false,
									Type:        domain.NodePropertyType_String,
									Options: []domain.NodePropertyOption{
										{Label: "Auto", Value: "auto", Description: "Automatically detect type from existing field (default)"},
										{Label: "String", Value: "string", Description: "Text value"},
										{Label: "Number", Value: "number", Description: "Numeric value (integer or float)"},
										{Label: "Boolean", Value: "boolean", Description: "True or false value"},
										{Label: "Array", Value: "array", Description: "Array/list parsed from JSON"},
										{Label: "Object", Value: "object", Description: "Object/map parsed from JSON"},
									},
								},
							},
						},
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
