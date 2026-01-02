package notionintegration

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	IntegrationEventType_PageCreated         domain.IntegrationTriggerEventType = "page.created"
	IntegrationEventType_PagePropertyUpdated domain.IntegrationTriggerEventType = "page.properties_updated"
	IntegrationEventType_PageContentUpdated  domain.IntegrationTriggerEventType = "page.content_updated"

	IntegrationEventType_NotionUniversalTrigger domain.IntegrationTriggerEventType = "notion_universal_trigger"
)

var (
	NotionSchema = domain.Integration{
		ID:                domain.IntegrationType_Notion,
		Name:              "Notion",
		Description:       "Manage Notion pages, databases, and blocks.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "notion_oauth",
				Name:        "Notion Account",
				Description: "The Notion account to use for the integration",
				Required:    false,
				Type:        domain.NodePropertyType_OAuth,
				OAuthType:   domain.OAuthTypeNotion,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "append_block",
				Name:        "Append a Block",
				Description: "Append child blocks to a parent block",
				ActionType:  NotionActionType_AppendBlock,
				Properties: []domain.NodeProperty{
					{
						Key:         "block_id",
						Name:        "Block ID",
						Description: "The ID of the parent block to append children to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "children",
						Name:        "Children Blocks",
						Description: "Array of block objects to append. Must be valid Notion block JSON.",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
					},
				},
			},
			{
				ID:          "get_many_child_blocks",
				Name:        "Get Many Child Blocks",
				Description: "Retrieve a list of child blocks from a parent block",
				ActionType:  NotionActionType_GetManyChildBlocks,
				Properties: []domain.NodeProperty{
					{
						Key:         "block_id",
						Name:        "Block ID",
						Description: "The ID of the parent block",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "start_cursor",
						Name:        "Start Cursor",
						Description: "Pagination cursor for the next page of results",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "page_size",
						Name:        "Page Size",
						Description: "Number of items to return (max 100). Default: 100",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "get_database",
				Name:        "Get a Database",
				Description: "Retrieve a database object by its ID",
				ActionType:  NotionActionType_GetDatabase,
				Properties: []domain.NodeProperty{
					{
						Key:          "database_id",
						Name:         "Database ID",
						Description:  "The ID of the database to retrieve",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: NotionPeekable_Databases,
					},
				},
			},
			{
				ID:          "get_many_databases",
				Name:        "Get Many Databases",
				Description: "Search and retrieve multiple databases",
				ActionType:  NotionActionType_GetManyDatabases,
				Properties: []domain.NodeProperty{
					{
						Key:         "query",
						Name:        "Search Query",
						Description: "Text to search for in database titles",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "start_cursor",
						Name:        "Start Cursor",
						Description: "Pagination cursor for the next page of results",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "page_size",
						Name:        "Page Size",
						Description: "Number of items to return (max 100). Default: 100",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "search_database",
				Name:        "Search a Database",
				Description: "Query a database with filters and sorts",
				ActionType:  NotionActionType_SearchDatabase,
				Properties: []domain.NodeProperty{
					{
						Key:          "database_id",
						Name:         "Database ID",
						Description:  "The ID of the database to query",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: NotionPeekable_Databases,
					},
					{
						Key:         "filter",
						Name:        "Filter",
						Description: "Filter object for database query. Must be valid Notion filter JSON.",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "sorts",
						Name:        "Sorts",
						Description: "Array of sort objects. Must be valid Notion sort JSON.",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "start_cursor",
						Name:        "Start Cursor",
						Description: "Pagination cursor for the next page of results",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "page_size",
						Name:        "Page Size",
						Description: "Number of items to return (max 100). Default: 100",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "create_database_page",
				Name:        "Create a Database Page",
				Description: "Create a new page in a database",
				ActionType:  NotionActionType_CreateDatabasePage,
				Properties: []domain.NodeProperty{
					{
						Key:          "database_id",
						Name:         "Database ID",
						Description:  "The ID of the database to create the page in",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: NotionPeekable_Databases,
					},
					{
						Key:         "properties",
						Name:        "Properties",
						Description: "Page properties object. Must be valid Notion properties JSON.",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "children",
						Name:        "Children Blocks",
						Description: "Array of block objects to add as page content. Must be valid Notion block JSON.",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
				},
			},
			{
				ID:          "get_database_page",
				Name:        "Get a Database Page",
				Description: "Retrieve a page from a database by its ID",
				ActionType:  NotionActionType_GetDatabasePage,
				Properties: []domain.NodeProperty{
					{
						Key:         "page_id",
						Name:        "Page ID",
						Description: "The ID of the page to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_database_pages",
				Name:        "Get Many Database Pages",
				Description: "Query and retrieve multiple pages from a database",
				ActionType:  NotionActionType_GetManyDatabasePages,
				Properties: []domain.NodeProperty{
					{
						Key:          "database_id",
						Name:         "Database ID",
						Description:  "The ID of the database to query",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: NotionPeekable_Databases,
					},
					{
						Key:         "filter",
						Name:        "Filter",
						Description: "Filter object for database query. Must be valid Notion filter JSON.",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "sorts",
						Name:        "Sorts",
						Description: "Array of sort objects. Must be valid Notion sort JSON.",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "start_cursor",
						Name:        "Start Cursor",
						Description: "Pagination cursor for the next page of results",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "page_size",
						Name:        "Page Size",
						Description: "Number of items to return (max 100). Default: 100",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "update_database_page",
				Name:        "Update a Database Page",
				Description: "Update the properties of a database page",
				ActionType:  NotionActionType_UpdateDatabasePage,
				Properties: []domain.NodeProperty{
					{
						Key:         "page_id",
						Name:        "Page ID",
						Description: "The ID of the page to update",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "properties",
						Name:        "Properties",
						Description: "Page properties object to update. Must be valid Notion properties JSON.",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "archived",
						Name:        "Archived",
						Description: "Set to true to archive the page, false to unarchive",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
			{
				ID:          "archive_page",
				Name:        "Archive a Page",
				Description: "Archive a page by setting its archived property to true",
				ActionType:  NotionActionType_ArchivePage,
				Properties: []domain.NodeProperty{
					{
						Key:         "page_id",
						Name:        "Page ID",
						Description: "The ID of the page to archive",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "create_page",
				Name:        "Create a Page",
				Description: "Create a new page in Notion",
				ActionType:  NotionActionType_CreatePage,
				Properties: []domain.NodeProperty{
					{
						Key:         "parent_page_id",
						Name:        "Parent Page ID",
						Description: "The ID of the parent page (use this OR parent_database_id)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:          "parent_database_id",
						Name:         "Parent Database ID",
						Description:  "The ID of the parent database (use this OR parent_page_id)",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: NotionPeekable_Databases,
					},
					{
						Key:         "properties",
						Name:        "Properties",
						Description: "Page properties object. Must be valid Notion properties JSON.",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "children",
						Name:        "Children Blocks",
						Description: "Array of block objects to add as page content. Must be valid Notion block JSON.",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
				},
			},
			{
				ID:          "search_page",
				Name:        "Search a Page",
				Description: "Search for pages in Notion workspace",
				ActionType:  NotionActionType_SearchPage,
				Properties: []domain.NodeProperty{
					{
						Key:         "query",
						Name:        "Search Query",
						Description: "Text to search for in page titles and content",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "start_cursor",
						Name:        "Start Cursor",
						Description: "Pagination cursor for the next page of results",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "page_size",
						Name:        "Page Size",
						Description: "Number of items to return (max 100). Default: 100",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "get_user",
				Name:        "Get a User",
				Description: "Retrieve a user object by its ID",
				ActionType:  NotionActionType_GetUser,
				Properties: []domain.NodeProperty{
					{
						Key:         "user_id",
						Name:        "User ID",
						Description: "The ID of the user to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_users",
				Name:        "Get Many Users",
				Description: "Retrieve a list of all users in the workspace",
				ActionType:  NotionActionType_GetManyUsers,
				Properties: []domain.NodeProperty{
					{
						Key:         "start_cursor",
						Name:        "Start Cursor",
						Description: "Pagination cursor for the next page of results",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "page_size",
						Name:        "Page Size",
						Description: "Number of items to return (max 100). Default: 100",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "notion_event_listener",
				Name:        "Notion Event Listener",
				Description: "Triggers on selected Notion events",
				EventType:   IntegrationEventType_NotionUniversalTrigger,
				Properties: []domain.NodeProperty{
					{
						Key:         "verification_token",
						Name:        "Verification Token (Recommended)",
						Description: "Enter the verification token from your Notion integration settings. Strongly recommended for production to verify webhook authenticity.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:          "database_id",
						Name:         "Database ID (Optional)",
						Description:  "The database to monitor for changes. Leave empty to monitor all accessible resources.",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: NotionPeekable_Databases,
					},
					{
						Key:         "selected_events",
						Name:        "Notion Events",
						Description: "Select one or more Notion events to trigger this flow",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 0,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:      "event",
									Name:     "Event",
									Type:     domain.NodePropertyType_String,
									Required: true,
									Options: []domain.NodePropertyOption{
										{
											Label:       "On Page Created",
											Value:       string(IntegrationEventType_PageCreated),
											Description: "Triggered when a new page is created in the database or workspace",
										},
										{
											Label:       "On Page Property Updated",
											Value:       string(IntegrationEventType_PagePropertyUpdated),
											Description: "Triggered when a page's properties are updated (title, status, etc.)",
										},
										{
											Label:       "On Page Content Updated",
											Value:       string(IntegrationEventType_PageContentUpdated),
											Description: "Triggered when a page's content blocks are updated",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
)
