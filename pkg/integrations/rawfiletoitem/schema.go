package rawfiletoitem

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_ConvertRawFileToItem domain.IntegrationActionType = "convert_raw_file_to_item"
)

var (
	Schema = schema

	formatOptions = []domain.NodePropertyOption{
		{
			Label:       "Auto Detect",
			Value:       "auto",
			Description: "Automatically detect file format based on extension, content-type, or content",
		},
		{
			Label:       "JSON",
			Value:       "json",
			Description: "JSON file (single object or array)",
		},
		{
			Label:       "NDJSON",
			Value:       "ndjson",
			Description: "Newline-Delimited JSON (.ndjson, .jsonl)",
		},
		{
			Label:       "CSV",
			Value:       "csv",
			Description: "Comma-Separated Values",
		},
		{
			Label:       "TSV",
			Value:       "tsv",
			Description: "Tab-Separated Values",
		},
		{
			Label:       "Excel (XLSX)",
			Value:       "xlsx",
			Description: "Microsoft Excel spreadsheet (.xlsx, .xls)",
		},
		{
			Label:       "XML",
			Value:       "xml",
			Description: "XML file",
		},
		{
			Label:       "YAML",
			Value:       "yaml",
			Description: "YAML file (.yaml, .yml)",
		},
	}

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_RawFileToItem,
		Name:                 "Raw File To Item",
		Description:          "Convert raw files to items. Supports JSON, NDJSON, CSV, TSV, Excel (XLSX), XML, and YAML formats.",
		IsCredentialOptional: true,
		Actions: []domain.IntegrationAction{
			{
				ID:          string(IntegrationActionType_ConvertRawFileToItem),
				Name:        "Convert Raw File to Item",
				ActionType:  IntegrationActionType_ConvertRawFileToItem,
				Description: "Converts a raw file to items. Supports JSON, NDJSON, CSV, TSV, Excel (XLSX), XML, and YAML formats. Can auto-detect format or use a specified format.",
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
						Description: "The file to convert to items",
						Required:    true,
						Type:        domain.NodePropertyType_File,
						Placeholder: "Select a file",
					},
					{
						Key:         "format",
						Name:        "File Format",
						Description: "Select the file format or use auto-detect",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options:     formatOptions,
					},
				},
			},
		},
	}
)
