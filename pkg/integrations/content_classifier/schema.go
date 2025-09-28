package router

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_Classify domain.IntegrationActionType = "classify"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_ContentClassifier,
		Name:                 "Content Classifier",
		Description:          "Classify content into different categories based on AI classification.",
		CanTestConnection:    false,
		IsCredentialOptional: true,
		CredentialProperties: []domain.NodeProperty{},
		Triggers:             []domain.IntegrationTrigger{},
		Actions: []domain.IntegrationAction{
			{
				ID:          "classify",
				Name:        "Classify Content",
				ActionType:  IntegrationActionType_Classify,
				Description: "Route data to different paths based on AI classification.",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "content",
						Name:        "Content",
						Description: "The content to classify.",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:              "categories",
						Name:             "Categories",
						Description:      "The categories to classify the content into.",
						Required:         true,
						Type:             domain.NodePropertyType_Array,
						GeneratesHandles: true,
						HandleGenerationOpts: &domain.HandleGenerationOptions{
							HandleType:        "output",
							NameFromProperty:  "Name",
							DefaultHandleType: domain.NodeHandleTypeDefault,
							Position:          domain.NodeHandlePositionBottom,
						},
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "Name",
									Name:        "Name",
									Description: "The name of the category.",
									Type:        domain.NodePropertyType_String,
								},
								{
									Key:         "Description",
									Name:        "Description",
									Description: "The description of the category.",
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
		},
	}
)
