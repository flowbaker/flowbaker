package postgresql

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
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
				ID:          "use_memory",
				Name:        "Agent Conversation Memory",
				ActionType:  "use_memory",
				Description: "Use PostgreSQL to store and retrieve conversation history for AI agent",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextMemoryProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "table_prefix",
						Name:        "Table Prefix",
						Description: "The prefix for the tables (e.g., 'ai_memory' creates ai_memory_conversations and ai_memory_messages tables)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:              "session_id",
						Name:             "Session ID",
						Description:      "Unique identifier for the session",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						ExpressionChoice: true,
					},
				},
			},
		},
	}
)
