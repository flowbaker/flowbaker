package postgresql

import (
	"flowbaker/internal/domain"
)

var (
	Schema                    = schema
	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_PostgreSQL,
		Name:        "PostgreSQL",
		Description: "Use PostgreSQL integration to perform database operations like inserting, finding, updating, and deleting documents.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "uri",
				Name:        "PostgreSQL URI",
				Description: "The connection string URI for your PostgreSQL deployment",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "execute_query",
				Name:        "Execute Query",
				ActionType:  IntegrationActionType_ExecuteQuery,
				Description: "Execute a raw SQL query",
				Properties: []domain.NodeProperty{
					{
						Key:         "query",
						Name:        "Query",
						Description: "The SQL query to execute",
						Required:    true,
						Type:        domain.NodePropertyType_Query,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension:        domain.PropertySyntaxExtensionType_SQL,
							Dialect:          domain.PropertySyntaxDialectType_PostgreSQL,
							EnableParameters: true,
						},
					},
				},
			},
			{
				ID:          "store_memory",
				Name:        "Store Memory",
				ActionType:  "store_memory",
				Description: "Store data in PostgreSQL for AI agent memory",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextMemoryProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Memory Key",
						Description: "Unique identifier for the memory",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "value",
						Name:        "Memory Value",
						Description: "Data to store in memory",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "retrieve_memory",
				Name:        "Retrieve Memory",
				ActionType:  "retrieve_memory",
				Description: "Retrieve data from PostgreSQL memory",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextMemoryProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Memory Key",
						Description: "Unique identifier for the memory to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
	}
)
