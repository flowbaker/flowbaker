package githubintegration

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	IntegrationEventType_CheckRun                     domain.IntegrationTriggerEventType = "check_run"
	IntegrationEventType_CheckSuite                   domain.IntegrationTriggerEventType = "check_suite"
	IntegrationEventType_CommitComment                domain.IntegrationTriggerEventType = "commit_comment"
	IntegrationEventType_Create                       domain.IntegrationTriggerEventType = "create"
	IntegrationEventType_Delete                       domain.IntegrationTriggerEventType = "delete"
	IntegrationEventType_DeployKey                    domain.IntegrationTriggerEventType = "deploy_key"
	IntegrationEventType_Deployment                   domain.IntegrationTriggerEventType = "deployment"
	IntegrationEventType_DeploymentStatus             domain.IntegrationTriggerEventType = "deployment_status"
	IntegrationEventType_Fork                         domain.IntegrationTriggerEventType = "fork"
	IntegrationEventType_AppAuthorization             domain.IntegrationTriggerEventType = "github_app_authorization" // Matches GitHub's name
	IntegrationEventType_Gollum                       domain.IntegrationTriggerEventType = "gollum"
	IntegrationEventType_Installation                 domain.IntegrationTriggerEventType = "installation"
	IntegrationEventType_InstallationRepositories     domain.IntegrationTriggerEventType = "installation_repositories"
	IntegrationEventType_IssueComment                 domain.IntegrationTriggerEventType = "issue_comment"
	IntegrationEventType_Issues                       domain.IntegrationTriggerEventType = "issues"
	IntegrationEventType_Label                        domain.IntegrationTriggerEventType = "label"
	IntegrationEventType_MarketplacePurchase          domain.IntegrationTriggerEventType = "marketplace_purchase"
	IntegrationEventType_Member                       domain.IntegrationTriggerEventType = "member"
	IntegrationEventType_Membership                   domain.IntegrationTriggerEventType = "membership"
	IntegrationEventType_Meta                         domain.IntegrationTriggerEventType = "meta"
	IntegrationEventType_Milestone                    domain.IntegrationTriggerEventType = "milestone"
	IntegrationEventType_OrgBlock                     domain.IntegrationTriggerEventType = "org_block"
	IntegrationEventType_Organization                 domain.IntegrationTriggerEventType = "organization"
	IntegrationEventType_PageBuild                    domain.IntegrationTriggerEventType = "page_build"
	IntegrationEventType_Project                      domain.IntegrationTriggerEventType = "project"
	IntegrationEventType_ProjectCard                  domain.IntegrationTriggerEventType = "project_card"
	IntegrationEventType_ProjectColumn                domain.IntegrationTriggerEventType = "project_column"
	IntegrationEventType_Public                       domain.IntegrationTriggerEventType = "public"
	IntegrationEventType_PullRequest                  domain.IntegrationTriggerEventType = "pull_request"
	IntegrationEventType_PullRequestReview            domain.IntegrationTriggerEventType = "pull_request_review"
	IntegrationEventType_PullRequestReviewComment     domain.IntegrationTriggerEventType = "pull_request_review_comment"
	IntegrationEventType_Push                         domain.IntegrationTriggerEventType = "push"
	IntegrationEventType_Release                      domain.IntegrationTriggerEventType = "release"
	IntegrationEventType_Repository                   domain.IntegrationTriggerEventType = "repository"
	IntegrationEventType_RepositoryImport             domain.IntegrationTriggerEventType = "repository_import"
	IntegrationEventType_RepositoryVulnerabilityAlert domain.IntegrationTriggerEventType = "repository_vulnerability_alert"
	IntegrationEventType_SecurityAdvisory             domain.IntegrationTriggerEventType = "security_advisory"
	IntegrationEventType_Star                         domain.IntegrationTriggerEventType = "star"
	IntegrationEventType_Status                       domain.IntegrationTriggerEventType = "status"
	IntegrationEventType_Team                         domain.IntegrationTriggerEventType = "team"
	IntegrationEventType_TeamAdd                      domain.IntegrationTriggerEventType = "team_add"
	IntegrationEventType_Watch                        domain.IntegrationTriggerEventType = "watch"

	// Consolidated trigger type (remains as is, specific to FlowBaker)
	IntegrationEventType_GithubUniversalTrigger domain.IntegrationTriggerEventType = "github_universal_trigger"
)

