package filetoitem

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_ConvertFileToItem domain.IntegrationActionType = "convert_file_to_item"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_FileToItem,
		Name:                 "File To Item",
		Description:          "Convert JSON and NDJSON files to items",
		IsCredentialOptional: true,
		Actions: []domain.IntegrationAction{
			{
				ID:          string(IntegrationActionType_ConvertFileToItem),
				Name:        "Convert File to Item",
				ActionType:  IntegrationActionType_ConvertFileToItem,
				Description: "Converts a JSON or NDJSON file to an item. Automatically detects JSON/NDJSON content regardless of file extension or content-type.",
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
						Key:         "file",
						Name:        "File",
						Description: "The file to convert to an item. Supports JSON (objects/arrays) and NDJSON (newline-delimited JSON) formats.",
						Required:    true,
						Type:        domain.NodePropertyType_File,
						Placeholder: "Select a JSON or NDJSON file",
					},
				},
			},
		},
	}
)
