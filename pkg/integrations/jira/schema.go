package jira

import (
	"github.com/flowbaker/flowbaker/internal/domain"
)

// Jira universal trigger event type (following GitHub pattern)
const (
	IntegrationTriggerType_JiraUniversalTrigger domain.IntegrationTriggerEventType = "jira_universal_trigger"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                domain.IntegrationType_Jira,
		Name:              "Jira",
		Description:       "Use Jira integration to manage issues, get changelogs, create notifications, and more.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "jira_oauth",
				Name:        "Jira Account",
				Description: "The Jira account to use for the integration",
				Required:    true,
				Type:        domain.NodePropertyType_OAuth,
				OAuthType:   domain.OAuthTypeJira,
			},
			{
				Key:         "webhook_secret",
				Name:        "Webhook Secret",
				Description: "The secret to use for the webhook",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "get_issue",
				Name:        "Get Issue",
				ActionType:  IntegrationActionType_GetIssue,
				Description: "Get a single issue by its key or ID",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_issues",
				Name:        "Get Many Issues",
				ActionType:  IntegrationActionType_GetManyIssues,
				Description: "Get multiple issues using JQL query",
				Properties: []domain.NodeProperty{
					{
						Key:         "jql_query",
						Name:        "JQL Query",
						Description: "JQL query to filter issues (e.g., project = PROJ AND status = Open)",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "max_results",
						Name:        "Max Results",
						Description: "Maximum number of issues to return (default: 50, max: 100)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "create_issue",
				Name:        "Create Issue",
				ActionType:  IntegrationActionType_CreateIssue,
				Description: "Create a new issue",
				Properties: []domain.NodeProperty{
					{
						Key:              "project_key",
						Name:             "Project Key",
						Description:      "The project key where the issue will be created",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     JiraIntegrationPeekable_Projects,
						ExpressionChoice: true,
					},
					{
						Key:          "issue_type",
						Name:         "Issue Type",
						Description:  "The type of issue (e.g., Bug, Task, Story)",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: JiraIntegrationPeekable_IssueTypes,
						Dependent:    []string{"project_key"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "project_key",
								ValueKey:    "project_key",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:          "parent_key",
						Name:         "Parent Issue Key",
						Description:  "Parent issue key for creating subtasks (e.g., PROJ-123). Required when creating subtasks.",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: JiraIntegrationPeekable_Issues,
						Dependent:    []string{"project_key", "issue_type"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "project_key",
								ValueKey:    "project_key",
							},
						},
						ShowIf: &domain.ShowIf{
							PropertyKey: "issue_type",
							Values:      []any{"Sub-task", "Subtask", "subtask", "sub-task", "Sub task"},
						},
						ExpressionChoice: true,
					},
					{
						Key:         "summary",
						Name:        "Summary",
						Description: "Brief summary of the issue",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Detailed description of the issue",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:          "priority",
						Name:         "Priority",
						Description:  "Priority of the issue (e.g., High, Medium, Low)",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: JiraIntegrationPeekable_Priorities,
						Dependent:    []string{"project_key"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "project_key",
								ValueKey:    "project_key",
							},
						},
					},
					{
						Key:          "assignee",
						Name:         "Assignee",
						Description:  "Email or username of the assignee",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: JiraIntegrationPeekable_Assignees,
						Dependent:    []string{"project_key"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "project_key",
								ValueKey:    "project_key",
							},
						},
					},
				},
			},
			{
				ID:          "update_issue",
				Name:        "Update Issue",
				ActionType:  IntegrationActionType_UpdateIssue,
				Description: "Update an existing issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key to update (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "summary",
						Name:        "Summary",
						Description: "Updated summary of the issue",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Updated description of the issue",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:          "priority",
						Name:         "Priority",
						Description:  "Updated priority of the issue",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: JiraIntegrationPeekable_Priorities,
					},
					{
						Key:          "assignee",
						Name:         "Assignee",
						Description:  "Updated assignee email or username",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: JiraIntegrationPeekable_Assignees,
						Dependent:    []string{"issue_key"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "issue_key",
								ValueKey:    "issue_key",
							},
						},
					},
					{
						Key:          "status",
						Name:         "Status",
						Description:  "Updated status of the issue",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: JiraIntegrationPeekable_Statuses,
						Dependent:    []string{"issue_key"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "issue_key",
								ValueKey:    "issue_key",
							},
						},
					},
				},
			},
			{
				ID:          "delete_issue",
				Name:        "Delete Issue",
				ActionType:  IntegrationActionType_DeleteIssue,
				Description: "Delete an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key to delete (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_issue_changelog",
				Name:        "Get Issue Changelog",
				ActionType:  IntegrationActionType_GetIssueChangelog,
				Description: "Get the changelog of an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key to get changelog for (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_issue_status",
				Name:        "Get Issue Status",
				ActionType:  IntegrationActionType_GetIssueStatus,
				Description: "Get the current status of an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key to get status for (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "create_email_notification",
				Name:        "Create Email Notification",
				ActionType:  IntegrationActionType_CreateEmailNotification,
				Description: "Create an email notification for an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key to create notification for (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "recipients",
						Name:        "Recipients",
						Description: "Email addresses of notification recipients",
						Required:    true,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "subject",
						Name:        "Subject",
						Description: "Email subject line",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message",
						Name:        "Message",
						Description: "Email message content",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			// Comment actions
			{
				ID:          "add_comment",
				Name:        "Add Comment",
				ActionType:  IntegrationActionType_AddComment,
				Description: "Add a comment to an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key to add comment to (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Comment Body",
						Description: "The content of the comment",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "get_comment",
				Name:        "Get Comment",
				ActionType:  IntegrationActionType_GetComment,
				Description: "Get a specific comment from an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key containing the comment (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "comment_id",
						Name:        "Comment ID",
						Description: "The ID of the comment to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_comments",
				Name:        "Get Many Comments",
				ActionType:  IntegrationActionType_GetManyComments,
				Description: "Get multiple comments from an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key to get comments from (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "max_results",
						Name:        "Max Results",
						Description: "Maximum number of comments to return (default: 50, max: 100)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "update_comment",
				Name:        "Update Comment",
				ActionType:  IntegrationActionType_UpdateComment,
				Description: "Update an existing comment on an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key containing the comment (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "comment_id",
						Name:        "Comment ID",
						Description: "The ID of the comment to update",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Comment Body",
						Description: "The updated content of the comment",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "remove_comment",
				Name:        "Remove Comment",
				ActionType:  IntegrationActionType_RemoveComment,
				Description: "Remove a comment from an issue",
				Properties: []domain.NodeProperty{
					{
						Key:         "issue_key",
						Name:        "Issue Key",
						Description: "The issue key containing the comment (e.g., PROJ-123)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "comment_id",
						Name:        "Comment ID",
						Description: "The ID of the comment to remove",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			// User actions
			{
				ID:          "get_user",
				Name:        "Get User",
				ActionType:  IntegrationActionType_GetUser,
				Description: "Get user information from Jira",
				Properties: []domain.NodeProperty{
					{
						Key:         "account_id",
						Name:        "Account ID",
						Description: "The account ID of the user (preferred method)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "username",
						Name:        "Username",
						Description: "The username of the user (legacy method)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "email_address",
						Name:        "Email Address",
						Description: "The email address of the user (search method)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "jira_event_listener",
				Name:        "Jira Event Listener",
				EventType:   IntegrationTriggerType_JiraUniversalTrigger,
				Description: "Triggers on selected Jira events for a project or organization.",
				Properties: []domain.NodeProperty{
					{
						Key:              "project_key",
						Name:             "Project Key (Optional)",
						Description:      "The project key to monitor (e.g., 'PROJ'). If empty, listens to organization/instance level events if applicable for selected event types.",
						Required:         false,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     JiraIntegrationPeekable_Projects,
						ExpressionChoice: true,
					},
					{
						Key:         "path",
						Name:        "Webhook Url",
						Description: "The project key to monitor (e.g., 'PROJ'). If empty, listens to organization/instance level events if applicable for selected event types.",
						Required:    true,
						Type:        domain.NodePropertyType_Endpoint,
					},
					{
						Key:         "selected_events",
						Name:        "Jira Events",
						Description: "Select one or more Jira events to trigger this flow. The webhook URL for these events will be generated by FlowBaker.",
						Required:    true,
						Type:        domain.NodePropertyType_ListTagInput,
						Options: []domain.NodePropertyOption{
							// Issue Events
							{Label: "On Issue Created", Value: "jira:issue_created", Description: "Triggered when a new issue is created"},
							{Label: "On Issue Updated", Value: "jira:issue_updated", Description: "Triggered when an issue is updated (fields changed, status transitions, etc.)"},
							{Label: "On Issue Deleted", Value: "jira:issue_deleted", Description: "Triggered when an issue is deleted"},

							// Comment Events
							{Label: "On Comment Created", Value: "comment_created", Description: "Triggered when a new comment is added to an issue"},
							{Label: "On Comment Updated", Value: "comment_updated", Description: "Triggered when an existing comment is modified"},
							{Label: "On Comment Deleted", Value: "comment_deleted", Description: "Triggered when a comment is removed from an issue"},

							// Board Events
							{Label: "On Board Created", Value: "board_created", Description: "Triggered when a new board is created"},
							{Label: "On Board Updated", Value: "board_updated", Description: "Triggered when board settings are modified"},
							{Label: "On Board Deleted", Value: "board_deleted", Description: "Triggered when a board is deleted"},
							{Label: "On Board Configuration Changed", Value: "board_configuration_changed", Description: "Triggered when board configuration is modified"},

							// Project Events
							{Label: "On Project Created", Value: "project_created", Description: "Triggered when a new project is created"},
							{Label: "On Project Updated", Value: "project_updated", Description: "Triggered when project settings are modified"},
							{Label: "On Project Deleted", Value: "project_deleted", Description: "Triggered when a project is deleted"},

							// Sprint Events (Agile)
							{Label: "On Sprint Created", Value: "sprint_created", Description: "Triggered when a new sprint is created"},
							{Label: "On Sprint Updated", Value: "sprint_updated", Description: "Triggered when sprint details are modified"},
							{Label: "On Sprint Started", Value: "sprint_started", Description: "Triggered when a sprint is started"},
							{Label: "On Sprint Closed", Value: "sprint_closed", Description: "Triggered when a sprint is completed/closed"},
							{Label: "On Sprint Deleted", Value: "sprint_deleted", Description: "Triggered when a sprint is deleted"},

							// User Events
							{Label: "On User Created", Value: "user_created", Description: "Triggered when a new user is created"},
							{Label: "On User Updated", Value: "user_updated", Description: "Triggered when user details are modified"},
							{Label: "On User Deleted", Value: "user_deleted", Description: "Triggered when a user is deleted"},

							// Version Events
							{Label: "On Version Created", Value: "version_created", Description: "Triggered when a new version/release is created"},
							{Label: "On Version Updated", Value: "version_updated", Description: "Triggered when version details are modified"},
							{Label: "On Version Deleted", Value: "version_deleted", Description: "Triggered when a version is deleted"},
							{Label: "On Version Released", Value: "version_released", Description: "Triggered when a version is marked as released"},
							{Label: "On Version Unreleased", Value: "version_unreleased", Description: "Triggered when a version release is reverted"},
							{Label: "On Version Moved", Value: "version_moved", Description: "Triggered when a version is moved to a different project"},

							// Worklog Events
							{Label: "On Worklog Created", Value: "worklog_created", Description: "Triggered when time is logged on an issue"},
							{Label: "On Worklog Updated", Value: "worklog_updated", Description: "Triggered when existing worklog is modified"},
							{Label: "On Worklog Deleted", Value: "worklog_deleted", Description: "Triggered when worklog is removed from an issue"},

							// Issue Link Events
							{Label: "On Issue Link Created", Value: "issuelink_created", Description: "Triggered when a link between issues is created"},
							{Label: "On Issue Link Deleted", Value: "issuelink_deleted", Description: "Triggered when a link between issues is removed"},

							// Option Events (Project Configuration)
							{Label: "On Attachments Option Changed", Value: "option_attachments_changed", Description: "Triggered when project attachment settings are modified"},
							{Label: "On Issue Links Option Changed", Value: "option_issuelinks_changed", Description: "Triggered when project issue linking settings are modified"},
							{Label: "On Subtasks Option Changed", Value: "option_subtasks_changed", Description: "Triggered when project subtask settings are modified"},
							{Label: "On Time Tracking Option Changed", Value: "option_timetracking_changed", Description: "Triggered when project time tracking settings are modified"},
							{Label: "On Unassigned Issues Option Changed", Value: "option_unassigned_issues_changed", Description: "Triggered when project unassigned issue settings are modified"},
							{Label: "On Voting Option Changed", Value: "option_voting_changed", Description: "Triggered when project voting settings are modified"},
							{Label: "On Watching Option Changed", Value: "option_watching_changed", Description: "Triggered when project watching settings are modified"},
						},
					},
				},
			},
		},
	}
)