var (
	GithubSchema = domain.Integration{
		ID:                domain.IntegrationType_Github,
		Name:              "GitHub",
		Description:       "Manage GitHub repositories, issues, and pull requests.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "github_oauth",
				Name:        "GitHub Account",
				Description: "The GitHub account to use for the integration",
				Required:    false,
				Type:        domain.NodePropertyType_OAuth,
				OAuthType:   domain.OAuthTypeGitHub,
			},
		},
		Actions: []domain.IntegrationAction{
			// Issue Actions
			{
				ID:          "create_issue",
				Name:        "Create Issue",
				Description: "Create an issue in a GitHub repository",
				ActionType:  GithubActionType_CreateIssue,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to create the issue in (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "title",
						Name:        "Title",
						Description: "The title of the issue",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "The body of the issue",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:          "assignee",
						Name:         "Assignee",
						Description:  "Login of the user to assign the issue to",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Users,
					},
					{
						Key:         "labels",
						Name:        "Labels",
						Description: "List of label names to apply to the issue. Stored as a JSON array of strings.",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
			},
			{
				ID:          "get_issue",
				Name:        "Get Issue",
				Description: "Get an issue from a GitHub repository",
				ActionType:  GithubActionType_GetIssue,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to get the issue from (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "issue_number",
						Name:        "Issue Number",
						Description: "The number of the issue to get",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "create_issue_comment",
				Name:        "Create Issue Comment",
				Description: "Create a comment on an issue",
				ActionType:  GithubActionType_CreateIssueComment,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the issue (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "issue_number",
						Name:        "Issue Number",
						Description: "The number of the issue to comment on",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "The content of the comment",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "edit_issue",
				Name:        "Edit Issue",
				Description: "Edit an existing issue",
				ActionType:  GithubActionType_EditIssue,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the issue (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "issue_number",
						Name:        "Issue Number",
						Description: "The number of the issue to edit",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "title",
						Name:        "Title",
						Description: "New title for the issue",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "New body for the issue",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "state",
						Name:        "State",
						Description: "State of the issue ('open' or 'closed')",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:          "assignee",
						Name:         "Assignee",
						Description:  "Login of the user to assign. Use empty string to unassign single assignee.",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Users,
					},
					{
						Key:         "labels",
						Name:        "Labels",
						Description: "List of label names. Replaces all existing labels. Stored as a JSON array of strings.",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
			},
			{
				ID:          "lock_issue",
				Name:        "Lock Issue",
				Description: "Lock an issue's conversation",
				ActionType:  GithubActionType_LockIssue,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the issue (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "issue_number",
						Name:        "Issue Number",
						Description: "The number of the issue to lock",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "lock_reason",
						Name:        "Lock Reason",
						Description: "Reason for locking ('off-topic', 'too heated', 'resolved', 'spam')",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Off-topic", Value: "off-topic"},
							{Label: "Too heated", Value: "too heated"},
							{Label: "Resolved", Value: "resolved"},
							{Label: "Spam", Value: "spam"},
						},
					},
				},
			},
			// File Actions
			{
				ID:          "create_file",
				Name:        "Create File",
				Description: "Create a new file in a repository",
				ActionType:  GithubActionType_CreateFile,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to create the file in (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "path",
						Name:        "File Path",
						Description: "The path to the file in the repository",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message",
						Name:        "Commit Message",
						Description: "The commit message",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "content",
						Name:        "Content",
						Description: "The content of the file as plain text.",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:          "branch",
						Name:         "Branch",
						Description:  "The branch name. Default: the repository's default branch.",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Branches,
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "repository_id",
								ValueKey:    "repository_id",
							},
						},
					},
				},
			},
			{
				ID:          "delete_file",
				Name:        "Delete File",
				Description: "Delete a file from a repository",
				ActionType:  GithubActionType_DeleteFile,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to delete the file from (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "path",
						Name:        "File Path",
						Description: "The path to the file in the repository",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message",
						Name:        "Commit Message",
						Description: "The commit message",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "sha",
						Name:        "SHA",
						Description: "The blob SHA of the file to be deleted",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:          "branch",
						Name:         "Branch",
						Description:  "The branch name. Default: the repository's default branch.",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Branches,
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "repository_id",
								ValueKey:    "repository_id",
							},
						},
					},
				},
			},
			{
				ID:          "update_file", // Renamed from EditFile for consistency
				Name:        "Update File",
				Description: "Update an existing file in a repository",
				ActionType:  GithubActionType_EditFile, // ActionType still refers to the original constant name
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository where the file exists (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "path",
						Name:        "File Path",
						Description: "The path to the file in the repository",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "message",
						Name:        "Commit Message",
						Description: "The commit message",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "content",
						Name:        "Content",
						Description: "The new content of the file as plain text.",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "sha",
						Name:        "SHA",
						Description: "The blob SHA of the file to be updated",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:          "branch",
						Name:         "Branch",
						Description:  "The branch name. Default: the repository's default branch.",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Branches,
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "repository_id",
								ValueKey:    "repository_id",
							},
						},
					},
				},
			},
			{
				ID:          "get_file",
				Name:        "Get File Content",
				Description: "Get the content of a file in a repository",
				ActionType:  GithubActionType_GetFile,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the file (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "path",
						Name:        "File Path",
						Description: "The path to the file in the repository",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "ref",
						Name:        "Reference",
						Description: "The name of the commit/branch/tag. Default: the repository's default branch.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "list_repository_content", // Renamed from ListFiles
				Name:        "List Repository Content",
				Description: "List content of a directory in a repository",
				ActionType:  GithubActionType_ListFiles, // ActionType still refers to the original constant name
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to list content from (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "path",
						Name:        "Directory Path",
						Description: "The path to the directory. Default: root of the repository.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "ref",
						Name:        "Reference",
						Description: "The name of the commit/branch/tag. Default: the repository's default branch.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			// Organization Actions
			{
				ID:          "org_get_repositories",
				Name:        "List Organization Repositories",
				Description: "List repositories for an organization",
				ActionType:  GithubActionType_OrgGetRepositories,
				Properties: []domain.NodeProperty{
					{
						Key:         "org",
						Name:        "Organization",
						Description: "The name of the GitHub organization",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "type",
						Name:        "Type",
						Description: "Type of repositories to list. Default: all",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "All", Value: "all"},
							{Label: "Public", Value: "public"},
							{Label: "Private", Value: "private"},
							{Label: "Forks", Value: "forks"},
							{Label: "Sources", Value: "sources"},
							{Label: "Member", Value: "member"},
						},
					},
					{
						Key:         "sort",
						Name:        "Sort By",
						Description: "Sort order. Default: created",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Created", Value: "created"},
							{Label: "Updated", Value: "updated"},
							{Label: "Pushed", Value: "pushed"},
							{Label: "Full Name", Value: "full_name"},
						},
					},
					{
						Key:         "direction",
						Name:        "Direction",
						Description: "Direction of sort. Default: desc if sort is 'created', otherwise 'asc'",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Ascending", Value: "asc"},
							{Label: "Descending", Value: "desc"},
						},
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of repositories to return (1-100). Default: 30",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page",
						Name:        "Page",
						Description: "Page number of results to fetch. Default: 1",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			// Release Actions
			{
				ID:          "create_release",
				Name:        "Create Release",
				Description: "Create a new release for a repository",
				ActionType:  GithubActionType_CreateRelease,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to create the release in (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "tag_name",
						Name:        "Tag Name",
						Description: "The name of the tag for this release",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "target_commitish",
						Name:        "Target Commitish",
						Description: "Specifies the commitish value (branch or commit SHA) that the Git tag is created from. Defaults to the repository's default branch.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "name",
						Name:        "Release Name",
						Description: "The name of the release",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Release Body",
						Description: "Text describing the contents of the release",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "draft",
						Name:        "Draft",
						Description: "Set to true to create a draft (unpublished) release. Default: false",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
					{
						Key:         "prerelease",
						Name:        "Prerelease",
						Description: "Set to true to identify the release as a prerelease. Default: false",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
			{
				ID:          "delete_release",
				Name:        "Delete Release",
				Description: "Delete a release from a repository",
				ActionType:  GithubActionType_DeleteRelease,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the release (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "release_id",
						Name:        "Release ID",
						Description: "The unique identifier of the release to delete",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "get_release",
				Name:        "Get Release",
				Description: "Get a specific release from a repository by ID or tag name",
				ActionType:  GithubActionType_GetRelease,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the release (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "release_id",
						Name:        "Release Identifier",
						Description: "The unique numeric ID of the release (as a string), the literal string 'latest', or a tag in the format 'tags/YOUR_TAG_NAME'.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "list_releases",
				Name:        "List Releases",
				Description: "List releases for a repository",
				ActionType:  GithubActionType_ListReleases,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to list releases from (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of releases to return (1-100). Default: 30",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page",
						Name:        "Page",
						Description: "Page number of results to fetch. Default: 1",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "update_release",
				Name:        "Update Release",
				Description: "Update an existing release",
				ActionType:  GithubActionType_UpdateRelease,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the release (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "release_id",
						Name:        "Release ID",
						Description: "The unique identifier of the release to update",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "tag_name",
						Name:        "Tag Name",
						Description: "The name of the tag",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "target_commitish",
						Name:        "Target Commitish",
						Description: "Specifies the commitish value that determines where the Git tag is created from.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "name",
						Name:        "Release Name",
						Description: "The name of the release",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "body",
						Name:        "Release Body",
						Description: "Text describing the contents of the release",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "draft",
						Name:        "Draft",
						Description: "Set to true for a draft release",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
					{
						Key:         "prerelease",
						Name:        "Prerelease",
						Description: "Set to true for a prerelease",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
			// Repository Actions
			{
				ID:          "get_repository",
				Name:        "Get Repository",
				Description: "Get details of a repository",
				ActionType:  GithubActionType_GetRepository,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to get details for (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
				},
			},
			{
				ID:          "list_repository_issues",
				Name:        "List Repository Issues",
				Description: "List issues for a repository",
				ActionType:  GithubActionType_GetRepositoryIssues,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to list issues from (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "milestone",
						Name:        "Milestone",
						Description: "Filter by milestone number, '*' for any, or 'none' for no milestone. Default: *",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "state",
						Name:        "State",
						Description: "Filter by issue state. Default: all",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Open", Value: "open"},
							{Label: "Closed", Value: "closed"},
							{Label: "All", Value: "all"},
						},
					},
					{
						Key:          "assignee",
						Name:         "Assignee",
						Description:  "Filter by assignee's login name. Use '*' for any, 'none' for no assignee",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Users,
					},
					{
						Key:          "creator",
						Name:         "Creator",
						Description:  "Filter by creator's login name",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Users,
					},
					{
						Key:          "mentioned",
						Name:         "Mentioned User",
						Description:  "Filter by login of a user mentioned in the issue.",
						Required:     false,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Users,
					},
					{
						Key:         "labels",
						Name:        "Labels",
						Description: "A list of label names to filter by. Stored as a JSON array of strings.",
						Required:    false,
						Type:        domain.NodePropertyType_TagInput,
					},
					{
						Key:         "sort",
						Name:        "Sort By",
						Description: "Sort results by. Default: created",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Created", Value: "created"},
							{Label: "Updated", Value: "updated"},
							{Label: "Comments", Value: "comments"},
						},
					},
					{
						Key:         "direction",
						Name:        "Direction",
						Description: "Direction of sort. Default: desc",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Ascending", Value: "asc"},
							{Label: "Descending", Value: "desc"},
						},
					},
					{
						Key:         "since",
						Name:        "Since",
						Description: "Timestamp (ISO 8601 YYYY-MM-DDTHH:MM:SSZ). Only issues updated at or after this time are returned.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of issues to return (1-100). Default: 30",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page",
						Name:        "Page",
						Description: "Page number of results to fetch. Default: 1",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "get_repository_license",
				Name:        "Get Repository License",
				Description: "Get the license for a repository",
				ActionType:  GithubActionType_GetRepositoryLicense,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to get the license from (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
				},
			},
			{
				ID:          "get_repository_community_profile", // Renamed from GetRepositoryProfile
				Name:        "Get Repository Community Profile",
				Description: "Get community profile metrics for a repository",
				ActionType:  GithubActionType_GetRepositoryProfile, // ActionType still refers to original
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to get community profile for (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
				},
			},
			{
				ID:          "list_pull_requests",
				Name:        "List Pull Requests",
				Description: "List pull requests for a repository",
				ActionType:  GithubActionType_GetRepositoryPRs,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to list pull requests from (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "state",
						Name:        "State",
						Description: "State of pull requests. Default: open",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Open", Value: "open"},
							{Label: "Closed", Value: "closed"},
							{Label: "All", Value: "all"},
						},
					},
					{
						Key:         "head",
						Name:        "Head",
						Description: "Filter by head user and branch name ('user:ref-name').",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "base",
						Name:        "Base Branch",
						Description: "Filter by base branch name.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "sort",
						Name:        "Sort By",
						Description: "Sort results by. Default: created",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Created", Value: "created"},
							{Label: "Updated", Value: "updated"},
							{Label: "Popularity", Value: "popularity"},
							{Label: "Long-running", Value: "long-running"},
						},
					},
					{
						Key:         "direction",
						Name:        "Direction",
						Description: "Direction of sort. Default: desc",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Ascending", Value: "asc"},
							{Label: "Descending", Value: "desc"},
						},
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of pull requests to return (1-100). Default: 30",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page",
						Name:        "Page",
						Description: "Page number of results to fetch. Default: 1",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "get_top_referral_paths", // Renamed from ListPopularPaths
				Name:        "Get Top Referral Paths",
				Description: "Get the top 10 popular content paths for a repository (last 14 days).",
				ActionType:  GithubActionType_ListPopularPaths, // ActionType still refers to original
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to get top paths for (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
				},
			},
			{
				ID:          "get_top_referral_sources", // Renamed from ListRepositoryReferrers
				Name:        "Get Top Referral Sources",
				Description: "Get the top 10 referral sources for a repository (last 14 days).",
				ActionType:  GithubActionType_ListRepositoryReferrers, // ActionType still refers to original
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to get top sources for (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
				},
			},
			// Review Actions (Pull Request Reviews)
			{
				ID:          "create_pull_request_review",
				Name:        "Create Pull Request Review",
				Description: "Create a review for a pull request",
				ActionType:  GithubActionType_CreateReview,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the pull request (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "pull_number",
						Name:        "Pull Request Number",
						Description: "The number of the pull request",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "event",
						Name:        "Event",
						Description: "The review action. If 'COMMENT', body is required. If 'APPROVE' or 'REQUEST_CHANGES', body is optional.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Approve", Value: "APPROVE"},
							{Label: "Request Changes", Value: "REQUEST_CHANGES"},
							{Label: "Comment", Value: "COMMENT"},
						},
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "The body text of the review. Required if event is 'COMMENT'.",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
					{
						Key:         "commit_id",
						Name:        "Commit ID",
						Description: "The SHA of the commit to review. Defaults to the pull request's latest commit.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_pull_request_review", // Renamed from GetReview
				Name:        "Get Pull Request Review",
				Description: "Get a specific review for a pull request",
				ActionType:  GithubActionType_GetReview, // ActionType still refers to original
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the pull request (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "pull_number",
						Name:        "Pull Request Number",
						Description: "The number of the pull request",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "review_id",
						Name:        "Review ID",
						Description: "The unique identifier of the review",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "list_pull_request_reviews", // Renamed from ListReviews
				Name:        "List Pull Request Reviews",
				Description: "List reviews for a pull request",
				ActionType:  GithubActionType_ListReviews, // ActionType still refers to original
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the pull request (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "pull_number",
						Name:        "Pull Request Number",
						Description: "The number of the pull request",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of reviews to return (1-100). Default: 30",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page",
						Name:        "Page",
						Description: "Page number of results to fetch. Default: 1",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "update_pull_request_review", // Renamed from UpdateReview
				Name:        "Update Pull Request Review",
				Description: "Update the body of an existing review for a pull request",
				ActionType:  GithubActionType_UpdateReview, // ActionType still refers to original
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository containing the pull request (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "pull_number",
						Name:        "Pull Request Number",
						Description: "The number of the pull request",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "review_id",
						Name:        "Review ID",
						Description: "The unique identifier of the review to update",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "The new body text of the review",
						Required:    true,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			// User Actions
			{
				ID:          "list_authenticated_user_repositories",
				Name:        "List Authenticated User Repositories",
				Description: "List repositories for the authenticated user",
				ActionType:  GithubActionType_UserGetRepositories,
				Properties: []domain.NodeProperty{
					{
						Key:         "visibility",
						Name:        "Visibility",
						Description: "Repository visibility. Default: all",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "All", Value: "all"},
							{Label: "Public", Value: "public"},
							{Label: "Private", Value: "private"},
						},
					},
					{
						Key:         "affiliation",
						Name:        "Affiliation",
						Description: "Comma-separated: owner, collaborator, organization_member. Default: all three.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "type",
						Name:        "Type",
						Description: "Repository type. Default: all. Avoid using with visibility or affiliation.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "All", Value: "all"},
							{Label: "Owner", Value: "owner"},
							{Label: "Public", Value: "public"},
							{Label: "Private", Value: "private"},
							{Label: "Member", Value: "member"},
						},
					},
					{
						Key:         "sort",
						Name:        "Sort By",
						Description: "Sort order. Default: full_name",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Created", Value: "created"},
							{Label: "Updated", Value: "updated"},
							{Label: "Pushed", Value: "pushed"},
							{Label: "Full Name", Value: "full_name"},
						},
					},
					{
						Key:         "direction",
						Name:        "Direction",
						Description: "Direction of sort. Default: asc for full_name, else desc.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Ascending", Value: "asc"},
							{Label: "Descending", Value: "desc"},
						},
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of repositories to return (1-100). Default: 30",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "page",
						Name:        "Page",
						Description: "Page number of results to fetch. Default: 1",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "invite_user_to_repository",
				Name:        "Invite User to Repository",
				Description: "Invite a user to collaborate on a repository",
				ActionType:  GithubActionType_UserInvite,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to invite the user to (e.g., 'owner/repo')",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:          "username",
						Name:         "Username",
						Description:  "The GitHub username of the user to invite",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Users,
					},
					{
						Key:         "permission",
						Name:        "Permission",
						Description: "Permission level. Default: push",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Pull", Value: "pull"},
							{Label: "Push", Value: "push"},
							{Label: "Admin", Value: "admin"},
							{Label: "Maintain", Value: "maintain"},
							{Label: "Triage", Value: "triage"},
						},
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "github_event_listener",
				Name:        "GitHub Event Listener",
				Description: "Triggers on selected GitHub events for a repository or organization.",
				EventType:   IntegrationEventType_GithubUniversalTrigger,
				Properties: []domain.NodeProperty{
					{
						Key:          "repository_id",
						Name:         "Repository",
						Description:  "The repository to monitor (e.g., 'owner/repo'). If empty, listens to organization/app level events if applicable for selected event types.",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: GithubPeekable_Repositories,
					},
					{
						Key:         "selected_events",
						Name:        "GitHub Events",
						Description: "Select one or more GitHub events to trigger this flow. The webhook URL for these events will be generated by FlowBaker.",
						Required:    true,
						Type:        domain.NodePropertyType_ListTagInput, // This will be rendered as a multi-select by the UI. Stored as JSON array string.
						Options: []domain.NodePropertyOption{
							{Label: "On Check Run", Value: string(IntegrationEventType_CheckRun), Description: "Triggered when a check run is created, reraised, completed, or has a requested_action."},
							{Label: "On Check Suite", Value: string(IntegrationEventType_CheckSuite), Description: "Triggered when a check suite is completed, requested, or rerequested."},
							{Label: "On Commit Comment Created", Value: string(IntegrationEventType_CommitComment), Description: "Triggered when a commit comment is created."},
							{Label: "On Create (Branch/Tag/Repo)", Value: string(IntegrationEventType_Create), Description: "Triggered when a Git branch or tag is created."},
							{Label: "On Delete (Branch/Tag)", Value: string(IntegrationEventType_Delete), Description: "Triggered when a Git branch or tag is deleted."},
							{Label: "On Deploy Key", Value: string(IntegrationEventType_DeployKey), Description: "Triggered when a deploy key is added or removed from a repository."},
							{Label: "On Deployment", Value: string(IntegrationEventType_Deployment), Description: "Triggered when a deployment is created."},
							{Label: "On Deployment Status", Value: string(IntegrationEventType_DeploymentStatus), Description: "Triggered when a deployment status is updated."},
							{Label: "On Fork Event", Value: string(IntegrationEventType_Fork), Description: "Triggered when a repository is forked."},
							{Label: "On GitHub App Authorization Revoked", Value: string(IntegrationEventType_AppAuthorization), Description: "Triggered when a GitHub App's authorization is revoked by a user."},
							{Label: "On Gollum Event (Wiki Updated)", Value: string(IntegrationEventType_Gollum), Description: "Triggered when a Wiki page is created or updated."},
							{Label: "On Installation", Value: string(IntegrationEventType_Installation), Description: "Triggered when a GitHub App is installed or uninstalled."},
							{Label: "On Installation Repositories", Value: string(IntegrationEventType_InstallationRepositories), Description: "Triggered when repositories are added to or removed from an installation."},
							{Label: "On Issue Comment", Value: string(IntegrationEventType_IssueComment), Description: "Triggered when an issue or pull request comment is created, edited, or deleted."},
							{Label: "On Issues", Value: string(IntegrationEventType_Issues), Description: "Triggered when an issue is opened, edited, deleted, transferred, pinned, unpinned, closed, reopened, assigned, unassigned, labeled, unlabeled, milestoned, or demilestoned."},
							{Label: "On Label", Value: string(IntegrationEventType_Label), Description: "Triggered when a label is created, edited, or deleted."},
							{Label: "On Marketplace Purchase", Value: string(IntegrationEventType_MarketplacePurchase), Description: "Triggered when a user purchases, cancels, or changes their GitHub Marketplace plan."},
							{Label: "On Member (Collaborator)", Value: string(IntegrationEventType_Member), Description: "Triggered when a user is added, removed, or has their permissions changed for a repository."},
							{Label: "On Membership (Team)", Value: string(IntegrationEventType_Membership), Description: "Triggered when a user is added or removed from a team."},
							{Label: "On Meta (Webhook Deleted)", Value: string(IntegrationEventType_Meta), Description: "Triggered when this webhook is deleted."},
							{Label: "On Milestone", Value: string(IntegrationEventType_Milestone), Description: "Triggered when a milestone is created, closed, opened, edited, or deleted."},
							{Label: "On Organization Block", Value: string(IntegrationEventType_OrgBlock), Description: "Triggered when an organization blocks or unblocks a user."},
							{Label: "On Organization", Value: string(IntegrationEventType_Organization), Description: "Triggered when an organization is deleted, renamed, or when a user is added, removed, or invited to an organization."},
							{Label: "On Page Build", Value: string(IntegrationEventType_PageBuild), Description: "Triggered on a successful or failed GitHub Pages build."},
							{Label: "On Project", Value: string(IntegrationEventType_Project), Description: "Triggered when a project is created, updated, closed, reopened, or deleted."},
							{Label: "On Project Card", Value: string(IntegrationEventType_ProjectCard), Description: "Triggered when a project card is created, edited, moved, converted, or deleted."},
							{Label: "On Project Column", Value: string(IntegrationEventType_ProjectColumn), Description: "Triggered when a project column is created, updated, moved, or deleted."},
							{Label: "On Public (Repo Open Sourced)", Value: string(IntegrationEventType_Public), Description: "Triggered when a private repository is made public."},
							{Label: "On Pull Request", Value: string(IntegrationEventType_PullRequest), Description: "Triggered when a pull request is opened, closed, reopened, edited, assigned, unassigned, review requested, review request removed, labeled, unlabeled, or synchronized."},
							{Label: "On Pull Request Review", Value: string(IntegrationEventType_PullRequestReview), Description: "Triggered when a pull request review is submitted, edited, or dismissed."},
							{Label: "On Pull Request Review Comment", Value: string(IntegrationEventType_PullRequestReviewComment), Description: "Triggered when a comment on a pull request's unified diff is created, edited, or deleted."},
							{Label: "On Push", Value: string(IntegrationEventType_Push), Description: "Triggered on a Git push to a repository."},
							{Label: "On Release", Value: string(IntegrationEventType_Release), Description: "Triggered when a release is published, unpublished, created, edited, deleted, or prereleased."},
							{Label: "On Repository", Value: string(IntegrationEventType_Repository), Description: "Triggered when a repository is created, deleted, archived, unarchived, made public, made private, or renamed."},
							{Label: "On Repository Import", Value: string(IntegrationEventType_RepositoryImport), Description: "Triggered when a repository import is successful, failed, or cancelled."},
							{Label: "On Repository Vulnerability Alert", Value: string(IntegrationEventType_RepositoryVulnerabilityAlert), Description: "Triggered when a security vulnerability is found or resolved in a repository."},
							{Label: "On Security Advisory", Value: string(IntegrationEventType_SecurityAdvisory), Description: "Triggered when a new security advisory is published, updated, or withdrawn."},
							{Label: "On Star", Value: string(IntegrationEventType_Star), Description: "Triggered when a repository is starred or unstarred."},
							{Label: "On Status (Commit)", Value: string(IntegrationEventType_Status), Description: "Triggered when the status of a Git commit changes."},
							{Label: "On Team", Value: string(IntegrationEventType_Team), Description: "Triggered when a team is created, deleted, edited, added to repository, or removed from repository."},
							{Label: "On Team Add (Repo to Team)", Value: string(IntegrationEventType_TeamAdd), Description: "Triggered when a repository is added to a team."},
							{Label: "On Watch (Legacy Star)", Value: string(IntegrationEventType_Watch), Description: "Triggered when a user stars a repository (legacy event)."},
						},
					},
				},
			},
		},
	}
)
