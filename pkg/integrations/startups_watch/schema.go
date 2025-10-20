package startupswatchintegration

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	// 12 Temel Endpoint - 12 Main Actions
	StartupsWatchActionType_GetStartup   domain.IntegrationActionType = "get_startup"
	StartupsWatchActionType_ListStartups domain.IntegrationActionType = "list_startups"
	// StartupsWatchActionType_SearchStartups   domain.IntegrationActionType = "search_startups"
	// StartupsWatchActionType_GetPerson        domain.IntegrationActionType = "get_person"
	// StartupsWatchActionType_ListPeople       domain.IntegrationActionType = "list_people"
	// StartupsWatchActionType_SearchPeople     domain.IntegrationActionType = "search_people"
	// StartupsWatchActionType_GetInvestor      domain.IntegrationActionType = "get_investor"
	// StartupsWatchActionType_ListInvestors    domain.IntegrationActionType = "list_investors"
	// StartupsWatchActionType_SearchInvestors  domain.IntegrationActionType = "search_investors"
	// StartupsWatchActionType_ListInvestments  domain.IntegrationActionType = "list_investments"
	// StartupsWatchActionType_ListAcquisitions domain.IntegrationActionType = "list_acquisitions"
	// StartupsWatchActionType_ListEvents       domain.IntegrationActionType = "list_events"
)

var (
	StartupsWatchSchema = domain.Integration{
		ID:                   domain.IntegrationType_StartupsWatch,
		Name:                 "Startups.watch",
		Description:          "Access comprehensive startup, investor, and ecosystem data from Startups.watch",
		CanTestConnection:    true,
		IsCredentialOptional: false,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "token",
				Name:        "Token",
				Description: "Your Startups.watch token",
				Required:    true,
				Type:        domain.NodePropertyType_String,
				IsSecret:    true,
			},
		},
		Actions: []domain.IntegrationAction{
			// 1. Get Startup
			{
				ID:          "get_startup",
				Name:        "Get Startup",
				Description: "Get detailed information about a specific startup",
				ActionType:  StartupsWatchActionType_GetStartup,
				Properties: []domain.NodeProperty{
					{
						Key:         "startup_id",
						Name:        "Startup ID",
						Description: "The unique identifier for the startup",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
				// look what it is
				SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			},
			{
				ID:          "list_startups",
				Name:        "List Startups",
				Description: "Get a paginated list of startups",
				ActionType:  StartupsWatchActionType_ListStartups,
				Properties: []domain.NodeProperty{
					{
						Key:         "page",
						Name:        "Page",
						Description: "Page number (default: 1)",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Number of results per page (max: 100, default: 20)",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
				SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			},
			// // 3. Search Startups
			// {
			// 	ID:          "search_startups",
			// 	Name:        "Search Startups",
			// 	Description: "Search for startups by name, description, or keywords",
			// 	ActionType:  StartupsWatchActionType_SearchStartups,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "query",
			// 			Name:        "Search Query",
			// 			Description: "Search terms to find startups",
			// 			Required:    true,
			// 			Type:        domain.NodePropertyType_String,
			// 		},
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 4. Get Person
			// {
			// 	ID:          "get_person",
			// 	Name:        "Get Person",
			// 	Description: "Get detailed information about a specific person",
			// 	ActionType:  StartupsWatchActionType_GetPerson,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "person_id",
			// 			Name:        "Person ID",
			// 			Description: "The unique identifier for the person",
			// 			Required:    true,
			// 			Type:        domain.NodePropertyType_String,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 5. List People
			// {
			// 	ID:          "list_people",
			// 	Name:        "List People",
			// 	Description: "Get a paginated list of people in the ecosystem",
			// 	ActionType:  StartupsWatchActionType_ListPeople,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 6. Search People
			// {
			// 	ID:          "search_people",
			// 	Name:        "Search People",
			// 	Description: "Search for people by name, title, or company",
			// 	ActionType:  StartupsWatchActionType_SearchPeople,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "query",
			// 			Name:        "Search Query",
			// 			Description: "Search terms to find people",
			// 			Required:    true,
			// 			Type:        domain.NodePropertyType_String,
			// 		},
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 7. Get Investor
			// {
			// 	ID:          "get_investor",
			// 	Name:        "Get Investor",
			// 	Description: "Get detailed information about a specific investor",
			// 	ActionType:  StartupsWatchActionType_GetInvestor,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "investor_id",
			// 			Name:        "Investor ID",
			// 			Description: "The unique identifier for the investor",
			// 			Required:    true,
			// 			Type:        domain.NodePropertyType_String,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 8. List Investors
			// {
			// 	ID:          "list_investors",
			// 	Name:        "List Investors",
			// 	Description: "Get a paginated list of investors",
			// 	ActionType:  StartupsWatchActionType_ListInvestors,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 9. Search Investors
			// {
			// 	ID:          "search_investors",
			// 	Name:        "Search Investors",
			// 	Description: "Search for investors by name or criteria",
			// 	ActionType:  StartupsWatchActionType_SearchInvestors,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "query",
			// 			Name:        "Search Query",
			// 			Description: "Search terms to find investors",
			// 			Required:    true,
			// 			Type:        domain.NodePropertyType_String,
			// 		},
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 10. List Investments
			// {
			// 	ID:          "list_investments",
			// 	Name:        "List Investments",
			// 	Description: "Get a paginated list of investments",
			// 	ActionType:  StartupsWatchActionType_ListInvestments,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 11. List Acquisitions
			// {
			// 	ID:          "list_acquisitions",
			// 	Name:        "List Acquisitions",
			// 	Description: "Get a paginated list of acquisitions",
			// 	ActionType:  StartupsWatchActionType_ListAcquisitions,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
			// // 12. List Events
			// {
			// 	ID:          "list_events",
			// 	Name:        "List Events",
			// 	Description: "Get a paginated list of events",
			// 	ActionType:  StartupsWatchActionType_ListEvents,
			// 	Properties: []domain.NodeProperty{
			// 		{
			// 			Key:         "page",
			// 			Name:        "Page",
			// 			Description: "Page number (default: 1)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 		{
			// 			Key:         "limit",
			// 			Name:        "Limit",
			// 			Description: "Number of results per page (max: 100, default: 20)",
			// 			Required:    false,
			// 			Type:        domain.NodePropertyType_Integer,
			// 		},
			// 	},
			// 	SupportedContexts: []domain.ActionUsageContext{domain.UsageContextWorkflow, domain.UsageContextTool},
			// 	HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
			// 		domain.UsageContextWorkflow: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 		domain.UsageContextTool: {
			// 			Output: []domain.NodeHandle{{Type: domain.NodeHandleTypeDefault}},
			// 		},
			// 	},
			// },
		},
	}
)
