package gitlab

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

// Action types
const (
	IntegrationActionType_ListProjects       domain.IntegrationActionType = "list_projects"
	IntegrationActionType_GetProject         domain.IntegrationActionType = "get_project"
	IntegrationActionType_CreateIssue        domain.IntegrationActionType = "create_issue"
	IntegrationActionType_UpdateIssue        domain.IntegrationActionType = "update_issue"
	IntegrationActionType_ListIssues         domain.IntegrationActionType = "list_issues"
	IntegrationActionType_AddIssueComment    domain.IntegrationActionType = "add_issue_comment"
	IntegrationActionType_CreateMergeRequest domain.IntegrationActionType = "create_merge_request"
	IntegrationActionType_ListMergeRequests  domain.IntegrationActionType = "list_merge_requests"
	IntegrationActionType_UpdateMergeRequest domain.IntegrationActionType = "update_merge_request"
	IntegrationActionType_AddMRComment       domain.IntegrationActionType = "add_mr_comment"
	IntegrationActionType_GetFileContent     domain.IntegrationActionType = "get_file_content"
	IntegrationActionType_CreateFile         domain.IntegrationActionType = "create_file"
	IntegrationActionType_UpdateFile         domain.IntegrationActionType = "update_file"
	IntegrationActionType_TriggerPipeline    domain.IntegrationActionType = "trigger_pipeline"
	IntegrationActionType_GetPipelineStatus  domain.IntegrationActionType = "get_pipeline_status"
	IntegrationActionType_ListPipelines      domain.IntegrationActionType = "list_pipelines"
	IntegrationActionType_CreateBranch       domain.IntegrationActionType = "create_branch"
	IntegrationActionType_ListBranches       domain.IntegrationActionType = "list_branches"
)

// Peekable types
const (
	GitLabPeekable_Projects domain.IntegrationPeekableType = "projects"
	GitLabPeekable_Branches domain.IntegrationPeekableType = "branches"
	GitLabPeekable_Users    domain.IntegrationPeekableType = "users"
	GitLabPeekable_Labels   domain.IntegrationPeekableType = "labels"
)

