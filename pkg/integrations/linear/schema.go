package linear

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	LinearIntegrationPeekable_ResourceTypes domain.IntegrationPeekableType = "linear_resource_types"
)

const (
	IntegrationTriggerEventType_OnNewLinearEvent domain.IntegrationTriggerEventType = "on_new_linear_event"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                domain.IntegrationType_Linear,
		Name:              "Linear",
		Description:       "Use Linear integration to create, read, update, and delete issues, and manage your team's workflow.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "linear_oauth",
				Name:        "Linear Account",
				Description: "The Linear account to use for the integration",
				Required:    false,
				Type:        domain.NodePropertyType_OAuth,
				OAuthType:   domain.OAuthTypeLinear,
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "on_new_linear_event",
				Name:        "On new Linear Event",
				EventType:   IntegrationTriggerEventType_OnNewLinearEvent,
				Description: "Triggers when a new event occurs in Linear (e.g., issue created, updated, commented).",
				Properties: []domain.NodeProperty{
					{
						Key:         "credential_id",
						Name:        "Linear Credential",
						Description: "Select the Linear API Key credential to use for this trigger.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:                    "team_id",
						Name:                   "Team",
						Description:            "Select the Linear team to listen for events.",
						Required:               true,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           LinearIntegrationPeekable_Teams,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,
					},
					{
						Key:              "resource_types",
						Name:             "Resource Types",
						Description:      "Select the types of Linear resources to listen for (e.g., Issue, Comment, Project).",
						Required:         true,
						Type:             domain.NodePropertyType_ListTagInput,
						ExpressionChoice: false,
						Options: []domain.NodePropertyOption{
							{
								Label:       "Issue",
								Value:       "Issue",
								Description: "Track and manage tasks, bugs, and features",
							},
							{
								Label:       "Comment",
								Value:       "Comment",
								Description: "Comments and discussions on issues",
							},
							{
								Label:       "Cycle",
								Value:       "Cycle",
								Description: "Time-boxed periods for completing work",
							},
							{
								Label:       "Issue Label",
								Value:       "IssueLabel",
								Description: "Categories and tags for organizing issues",
							},
							{
								Label:       "Reaction",
								Value:       "Reaction",
								Description: "Emoji reactions on issues and comments",
							},
							{
								Label:       "Project",
								Value:       "Project",
								Description: "Collections of issues organized by project",
							},
						},
					},
				},
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "create_issue",
				Name:        "Create Issue",
				ActionType:  IntegrationActionType_CreateIssue,
				Description: "Creates a new issue in Linear.",
				Properties: []domain.NodeProperty{
					{
						Key:         "title",
						Name:        "Issue Title",
						Description: "The title of the new Linear issue.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "A detailed description for the issue (supports Markdown).",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:              "team_id",
						Name:             "Team",
						Description:      "The team the issue belongs to.",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     LinearIntegrationPeekable_Teams,
						ExpressionChoice: true,
					},
					{
						Key:              "priority_id",
						Name:             "Priority",
						Description:      "The priority level of the issue (e.g., 0=No priority, 1=Urgent, 2=High, 3=Medium, 4=Low).",
						Required:         false,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     LinearIntegrationPeekable_Priorities,
						ExpressionChoice: true,
					},
					{
						Key:                    "assignee_id",
						Name:                   "Assignee",
						Description:            "The user to whom the issue will be assigned.",
						Required:               false,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           LinearIntegrationPeekable_Users,
						ExpressionChoice:       true,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,
					},
					{
						Key:         "label_ids",
						Name:        "Labels",
						Description: "One or more labels to attach to the issue.",
						Required:    false,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "label_id",
									Name:        "Label ID",
									Description: "The ID of the label",
									Type:        domain.NodePropertyType_String,
								},
							},
						},
						Peekable:               true,
						PeekableType:           LinearIntegrationPeekable_Labels,
						ExpressionChoice:       true,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,
					},
				},
			},
			{
				ID:          "delete_issue",
				Name:        "Delete Issue",
				ActionType:  IntegrationActionType_DeleteIssue,
				Description: "Deletes an existing issue in Linear.",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_id",
						Name:        "Issue ID",
						Description: "The ID of the Linear issue to delete.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_issue",
				Name:        "Get Issue",
				ActionType:  IntegrationActionType_GetIssue,
				Description: "Retrieves a single Linear issue by its ID.",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_id",
						Name:        "Issue ID",
						Description: "The ID of the Linear issue to retrieve.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_issues",
				Name:        "Get Many Issues",
				ActionType:  IntegrationActionType_GetManyIssues,
				Description: "Retrieves multiple Linear issues based on filters.",
				Properties: []domain.NodeProperty{
					{
						Key:              "team_id",
						Name:             "Filter by Team",
						Description:      "Optionally filter issues by the team they belong to.",
						Required:         false,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     LinearIntegrationPeekable_Teams,
						ExpressionChoice: true,
					},
					{
						Key:                    "assignee_id",
						Name:                   "Filter by Assignee",
						Description:            "Optionally filter issues by the assignee.",
						Required:               false,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           LinearIntegrationPeekable_Users,
						ExpressionChoice:       true,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,
					},
					{
						Key:         "label_ids",
						Name:        "Filter by Labels",
						Description: "Optionally filter issues by one or more labels.",
						Required:    false,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "label_id",
									Name:        "Label ID",
									Description: "The ID of the label",
									Type:        domain.NodePropertyType_String,
								},
							},
						},
						Peekable:               true,
						PeekableType:           LinearIntegrationPeekable_Labels,
						ExpressionChoice:       true,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,
					},
					{
						Key:         "query",
						Name:        "Search Query",
						Description: "Optionally filter issues by a text query in their title or description.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of issues to retrieve (default 50).",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "update_issue",
				Name:        "Update Issue",
				ActionType:  IntegrationActionType_UpdateIssue,
				Description: "Updates an existing Linear issue.",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_id",
						Name:        "Issue ID",
						Description: "The ID of the Linear issue to update.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "title",
						Name:        "New Title",
						Description: "The new title for the issue.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "New Description",
						Description: "The new description for the issue (supports Markdown).",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:              "team_id",
						Name:             "New Team",
						Description:      "Assign the issue to a different team.",
						Required:         false,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     LinearIntegrationPeekable_Teams,
						ExpressionChoice: true,
					},
					{
						Key:              "priority_id",
						Name:             "New Priority",
						Description:      "Change the priority level of the issue.",
						Required:         false,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     LinearIntegrationPeekable_Priorities,
						ExpressionChoice: true,
					},
					{
						Key:                    "assignee_id",
						Name:                   "New Assignee",
						Description:            "Reassign the issue to a different user.",
						Required:               false,
						Type:                   domain.NodePropertyType_String,
						Peekable:               true,
						PeekableType:           LinearIntegrationPeekable_Users,
						ExpressionChoice:       true,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,
					},
					{
						Key:         "label_ids",
						Name:        "New Labels",
						Description: "Replace existing labels with new ones, or add/remove (provide an empty array to remove all labels).",
						Required:    false,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "label_id",
									Name:        "Label ID",
									Description: "The ID of the label",
									Type:        domain.NodePropertyType_String,
								},
							},
						},
						Peekable:               true,
						PeekableType:           LinearIntegrationPeekable_Labels,
						ExpressionChoice:       true,
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,
					},
					{
						Key:          "state_id",
						Name:         "New Status (Workflow State)",
						Description:  "Change the workflow status of the issue (e.g., 'Todo', 'In Progress', 'Done').",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: LinearIntegrationPeekable_States,
						Dependent:    []string{"team_id"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "team_id",
								ValueKey:    "team_id",
							},
						},
						PeekablePaginationType: domain.PeekablePaginationType_Cursor,

						ExpressionChoice: true,
					},
				},
			},
		},
	}
)
