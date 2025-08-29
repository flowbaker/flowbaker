package knowledge

import (
	"flowbaker/internal/domain"
)

const (
	IntegrationActionType_Search domain.IntegrationActionType = "search"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Knowledge,
		Name:        "Knowledge",
		Description: "Search and retrieve information from knowledge bases",
		Actions: []domain.IntegrationAction{
			{
				ID:         "search",
				Name:       "Search Knowledge",
				ActionType: IntegrationActionType_Search,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextTool,
				},

				Description: "Search for information in a knowledge base using semantic search",
				Properties: []domain.NodeProperty{
					{
						Key:          "knowledge_base_id",
						Name:         "Knowledge Base",
						Description:  "The knowledge base to search in",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: KnowledgeIntegrationPeekable_Knowledges,
					},
					{
						Key:         "query",
						Name:        "Search Query",
						Description: "The text to search for in the knowledge base",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
						Placeholder: "Enter your search query...",
						Help:        "Use natural language to describe what you're looking for",
					},
					{
						Key:         "limit",
						Name:        "Result Limit",
						Description: "Maximum number of results to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
						Advanced:    true,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     1,
							Max:     50,
							Default: 10,
							Step:    1,
						},
					},
					{
						Key:         "similarity_threshold",
						Name:        "Similarity Threshold",
						Description: "Minimum similarity score for results (0.0 to 1.0)",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						Advanced:    true,
						NumberOpts: &domain.NumberPropertyOptions{
							Min:     0.0,
							Max:     1.0,
							Default: 0.7,
							Step:    0.1,
						},
					},
				},
			},
		},
	}
)