var (
	GitLabSchema = domain.Integration{
		ID:          domain.IntegrationType_GitLab,
		Name:        "GitLab",
		Description: "Integrate with GitLab for project management, issue tracking, merge requests, and CI/CD automation.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "api_token",
				Name:        "API Token",
				Description: "Personal Access Token with appropriate scopes (api, read_user, read_repository)",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "base_url",
				Name:        "GitLab Base URL",
				Description: "Base URL for GitLab instance (default: https://gitlab.com)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
				Placeholder: "https://gitlab.com",
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "list_projects",
				Name:        "List Projects",
				Description: "List GitLab projects accessible to the authenticated user",
				ActionType:  IntegrationActionType_ListProjects,
				Properties: []domain.NodeProperty{
					{
						Key:         "owned",
						Name:        "Owned Only",
						Description: "List only projects owned by the authenticated user",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
					{
						Key:         "starred",
						Name:        "Starred Only",
						Description: "List only starred projects",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
					{
						Key:         "search",
						Name:        "Search Query",
						Description: "Search projects by name",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_project",
				Name:        "Get Project",
				Description: "Get detailed information about a specific project",
				ActionType:  IntegrationActionType_GetProject,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "create_issue",
				Name:        "Create Issue",
				Description: "Create a new issue in a GitLab project",
				ActionType:  IntegrationActionType_CreateIssue,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "title",
						Name:        "Issue Title",
						Description: "The title of the issue",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Issue Description",
						Description: "The description/body of the issue",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "assignee_ids",
						Name:        "Assignee IDs",
						Description: "Array of user IDs to assign the issue to",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "labels",
						Name:        "Labels",
						Description: "Comma-separated list of labels",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "milestone_id",
						Name:        "Milestone ID",
						Description: "The ID of the milestone to assign the issue to",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
					},
				},
			},
			{
				ID:          "update_issue",
				Name:        "Update Issue",
				Description: "Update an existing issue",
				ActionType:  IntegrationActionType_UpdateIssue,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "issue_iid",
						Name:        "Issue IID",
						Description: "The internal ID of the issue",
						Required:    true,
						Type:        domain.NodePropertyType_Number,
					},
					{
						Key:         "title",
						Name:        "Issue Title",
						Description: "The title of the issue",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Issue Description",
						Description: "The description/body of the issue",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "state_event",
						Name:        "State Event",
						Description: "Change the state of the issue (close, reopen)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Close", Value: "close"},
							{Label: "Reopen", Value: "reopen"},
						},
					},
					{
						Key:         "assignee_ids",
						Name:        "Assignee IDs",
						Description: "Array of user IDs to assign the issue to",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "labels",
						Name:        "Labels",
						Description: "Comma-separated list of labels",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
			},
			{
				ID:          "list_issues",
				Name:        "List Issues",
				Description: "List issues from a GitLab project",
				ActionType:  IntegrationActionType_ListIssues,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "state",
						Name:        "Issue State",
						Description: "Filter issues by state",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "All", Value: "all"},
							{Label: "Open", Value: "opened"},
							{Label: "Closed", Value: "closed"},
						},
						Placeholder: "all",
					},
					{
						Key:         "assignee_id",
						Name:        "Assignee ID",
						Description: "Filter by assignee user ID",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
					},
					{
						Key:         "author_id",
						Name:        "Author ID",
						Description: "Filter by author user ID",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
					},
					{
						Key:         "labels",
						Name:        "Labels",
						Description: "Comma-separated list of labels to filter by",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
			},
			{
				ID:          "add_issue_comment",
				Name:        "Add Issue Comment",
				Description: "Add a comment to an existing issue",
				ActionType:  IntegrationActionType_AddIssueComment,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "issue_iid",
						Name:        "Issue IID",
						Description: "The internal ID of the issue",
						Required:    true,
						Type:        domain.NodePropertyType_Number,
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
				ID:          "create_merge_request",
				Name:        "Create Merge Request",
				Description: "Create a new merge request",
				ActionType:  IntegrationActionType_CreateMergeRequest,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "title",
						Name:        "Title",
						Description: "The title of the merge request",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "source_branch",
						Name:        "Source Branch",
						Description: "The source branch for the merge request",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "target_branch",
						Name:        "Target Branch",
						Description: "The target branch for the merge request",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "The description of the merge request",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "assignee_ids",
						Name:        "Assignee IDs",
						Description: "Array of user IDs to assign the merge request to",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "reviewer_ids",
						Name:        "Reviewer IDs",
						Description: "Array of user IDs to request review from",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
			},
			{
				ID:          "list_merge_requests",
				Name:        "List Merge Requests",
				Description: "List merge requests from a GitLab project",
				ActionType:  IntegrationActionType_ListMergeRequests,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "state",
						Name:        "Merge Request State",
						Description: "Filter merge requests by state",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "All", Value: "all"},
							{Label: "Open", Value: "opened"},
							{Label: "Closed", Value: "closed"},
							{Label: "Merged", Value: "merged"},
						},
						Placeholder: "all",
					},
					{
						Key:         "source_branch",
						Name:        "Source Branch",
						Description: "Filter by source branch",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "target_branch",
						Name:        "Target Branch",
						Description: "Filter by target branch",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_file_content",
				Name:        "Get File Content",
				Description: "Get the content of a file from the repository",
				ActionType:  IntegrationActionType_GetFileContent,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "file_path",
						Name:        "File Path",
						Description: "The path to the file in the repository",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "ref",
						Name:        "Branch/Tag/Commit",
						Description: "The branch, tag, or commit SHA to get the file from",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Placeholder: "main",
					},
				},
			},
			{
				ID:          "create_file",
				Name:        "Create File",
				Description: "Create a new file in the repository",
				ActionType:  IntegrationActionType_CreateFile,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "file_path",
						Name:        "File Path",
						Description: "The path where the file should be created",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "content",
						Name:        "File Content",
						Description: "The content of the file",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
					},
					{
						Key:         "commit_message",
						Name:        "Commit Message",
						Description: "The commit message for this change",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "branch",
						Name:        "Branch",
						Description: "The branch to commit to",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Placeholder: "main",
					},
				},
			},
			{
				ID:          "trigger_pipeline",
				Name:        "Trigger Pipeline",
				Description: "Trigger a CI/CD pipeline for a project",
				ActionType:  IntegrationActionType_TriggerPipeline,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "ref",
						Name:        "Branch/Tag",
						Description: "The branch or tag to run the pipeline on",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "variables",
						Name:        "Pipeline Variables",
						Description: "JSON object of variables to pass to the pipeline",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
				},
			},
			{
				ID:          "list_branches",
				Name:        "List Branches",
				Description: "List branches from a GitLab project",
				ActionType:  IntegrationActionType_ListBranches,
				Properties: []domain.NodeProperty{
					{
						Key:         "project_id",
						Name:        "Project ID",
						Description: "The ID or path of the project",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "search",
						Name:        "Search Query",
						Description: "Search branches by name",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
		Triggers:             []domain.IntegrationTrigger{},
		CanTestConnection:    true,
		IsCredentialOptional: false,
	}
)