package brightdata

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_ScrapData domain.IntegrationActionType = "scrap_data"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_BrightData,
		Name:        "BrightData",
		Description: "Scrape data using BrightData's async scraping API",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "api_token",
				Name:        "API Token",
				Description: "BrightData API Bearer Token",
				Required:    true,
				Type:        domain.NodePropertyType_String,
				Placeholder: "Enter your BrightData API token",
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          string(IntegrationActionType_ScrapData),
				Name:        "Scrap Data",
				ActionType:  IntegrationActionType_ScrapData,
				Description: "Trigger an async scraping job and wait for results. Returns scraped data as a file.",
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
						Key:         "dataset_id",
						Name:        "Dataset ID",
						Description: "The BrightData dataset ID to scrape",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Placeholder: "gd_l1viktl72bvl7bjuj0",
					},
					{
						Key:         "body_type",
						Name:        "Body Type",
						Description: "The type of request body",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "JSON", Value: "json", Description: "JSON body"},
							{Label: "Text", Value: "text", Description: "Plain text body"},
							{Label: "None", Value: "none", Description: "No body"},
						},
					},
					{
						Key:         "json_body",
						Name:        "JSON Body",
						Description: "JSON body for the scraping request (when body_type is json)",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
						Placeholder: "[{\"url\": \"https://example.com\"}]",
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "json",
						},
					},
					{
						Key:         "text_body",
						Name:        "Text Body",
						Description: "Plain text body for the scraping request (when body_type is text)",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
						Placeholder: "Enter text body",
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "text",
						},
					},
					{
						Key:         "polling_timeout_seconds",
						Name:        "Polling Timeout (seconds)",
						Description: "Maximum time to wait for scraping to complete (default: 60 seconds)",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
						Placeholder: "60",
					},
				},
			},
		},
	}
)
