package githubintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/google/go-github/v57/github" // Assuming a recent version, adjust if necessary
	"golang.org/x/oauth2"
)

const (
	// File Actions
	GithubActionType_CreateFile domain.IntegrationActionType = "github_create_file"
	GithubActionType_DeleteFile domain.IntegrationActionType = "github_delete_file"
	GithubActionType_EditFile   domain.IntegrationActionType = "github_edit_file"
	GithubActionType_GetFile    domain.IntegrationActionType = "github_get_file"
	GithubActionType_ListFiles  domain.IntegrationActionType = "github_list_files"

	// Organization Actions
	GithubActionType_OrgGetRepositories domain.IntegrationActionType = "github_org_get_repositories"

	// Release Actions
	GithubActionType_CreateRelease domain.IntegrationActionType = "github_create_release"
	GithubActionType_DeleteRelease domain.IntegrationActionType = "github_delete_release"
	GithubActionType_GetRelease    domain.IntegrationActionType = "github_get_release"
	GithubActionType_ListReleases  domain.IntegrationActionType = "github_list_releases"
	GithubActionType_UpdateRelease domain.IntegrationActionType = "github_update_release"

	// Repository Actions
	GithubActionType_GetRepository           domain.IntegrationActionType = "github_get_repository"
	GithubActionType_GetRepositoryIssues     domain.IntegrationActionType = "github_get_repository_issues"
	GithubActionType_GetRepositoryLicense    domain.IntegrationActionType = "github_get_repository_license"
	GithubActionType_GetRepositoryProfile    domain.IntegrationActionType = "github_get_repository_profile"
	GithubActionType_GetRepositoryPRs        domain.IntegrationActionType = "github_get_repository_prs"
	GithubActionType_ListPopularPaths        domain.IntegrationActionType = "github_list_popular_paths"
	GithubActionType_ListRepositoryReferrers domain.IntegrationActionType = "github_list_repository_referrers"

	// Review Actions
	GithubActionType_CreateReview domain.IntegrationActionType = "github_create_review"
	GithubActionType_GetReview    domain.IntegrationActionType = "github_get_review"
	GithubActionType_ListReviews  domain.IntegrationActionType = "github_list_reviews"
	GithubActionType_UpdateReview domain.IntegrationActionType = "github_update_review"

	// User Actions
	GithubActionType_UserGetRepositories domain.IntegrationActionType = "github_user_get_repositories"
	GithubActionType_UserInvite          domain.IntegrationActionType = "github_user_invite"

	// Issue Actions
	GithubActionType_CreateIssue        domain.IntegrationActionType = "github_create_issue"
	GithubActionType_CreateIssueComment domain.IntegrationActionType = "github_create_issue_comment"
	GithubActionType_EditIssue          domain.IntegrationActionType = "github_edit_issue"
	GithubActionType_GetIssue           domain.IntegrationActionType = "github_get_issue"
	GithubActionType_LockIssue          domain.IntegrationActionType = "github_lock_issue"

	// Peekable Types
	GithubPeekable_Repositories domain.IntegrationPeekableType = "github_repositories"
	GithubPeekable_Users        domain.IntegrationPeekableType = "github_users"
	GithubPeekable_Branches     domain.IntegrationPeekableType = "github_branches"
)

type GithubIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	// Add other necessary services like routeService, oauthAccountRepo if needed for GitHub
}

func NewGithubIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &GithubIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *GithubIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewGithubIntegration(ctx, GithubIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

// GithubIntegration implements the domain.IntegrationExecutor interface for GitHub.
// It handles the execution of specific actions (defined in schema.go) like creating issues,
// fetching files, etc., and provides data for UI elements via peek functions.
//
// Note on Triggers:
// The triggering of flows based on GitHub events (e.g., push, issues) is primarily managed
// by FlowBaker's core event dispatching system. The `GithubSchema` in `schema.go` defines
// a universal trigger (`IntegrationEventType_GithubUniversalTrigger`) that allows users to
// select multiple GitHub event types. The dispatching system is responsible for:
//  1. Receiving webhook events from GitHub.
//  2. Matching these events against the `selected_events` configured for the universal trigger
//     in a flow.
//  3. Evaluating any `event_filters` specified in the trigger configuration.
//
// This GithubIntegration's `Execute` method is called when an *action* within a flow
// is run, not directly for trigger event dispatch.
type GithubIntegration struct {
	githubClient *github.Client

	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]

	actionManager *domain.IntegrationActionManager

	peekFuncs map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type GithubIntegrationDependencies struct {
	CredentialID string

	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	// Add other repo/service dependencies here
}

func NewGithubIntegration(ctx context.Context, deps GithubIntegrationDependencies) (*GithubIntegration, error) {
	integration := &GithubIntegration{
		binder:           deps.ParameterBinder,
		credentialGetter: deps.CredentialGetter,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(GithubActionType_CreateFile, integration.CreateFile).
		AddPerItem(GithubActionType_DeleteFile, integration.DeleteFile).
		AddPerItem(GithubActionType_EditFile, integration.EditFile).
		AddPerItem(GithubActionType_GetFile, integration.GetFile).
		AddPerItemMulti(GithubActionType_ListFiles, integration.ListFiles).
		AddPerItemMulti(GithubActionType_OrgGetRepositories, integration.OrgGetRepositories).
		AddPerItemMulti(GithubActionType_ListReleases, integration.ListReleases).
		AddPerItemMulti(GithubActionType_GetRepositoryIssues, integration.GetRepositoryIssues).
		AddPerItemMulti(GithubActionType_GetRepositoryPRs, integration.GetRepositoryPRs).
		AddPerItemMulti(GithubActionType_ListPopularPaths, integration.ListPopularPaths).
		AddPerItemMulti(GithubActionType_ListRepositoryReferrers, integration.ListRepositoryReferrers).
		AddPerItemMulti(GithubActionType_UserGetRepositories, integration.UserGetRepositories).
		AddPerItemMulti(GithubActionType_ListReviews, integration.ListReviews).
		AddPerItem(GithubActionType_CreateRelease, integration.CreateRelease).
		AddPerItem(GithubActionType_DeleteRelease, integration.DeleteRelease).
		AddPerItem(GithubActionType_GetRelease, integration.GetRelease).
		AddPerItem(GithubActionType_UpdateRelease, integration.UpdateRelease).
		AddPerItem(GithubActionType_GetRepository, integration.GetRepository).
		AddPerItem(GithubActionType_GetRepositoryLicense, integration.GetRepositoryLicense).
		AddPerItem(GithubActionType_GetRepositoryProfile, integration.GetRepositoryProfile).
		AddPerItem(GithubActionType_CreateReview, integration.CreateReview).
		AddPerItem(GithubActionType_GetReview, integration.GetReview).
		AddPerItem(GithubActionType_UpdateReview, integration.UpdateReview).
		AddPerItem(GithubActionType_UserInvite, integration.UserInvite).
		AddPerItem(GithubActionType_CreateIssue, integration.CreateIssue).
		AddPerItem(GithubActionType_CreateIssueComment, integration.CreateIssueComment).
		AddPerItem(GithubActionType_EditIssue, integration.EditIssue).
		AddPerItem(GithubActionType_GetIssue, integration.GetIssue).
		AddPerItem(GithubActionType_LockIssue, integration.LockIssue)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		GithubPeekable_Repositories: integration.PeekRepositories,
		GithubPeekable_Users:        integration.PeekUsers,
		GithubPeekable_Branches:     integration.PeekBranches,
	}

	integration.actionManager = actionManager
	integration.peekFuncs = peekFuncs

	if deps.CredentialID == "" {
		return nil, fmt.Errorf("credential ID is required for GitHub integration")
	}

	oauthAccount, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get decrypted GitHub OAuth credential: %w", err)
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: oauthAccount.AccessToken},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	integration.githubClient = github.NewClient(tc)

	return integration, nil
}

func (i *GithubIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

// parseOwnerRepo extracts owner and repository name from repository_id parameter
// Supports formats: "owner/repo" or just "repo" (uses authenticated user as owner)
func (i *GithubIntegration) parseOwnerRepo(ctx context.Context, repoID string) (string, string, error) {
	ownerRepo := strings.Split(repoID, "/")
	if len(ownerRepo) == 2 {
		return ownerRepo[0], ownerRepo[1], nil
	} else if len(ownerRepo) == 1 {
		user, _, err := i.githubClient.Users.Get(ctx, "")
		if err != nil || user.Login == nil {
			return "", "", fmt.Errorf("repository_id must be in 'owner/repo' format or an owner must be determinable: %v", err)
		}
		return *user.Login, repoID, nil
	} else {
		return "", "", fmt.Errorf("invalid repository_id format: %s", repoID)
	}
}

func (i *GithubIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function %s not found for GitHub integration", params.PeekableType)
	}
	return peekFunc(ctx, params)
}

// Example of how to use commonLogicForAllActions
// Define Params struct for each action if needed
// type CreateFileParams struct {
// 	Owner      string `json:"owner"`
// 	Repo       string `json:"repo"`
// 	Path       string `json:"path"`
// 	Message    string `json:"message"`
// 	Content    []byte `json:"content"` // Base64 encoded
// 	Branch     string `json:"branch,omitempty"`
// }

// File Actions
func (i *GithubIntegration) CreateFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateFileParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	// Content is now plain text, convert directly to bytes for the GitHub library.
	contentBytes := []byte(params.Content)

	opts := &github.RepositoryContentFileOptions{
		Message: &params.Message,
		Content: contentBytes,
		Branch:  params.Branch,
	}

	content, _, err := i.githubClient.Repositories.CreateFile(ctx, owner, repoName, params.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s in %s/%s: %w", params.Path, owner, repoName, err)
	}

	return content, nil
}

func (i *GithubIntegration) DeleteFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := DeleteFileParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required to delete a file")
	}
	if params.Message == "" {
		return nil, fmt.Errorf("message (commit message) is required to delete a file")
	}
	if params.SHA == "" {
		return nil, fmt.Errorf("sha is required to delete a file")
	}

	opts := &github.RepositoryContentFileOptions{
		Message: &params.Message,
		SHA:     &params.SHA,
		Branch:  params.Branch,
	}

	resp, _, err := i.githubClient.Repositories.DeleteFile(ctx, owner, repoName, params.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to delete file %s in %s/%s: %w", params.Path, owner, repoName, err)
	}

	return resp, nil // Returns *RepositoryContentResponse
}

func (i *GithubIntegration) EditFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateFileParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required to update a file")
	}
	if params.Message == "" {
		return nil, fmt.Errorf("message (commit message) is required to update a file")
	}
	if params.Content == "" {
		// It's valid to update a file to be empty, so this check might be too strict.
		// However, if content is truly optional for an update, the API might require other fields or handling.
		// For now, keeping it as required for an update action to provide new content.
		return nil, fmt.Errorf("content is required to update a file")
	}
	if params.SHA == nil || *params.SHA == "" {
		return nil, fmt.Errorf("SHA is required to update file %s in %s/%s", params.Path, owner, repoName)
	}

	// Content is now plain text, convert directly to bytes for the GitHub library.
	contentBytes := []byte(params.Content)

	opts := &github.RepositoryContentFileOptions{
		Message: &params.Message,
		Content: contentBytes,
		SHA:     params.SHA, // SHA is required for update
		Branch:  params.Branch,
	}

	content, _, err := i.githubClient.Repositories.UpdateFile(ctx, owner, repoName, params.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to update file %s in %s/%s: %w", params.Path, owner, repoName, err)
	}

	return content, nil // Returns *RepositoryContentResponse
}

func (i *GithubIntegration) GetFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetFileParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}
	if params.Path == "" {
		return nil, fmt.Errorf("path is required to get a file")
	}

	var opts *github.RepositoryContentGetOptions
	if params.Ref != nil && *params.Ref != "" {
		opts = &github.RepositoryContentGetOptions{Ref: *params.Ref}
	}

	// GetContents returns fileContent, directoryContent, resp, err
	// We are interested in fileContent for GetFile.
	fileContent, _, _, err := i.githubClient.Repositories.GetContents(ctx, owner, repoName, params.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get file %s from %s/%s: %w", params.Path, owner, repoName, err)
	}
	if fileContent == nil {
		// This means the path is likely a directory or does not exist.
		// The GetContents API returns (nil, directoryContents, resp, err) if it's a directory.
		return nil, fmt.Errorf("path %s in %s/%s is a directory or does not exist as a file", params.Path, owner, repoName)
	}

	return fileContent, nil // Returns *github.RepositoryContent if it's a file
}

func (i *GithubIntegration) ListFiles(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListFilesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}
	// Path is optional for ListFiles, if empty, it lists the root directory.

	var opts *github.RepositoryContentGetOptions
	if params.Ref != nil && *params.Ref != "" {
		opts = &github.RepositoryContentGetOptions{Ref: *params.Ref}
	}

	// GetContents returns fileContent, directoryContent, resp, err
	// We are interested in directoryContent for ListFiles.
	_, dirContents, _, err := i.githubClient.Repositories.GetContents(ctx, owner, repoName, params.Path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list files/directories at path '%s' in %s/%s: %w", params.Path, owner, repoName, err)
	}
	// If dirContent is nil, it could mean the path was a file, or the directory is empty (or non-existent and not an error the API threw)
	// For ListFiles, we want to return nil if the path is a file or an empty/non-existent directory that didn't error.

	items := make([]domain.Item, 0)

	for _, dirContent := range dirContents {
		items = append(items, dirContent)
	}

	return items, nil
}

// Organization Actions
func (i *GithubIntegration) OrgGetRepositories(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := OrgGetRepositoriesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.Org == "" {
		return nil, fmt.Errorf("organization name (org) is required")
	}

	// Set limit with default value
	limit := 30
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 100 {
		limit = *params.Limit
	}

	// Set page with default value
	page := 1
	if params.Page != nil && *params.Page > 0 {
		page = *params.Page
	}

	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: limit, Page: page},
	}
	if params.Type != nil && *params.Type != "" {
		opts.Type = *params.Type
	} else {
		opts.Type = "all"
	}
	if params.Sort != nil && *params.Sort != "" {
		opts.Sort = *params.Sort
	} else {
		opts.Sort = "created"
	}

	if params.Direction != nil && *params.Direction != "" {
		opts.Direction = *params.Direction
	} else {
		if opts.Sort == "created" || opts.Sort == "updated" || opts.Sort == "pushed" { // These default to desc
			opts.Direction = "desc"
		} else { // full_name defaults to asc
			opts.Direction = "asc"
		}
	}

	// Get only the requested number of repositories (single page)
	repos, _, err := i.githubClient.Repositories.ListByOrg(ctx, params.Org, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for organization %s: %w", params.Org, err)
	}

	items := make([]domain.Item, 0)

	for _, repo := range repos {
		items = append(items, repo)
	}

	return items, nil
}

// Release Actions
func (i *GithubIntegration) CreateRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateReleaseParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.TagName == "" {
		return nil, fmt.Errorf("tag_name is required")
	}

	releaseReq := &github.RepositoryRelease{
		TagName:              &params.TagName,
		TargetCommitish:      params.TargetCommitish,
		Name:                 params.Name,
		Body:                 params.Body,
		Draft:                params.Draft,
		Prerelease:           params.Prerelease,
		GenerateReleaseNotes: params.GenerateReleaseNotes,
	}

	release, _, err := i.githubClient.Repositories.CreateRelease(ctx, owner, repoName, releaseReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create release for %s/%s tag %s: %w", owner, repoName, params.TagName, err)
	}

	return release, nil
}

func (i *GithubIntegration) DeleteRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := DeleteReleaseParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.ReleaseID == 0 {
		return nil, fmt.Errorf("release_id is required")
	}

	_, err = i.githubClient.Repositories.DeleteRelease(ctx, owner, repoName, params.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete release ID %d from %s/%s: %w", params.ReleaseID, owner, repoName, err)
	}

	return map[string]interface{}{"message": fmt.Sprintf("Successfully deleted release ID %d from %s/%s", params.ReleaseID, owner, repoName)}, nil
}

func (i *GithubIntegration) GetRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetReleaseParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.ReleaseID == "" {
		return nil, fmt.Errorf("release_id (ID, 'latest', or 'tags/:tag') is required")
	}

	var release *github.RepositoryRelease
	if params.ReleaseID == "latest" {
		release, _, err = i.githubClient.Repositories.GetLatestRelease(ctx, owner, repoName)
	} else if strings.HasPrefix(params.ReleaseID, "tags/") {
		tag := strings.TrimPrefix(params.ReleaseID, "tags/")
		release, _, err = i.githubClient.Repositories.GetReleaseByTag(ctx, owner, repoName, tag)
	} else {
		releaseIDInt, convErr := strconv.ParseInt(params.ReleaseID, 10, 64)
		if convErr != nil {
			return nil, fmt.Errorf("invalid release_id format: %s. Must be int ID, 'latest', or 'tags/:tag'", params.ReleaseID)
		}
		release, _, err = i.githubClient.Repositories.GetRelease(ctx, owner, repoName, releaseIDInt)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get release '%s' from %s/%s: %w", params.ReleaseID, owner, repoName, err)
	}

	return release, nil
}

func (i *GithubIntegration) ListReleases(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListReleasesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	// Set limit with default value
	limit := 30
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 100 {
		limit = *params.Limit
	}

	// Set page with default value
	page := 1
	if params.Page != nil && *params.Page > 0 {
		page = *params.Page
	}

	opts := &github.ListOptions{PerPage: limit, Page: page}
	releases, _, err := i.githubClient.Repositories.ListReleases(ctx, owner, repoName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases for %s/%s: %w", owner, repoName, err)
	}

	items := make([]domain.Item, 0)

	for _, release := range releases {
		items = append(items, release)
	}

	return items, nil
}

func (i *GithubIntegration) UpdateRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := UpdateReleaseParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.ReleaseID == 0 {
		return nil, fmt.Errorf("release_id is required to update")
	}

	releaseReq := &github.RepositoryRelease{
		TagName:              params.TagName,
		TargetCommitish:      params.TargetCommitish,
		Name:                 params.Name,
		Body:                 params.Body,
		Draft:                params.Draft,
		Prerelease:           params.Prerelease,
		GenerateReleaseNotes: params.GenerateReleaseNotes,
	}

	updatedRelease, _, err := i.githubClient.Repositories.EditRelease(ctx, owner, repoName, params.ReleaseID, releaseReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update release ID %d for %s/%s: %w", params.ReleaseID, owner, repoName, err)
	}

	return updatedRelease, nil
}

// Repository Actions
func (i *GithubIntegration) GetRepository(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetRepositoryParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	repo, _, err := i.githubClient.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository %s/%s: %w", owner, repoName, err)
	}

	return repo, nil
}

func (i *GithubIntegration) GetRepositoryIssues(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetRepositoryIssuesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	// Set limit with default value
	limit := 30
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 100 {
		limit = *params.Limit
	}

	// Set page with default value
	page := 1
	if params.Page != nil && *params.Page > 0 {
		page = *params.Page
	}

	opts := &github.IssueListByRepoOptions{
		ListOptions: github.ListOptions{PerPage: limit, Page: page},
	}
	if params.Milestone != nil && *params.Milestone != "" {
		opts.Milestone = *params.Milestone
	}
	// Don't set default milestone filter - let GitHub use its default
	if params.State != nil && *params.State != "" {
		opts.State = *params.State
	} else {
		opts.State = "all" // Changed from "open" to "all" to show both open and closed issues by default
	}
	if params.Assignee != nil && *params.Assignee != "" {
		// GitHub API treats "*" as "any assignee" but sometimes doesn't work as expected
		// If user selects "*" (any), don't set the assignee filter at all
		if *params.Assignee != "*" {
			opts.Assignee = *params.Assignee
		}
		// If "*" is selected, leave opts.Assignee empty to get all issues regardless of assignee
	}
	// Don't set default assignee filter - let GitHub use its default
	if params.Creator != nil && *params.Creator != "" {
		// GitHub API treats "*" as "any creator" but sometimes doesn't work as expected
		// If user selects "*" (any), don't set the creator filter at all
		if *params.Creator != "*" {
			opts.Creator = *params.Creator
		}
		// If "*" is selected, leave opts.Creator empty to get all issues regardless of creator
	}
	if params.Mentioned != nil && *params.Mentioned != "" {
		opts.Mentioned = *params.Mentioned
	}
	if params.Labels != nil && len(*params.Labels) > 0 {
		opts.Labels = *params.Labels
	}
	if params.Sort != nil && *params.Sort != "" {
		opts.Sort = *params.Sort
	} else {
		opts.Sort = "created"
	}
	if params.Direction != nil && *params.Direction != "" {
		opts.Direction = *params.Direction
	} else {
		opts.Direction = "desc"
	}
	if params.Since != nil && *params.Since != "" {
		t, err := time.Parse(time.RFC3339, *params.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid since date format: %w", err)
		}
		opts.Since = t
	}

	issues, _, err := i.githubClient.Issues.ListByRepo(ctx, owner, repoName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues for %s/%s: %w", owner, repoName, err)
	}

	items := make([]domain.Item, 0)

	for _, issue := range issues {
		items = append(items, issue)
	}

	return items, nil
}

func (i *GithubIntegration) GetRepositoryLicense(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetRepositoryLicenseParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	license, _, err := i.githubClient.Repositories.License(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get license for %s/%s: %w", owner, repoName, err)
	}

	return license, nil
}

type GetRepositoryParams struct {
	Owner string `json:"repository_id"`
}

func (i *GithubIntegration) GetRepositoryProfile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetRepositoryParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	profile, _, err := i.githubClient.Repositories.GetCommunityHealthMetrics(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get community profile for repository %s/%s: %w", owner, repoName, err)
	}

	return profile, nil
}

func (i *GithubIntegration) GetRepositoryPRs(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetRepositoryPRsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	// Set limit with default value
	limit := 30
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 100 {
		limit = *params.Limit
	}

	// Set page with default value
	page := 1
	if params.Page != nil && *params.Page > 0 {
		page = *params.Page
	}

	opts := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: limit, Page: page},
	}
	if params.State != nil && *params.State != "" {
		opts.State = *params.State
	} else {
		opts.State = "open"
	}
	if params.Head != nil && *params.Head != "" {
		opts.Head = *params.Head
	}
	if params.Base != nil && *params.Base != "" {
		opts.Base = *params.Base
	}
	if params.Sort != nil && *params.Sort != "" {
		opts.Sort = *params.Sort
	} else {
		opts.Sort = "created"
	}
	if params.Direction != nil && *params.Direction != "" {
		opts.Direction = *params.Direction
	} else {
		opts.Direction = "desc"
	}

	prs, _, err := i.githubClient.PullRequests.List(ctx, owner, repoName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests for %s/%s: %w", owner, repoName, err)
	}

	items := make([]domain.Item, 0)

	for _, pr := range prs {
		items = append(items, pr)
	}

	return items, nil
}

func (i *GithubIntegration) ListPopularPaths(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetRepositoryParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	paths, _, err := i.githubClient.Repositories.ListTrafficPaths(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to list popular paths for repository %s/%s: %w", owner, repoName, err)
	}

	items := make([]domain.Item, 0)

	for _, path := range paths {
		items = append(items, path)
	}

	return items, nil
}

func (i *GithubIntegration) ListRepositoryReferrers(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetRepositoryParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	referrers, _, err := i.githubClient.Repositories.ListTrafficReferrers(ctx, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to list repository referrers for %s/%s: %w", owner, repoName, err)
	}

	items := make([]domain.Item, 0)

	for _, referrer := range referrers {
		items = append(items, referrer)
	}

	return items, nil
}

// Review Actions
func (i *GithubIntegration) CreateReview(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateReviewParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.PullNumber == 0 {
		return nil, fmt.Errorf("pull_number is required to create a review")
	}
	// Body is required if event is COMMENT, or if event is not provided.
	if (params.Event == nil || *params.Event == "COMMENT") && (params.Body == nil || *params.Body == "") {
		return nil, fmt.Errorf("body is required when event is COMMENT or not specified")
	}

	var comments []*github.DraftReviewComment
	if params.Comments != nil {
		comments = *params.Comments
	}

	reviewRequest := &github.PullRequestReviewRequest{
		CommitID: params.CommitID,
		Body:     params.Body,
		Event:    params.Event,
		Comments: comments,
	}

	review, _, err := i.githubClient.PullRequests.CreateReview(ctx, owner, repoName, params.PullNumber, reviewRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create review for PR #%d in %s/%s: %w", params.PullNumber, owner, repoName, err)
	}

	return review, nil
}

func (i *GithubIntegration) GetReview(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetReviewParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.PullNumber == 0 {
		return nil, fmt.Errorf("pull_number is required to get a review")
	}
	if params.ReviewID == 0 {
		return nil, fmt.Errorf("review_id is required to get a review")
	}

	review, _, err := i.githubClient.PullRequests.GetReview(ctx, owner, repoName, params.PullNumber, params.ReviewID)
	if err != nil {
		return nil, fmt.Errorf("failed to get review ID %d for PR #%d in %s/%s: %w", params.ReviewID, params.PullNumber, owner, repoName, err)
	}

	return review, nil
}

func (i *GithubIntegration) ListReviews(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListReviewsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}
	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.PullNumber == 0 {
		return nil, fmt.Errorf("pull_number is required to list reviews")
	}

	// Set limit with default value
	limit := 30
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 100 {
		limit = *params.Limit
	}

	// Set page with default value
	page := 1
	if params.Page != nil && *params.Page > 0 {
		page = *params.Page
	}

	opts := &github.ListOptions{PerPage: limit, Page: page}
	reviews, _, err := i.githubClient.PullRequests.ListReviews(ctx, owner, repoName, params.PullNumber, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list reviews for PR #%d in %s/%s: %w", params.PullNumber, owner, repoName, err)
	}

	items := make([]domain.Item, 0)

	for _, review := range reviews {
		items = append(items, review)
	}

	return items, nil
}

func (i *GithubIntegration) UpdateReview(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := UpdateReviewParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.PullNumber == 0 {
		return nil, fmt.Errorf("pull_number is required to update a review")
	}
	if params.ReviewID == 0 {
		return nil, fmt.Errorf("review_id is required to update a review")
	}
	if params.Body == "" {
		return nil, fmt.Errorf("body is required to update a review")
	}

	review, _, err := i.githubClient.PullRequests.UpdateReview(ctx, owner, repoName, params.PullNumber, params.ReviewID, params.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to update review ID %d for PR #%d in %s/%s: %w", params.ReviewID, params.PullNumber, owner, repoName, err)
	}

	return review, nil
}

// User Actions
func (i *GithubIntegration) UserGetRepositories(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := UserGetRepositoriesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	// Set limit with default value
	limit := 30
	if params.Limit != nil && *params.Limit > 0 && *params.Limit <= 100 {
		limit = *params.Limit
	}

	// Set page with default value
	page := 1
	if params.Page != nil && *params.Page > 0 {
		page = *params.Page
	}

	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: limit, Page: page},
	}
	if params.Visibility != nil && *params.Visibility != "" {
		opts.Visibility = *params.Visibility
	} else {
		opts.Visibility = "all"
	}
	if params.Affiliation != nil && *params.Affiliation != "" {
		opts.Affiliation = *params.Affiliation
	} else {
		opts.Affiliation = "owner,collaborator,organization_member"
	}
	// Not setting Type by default as GitHub advises against using it with visibility or affiliation.
	if params.Type != nil && *params.Type != "" {
		opts.Type = *params.Type
	}
	if params.Sort != nil && *params.Sort != "" {
		opts.Sort = *params.Sort
	} else {
		opts.Sort = "full_name"
	}
	if params.Direction != nil && *params.Direction != "" {
		opts.Direction = *params.Direction
	} else {
		if opts.Sort == "full_name" {
			opts.Direction = "asc"
		} else {
			opts.Direction = "desc"
		}
	}

	// Empty string for user lists repositories for the authenticated user.
	repos, _, err := i.githubClient.Repositories.List(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for authenticated user: %w", err)
	}

	items := make([]domain.Item, 0)

	for _, repo := range repos {
		items = append(items, repo)
	}

	return items, nil
}

func (i *GithubIntegration) UserInvite(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := UserInviteParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.Username == "" {
		return nil, fmt.Errorf("username is required to invite a user to a repository")
	}

	opts := &github.RepositoryAddCollaboratorOptions{}
	if params.Permission != nil && *params.Permission != "" {
		opts.Permission = *params.Permission
	} else {
		opts.Permission = "push" // Default to push as per schema, API default might be 'read'
	}

	invitation, _, err := i.githubClient.Repositories.AddCollaborator(ctx, owner, repoName, params.Username, opts)
	if err != nil {
		// Check if it's an error because user is already a collaborator (often a 204 No Content on success or specific message)
		// For now, just propagate error.
		return nil, fmt.Errorf("failed to invite user %s to repository %s/%s: %w", params.Username, owner, repoName, err)
	}
	// If successful and invitation is nil (e.g. user already collaborator, or direct add without invitation object being returned for org repos)
	if invitation == nil {
		return map[string]interface{}{"message": fmt.Sprintf("Successfully added or confirmed %s as collaborator to %s/%s with permission %s", params.Username, owner, repoName, opts.Permission)}, nil
	}

	return invitation, nil
}

// Issue Actions

type CreateIssueParams struct {
	Owner    string   `json:"repository_id"` // Assuming repository_id is in owner/repo format or just repo name if owner is implicit
	Repo     string   `json:"-"`             // Will be extracted from Owner if in owner/repo format
	Title    string   `json:"title"`
	Body     string   `json:"body,omitempty"`
	Assignee string   `json:"assignee,omitempty"` // Single assignee for simplicity, can be changed to []string
	Labels   []string `json:"labels,omitempty"`
}

type GetIssueParams struct {
	Owner       string `json:"repository_id"` // Assuming repository_id is in owner/repo format
	Repo        string `json:"-"`             // Will be extracted from Owner
	IssueNumber int    `json:"issue_number"`
}

type CreateIssueCommentParams struct {
	Owner       string `json:"repository_id"`
	Repo        string `json:"-"`
	IssueNumber int    `json:"issue_number"`
	Body        string `json:"body"`
}

type EditIssueParams struct {
	Owner       string  `json:"repository_id"`
	Repo        string  `json:"-"`
	IssueNumber int     `json:"issue_number"`
	Title       *string `json:"title,omitempty"`
	Body        *string `json:"body,omitempty"`
	State       *string `json:"state,omitempty"`
	// For Assignees, the GitHub API uses a list.
	// If a single 'assignee' string is provided, it will be wrapped in a list.
	// If 'assignees' list is provided, it will be used directly.
	// To clear assignees, provide an empty list for 'assignees' or an empty string for 'assignee'.
	Assignee  *string   `json:"assignee,omitempty"`
	Assignees *[]string `json:"assignees,omitempty"`
	Labels    *[]string `json:"labels,omitempty"`    // To clear, provide an empty list
	Milestone *int      `json:"milestone,omitempty"` // To clear, provide null or omit. GitHub API might require explicit null or 0.
}

type LockIssueParams struct {
	Owner       string  `json:"repository_id"`
	Repo        string  `json:"-"`
	IssueNumber int     `json:"issue_number"`
	LockReason  *string `json:"lock_reason,omitempty"` // off-topic, too heated, resolved, spam
}

func (i *GithubIntegration) CreateIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateIssueParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}
	params.Repo = repoName

	issueRequest := &github.IssueRequest{
		Title: &params.Title,
	}
	if params.Body != "" {
		issueRequest.Body = &params.Body
	}
	if params.Assignee != "" { // Simplified: sets a single assignee
		issueRequest.Assignees = &[]string{params.Assignee}
	}
	if len(params.Labels) > 0 {
		issueRequest.Labels = &params.Labels
	}

	issue, _, err := i.githubClient.Issues.Create(ctx, owner, repoName, issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue in %s/%s: %w", owner, repoName, err)
	}

	return issue, nil
}

func (i *GithubIntegration) CreateIssueComment(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateIssueCommentParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.IssueNumber == 0 {
		return nil, fmt.Errorf("issue_number is required")
	}
	if params.Body == "" {
		return nil, fmt.Errorf("body is required for comment")
	}

	comment := &github.IssueComment{Body: &params.Body}
	issueComment, _, err := i.githubClient.Issues.CreateComment(ctx, owner, repoName, params.IssueNumber, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment for issue #%d in %s/%s: %w", params.IssueNumber, owner, repoName, err)
	}

	return issueComment, nil
}

func (i *GithubIntegration) EditIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := EditIssueParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.IssueNumber == 0 {
		return nil, fmt.Errorf("issue_number is required")
	}

	issueRequest := &github.IssueRequest{}
	updated := false
	if params.Title != nil {
		issueRequest.Title = params.Title
		updated = true
	}
	if params.Body != nil {
		issueRequest.Body = params.Body
		updated = true
	}
	if params.State != nil {
		issueRequest.State = params.State
		updated = true
	}
	if params.Assignee != nil { // Single assignee string
		if *params.Assignee == "" { // Empty string to clear
			issueRequest.Assignees = &[]string{}
		} else {
			issueRequest.Assignees = &[]string{*params.Assignee}
		}
		updated = true
	} else if params.Assignees != nil { // Slice of assignees
		issueRequest.Assignees = params.Assignees
		updated = true
	}
	if params.Labels != nil {
		issueRequest.Labels = params.Labels // To clear, pass an empty slice
		updated = true
	}
	if params.Milestone != nil { // To remove a milestone, API typically requires sending null or 0
		issueRequest.Milestone = params.Milestone
		updated = true
	}

	if !updated {
		return nil, fmt.Errorf("no fields provided to update issue #%d in %s/%s", params.IssueNumber, owner, repoName)
	}

	issue, _, err := i.githubClient.Issues.Edit(ctx, owner, repoName, params.IssueNumber, issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to edit issue #%d in %s/%s: %w", params.IssueNumber, owner, repoName, err)
	}

	return issue, nil
}

func (i *GithubIntegration) GetIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetIssueParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}
	// params.Repo = repoName // Repo field in GetIssueParams is mostly for internal clarity if needed elsewhere

	if params.IssueNumber == 0 {
		return nil, fmt.Errorf("issue_number is required")
	}

	issue, _, err := i.githubClient.Issues.Get(ctx, owner, repoName, params.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue %d from %s/%s: %w", params.IssueNumber, owner, repoName, err)
	}

	return issue, nil
}

func (i *GithubIntegration) LockIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := LockIssueParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	owner, repoName, err := i.parseOwnerRepo(ctx, params.Owner)
	if err != nil {
		return nil, err
	}

	if params.IssueNumber == 0 {
		return nil, fmt.Errorf("issue_number is required")
	}

	var lockOpts *github.LockIssueOptions
	if params.LockReason != nil && *params.LockReason != "" {
		lockOpts = &github.LockIssueOptions{LockReason: *params.LockReason}
	}

	_, err = i.githubClient.Issues.Lock(ctx, owner, repoName, params.IssueNumber, lockOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to lock issue #%d in %s/%s: %w", params.IssueNumber, owner, repoName, err)
	}

	result := map[string]interface{}{
		"message": fmt.Sprintf("Successfully locked issue #%d in %s/%s", params.IssueNumber, owner, repoName),
	}

	// Lock doesn't return the issue object, just a response.
	return result, nil
}

// --- Placeholder Peek Implementations ---
func (i *GithubIntegration) PeekRepositories(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 100)
	offset := params.GetOffset()

	page := (offset / limit) + 1

	opts := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: limit,
		},
	}

	repos, resp, err := i.githubClient.Repositories.ListByAuthenticatedUser(ctx, opts)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list repositories: %w", err)
	}

	var results []domain.PeekResultItem
	for _, repo := range repos {
		if repo.FullName == nil || repo.Name == nil {
			continue
		}
		results = append(results, domain.PeekResultItem{
			Key:     *repo.FullName,
			Value:   *repo.FullName,
			Content: *repo.Name,
		})
	}

	hasMore := resp.NextPage > 0
	nextOffset := offset + len(results)

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			Offset:  nextOffset,
			HasMore: hasMore,
		},
	}

	result.SetOffset(nextOffset)
	result.SetHasMore(hasMore)

	return result, nil
}

func (i *GithubIntegration) PeekUsers(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 100)
	offset := params.GetOffset()

	var results []domain.PeekResultItem

	if offset == 0 {
		results = append(results, domain.PeekResultItem{
			Key:     "*",
			Value:   "*",
			Content: "Any user",
		})
		results = append(results, domain.PeekResultItem{
			Key:     "none",
			Value:   "none",
			Content: "No assignee/creator",
		})

		user, _, err := i.githubClient.Users.Get(ctx, "")
		if err == nil && user.Login != nil {
			results = append(results, domain.PeekResultItem{
				Key:     *user.Login,
				Value:   *user.Login,
				Content: *user.Login + " (you)",
			})
		}
	}

	orgs, _, err := i.githubClient.Organizations.List(ctx, "", nil)
	if err == nil && len(orgs) > 0 {
		if len(orgs) > 0 && orgs[0].Login != nil {
			page := (offset / limit) + 1

			members, resp, err := i.githubClient.Organizations.ListMembers(ctx, *orgs[0].Login, &github.ListMembersOptions{
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: limit,
				},
			})
			if err == nil {
				for _, member := range members {
					if member.Login != nil {
						found := false
						for _, existing := range results {
							if existing.Key == *member.Login {
								found = true
								break
							}
						}
						if !found {
							results = append(results, domain.PeekResultItem{
								Key:     *member.Login,
								Value:   *member.Login,
								Content: *member.Login,
							})
						}
					}
				}

				hasMore := resp.NextPage > 0
				nextOffset := offset + len(members)

				result := domain.PeekResult{
					Result: results,
					Pagination: domain.PaginationMetadata{
						Offset:  nextOffset,
						HasMore: hasMore,
					},
				}

				result.SetOffset(nextOffset)
				result.SetHasMore(hasMore)

				return result, nil
			}
		}
	}

	return domain.PeekResult{Result: results}, nil
}

func (i *GithubIntegration) PeekBranches(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	// Parse PayloadJSON to get repository_id
	var payload map[string]interface{}
	if len(params.PayloadJSON) > 0 {
		if err := json.Unmarshal(params.PayloadJSON, &payload); err != nil {
			return domain.PeekResult{}, fmt.Errorf("failed to parse payload JSON: %w", err)
		}
	}

	// Try to get repository_id from different sources
	var repositoryIDStr string

	// First try from payload
	if repositoryID, ok := payload["repository_id"]; ok {
		if repoStr, ok := repositoryID.(string); ok && repoStr != "" {
			repositoryIDStr = repoStr
		}
	}

	// If not found in payload, try params.Path
	if repositoryIDStr == "" && params.Path != "" {
		repositoryIDStr = params.Path
	}

	// If still not found, return empty result (no branches available)
	if repositoryIDStr == "" {
		return domain.PeekResult{
			Result: []domain.PeekResultItem{},
		}, nil
	}

	// Parse owner and repository name
	owner, repoName, err := i.parseOwnerRepo(ctx, repositoryIDStr)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to parse repository ID: %w", err)
	}

	// List branches for the repository
	branches, _, err := i.githubClient.Repositories.ListBranches(ctx, owner, repoName, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list branches for %s/%s: %w", owner, repoName, err)
	}

	var results []domain.PeekResultItem
	for _, branch := range branches {
		if branch.Name == nil {
			continue
		}

		results = append(results, domain.PeekResultItem{
			Key:     *branch.Name,
			Value:   *branch.Name,
			Content: *branch.Name,
		})
	}

	return domain.PeekResult{Result: results}, nil
}

// Review Action Params (Pull Request Reviews)
type CreateReviewParams struct {
	Owner      string                        `json:"repository_id"`
	Repo       string                        `json:"-"`
	PullNumber int                           `json:"pull_number"`
	CommitID   *string                       `json:"commit_id,omitempty"`
	Body       *string                       `json:"body,omitempty"`
	Event      *string                       `json:"event,omitempty"`    // APPROVE, REQUEST_CHANGES, or COMMENT
	Comments   *[]*github.DraftReviewComment `json:"comments,omitempty"` // For pending review comments
}

type GetReviewParams struct {
	Owner      string `json:"repository_id"`
	Repo       string `json:"-"`
	PullNumber int    `json:"pull_number"`
	ReviewID   int64  `json:"review_id"`
}

type ListReviewsParams struct {
	Owner      string `json:"repository_id"`
	Repo       string `json:"-"`
	PullNumber int    `json:"pull_number"`
	Limit      *int   `json:"limit,omitempty"` // Maximum number of reviews to return (1-100). Default: 30
	Page       *int   `json:"page,omitempty"`  // Page number of results to fetch. Default: 1
}

type UpdateReviewParams struct {
	Owner      string `json:"repository_id"`
	Repo       string `json:"-"`
	PullNumber int    `json:"pull_number"`
	ReviewID   int64  `json:"review_id"`
	Body       string `json:"body"` // Required
}

// User Action Params
type UserGetRepositoriesParams struct {
	Visibility  *string `json:"visibility,omitempty"`  // all, public, private. Default: all
	Affiliation *string `json:"affiliation,omitempty"` // Comma-separated: owner,collaborator,organization_member. Default: owner,collaborator,organization_member
	Type        *string `json:"type,omitempty"`        // all, owner, public, private, member. Default: all. Caution: Using type with visibility or affiliation is not recommended by GitHub.
	Sort        *string `json:"sort,omitempty"`        // created, updated, pushed, full_name. Default: full_name
	Direction   *string `json:"direction,omitempty"`   // asc, desc. Default: asc for full_name, desc for others.
	Limit       *int    `json:"limit,omitempty"`       // Maximum number of repositories to return (1-100). Default: 30
	Page        *int    `json:"page,omitempty"`        // Page number of results to fetch. Default: 1
}

type UserInviteParams struct {
	Owner      string  `json:"repository_id"`
	Repo       string  `json:"-"`
	Username   string  `json:"username"`
	Permission *string `json:"permission,omitempty"` // pull, push, admin, maintain, triage. Default: push (though API default is read)
}

type CreateFileParams struct {
	Owner   string  `json:"repository_id"`
	Repo    string  `json:"-"`
	Path    string  `json:"path"`
	Message string  `json:"message"`
	Content string  `json:"content"` // Plain text content
	Branch  *string `json:"branch,omitempty"`
	SHA     *string `json:"sha,omitempty"` // Required for UpdateFile (EditFile), optional for CreateFile
}

type DeleteFileParams struct {
	Owner   string  `json:"repository_id"`
	Repo    string  `json:"-"`
	Path    string  `json:"path"`
	Message string  `json:"message"`
	SHA     string  `json:"sha"` // SHA of the file to delete
	Branch  *string `json:"branch,omitempty"`
}

type GetFileParams struct {
	Owner string  `json:"repository_id"`
	Repo  string  `json:"-"`
	Path  string  `json:"path"`
	Ref   *string `json:"ref,omitempty"` // Branch, tag, or commit SHA
}

type ListFilesParams struct {
	Owner string  `json:"repository_id"`
	Repo  string  `json:"-"`
	Path  string  `json:"path,omitempty"` // Optional: directory path, defaults to root
	Ref   *string `json:"ref,omitempty"`  // Branch, tag, or commit SHA
}

// Organization Action Params
type OrgGetRepositoriesParams struct {
	Org       string  `json:"org"`
	Type      *string `json:"type,omitempty"`      // all, public, private, forks, sources, member. Default: all
	Sort      *string `json:"sort,omitempty"`      // created, updated, pushed, full_name. Default: created
	Direction *string `json:"direction,omitempty"` // asc, desc. Default: desc for created, else asc
	Limit     *int    `json:"limit,omitempty"`     // Maximum number of repositories to return (1-100). Default: 30
	Page      *int    `json:"page,omitempty"`      // Page number of results to fetch. Default: 1
}

// Release Action Params
type CreateReleaseParams struct {
	Owner                string  `json:"repository_id"`
	Repo                 string  `json:"-"`
	TagName              string  `json:"tag_name"`
	TargetCommitish      *string `json:"target_commitish,omitempty"`
	Name                 *string `json:"name,omitempty"`
	Body                 *string `json:"body,omitempty"`
	Draft                *bool   `json:"draft,omitempty"`
	Prerelease           *bool   `json:"prerelease,omitempty"`
	GenerateReleaseNotes *bool   `json:"generate_release_notes,omitempty"` // Supported in recent go-github versions
}

type DeleteReleaseParams struct {
	Owner     string `json:"repository_id"`
	Repo      string `json:"-"`
	ReleaseID int64  `json:"release_id"`
}

type GetReleaseParams struct {
	Owner     string `json:"repository_id"`
	Repo      string `json:"-"`
	ReleaseID string `json:"release_id"` // Can be int64 ID, "latest", or "tags/:tag_name"
}

type ListReleasesParams struct {
	Owner string `json:"repository_id"`
	Repo  string `json:"-"`
	Limit *int   `json:"limit,omitempty"` // Maximum number of releases to return (1-100). Default: 30
	Page  *int   `json:"page,omitempty"`  // Page number of results to fetch. Default: 1
}

type UpdateReleaseParams struct {
	Owner                string  `json:"repository_id"`
	Repo                 string  `json:"-"`
	ReleaseID            int64   `json:"release_id"`
	TagName              *string `json:"tag_name,omitempty"`
	TargetCommitish      *string `json:"target_commitish,omitempty"`
	Name                 *string `json:"name,omitempty"`
	Body                 *string `json:"body,omitempty"`
	Draft                *bool   `json:"draft,omitempty"`
	Prerelease           *bool   `json:"prerelease,omitempty"`
	GenerateReleaseNotes *bool   `json:"generate_release_notes,omitempty"`
}

type GetRepositoryIssuesParams struct {
	Owner     string    `json:"repository_id"`
	Repo      string    `json:"-"`
	Milestone *string   `json:"milestone,omitempty"` // Milestone number or "*" for any, "none" for no milestone
	State     *string   `json:"state,omitempty"`     // open, closed, all. Default: open
	Assignee  *string   `json:"assignee,omitempty"`  // User login, "*" for any, "none" for no assignee
	Creator   *string   `json:"creator,omitempty"`   // User login
	Mentioned *string   `json:"mentioned,omitempty"` // User login
	Labels    *[]string `json:"labels,omitempty"`    // List of label names to filter by.
	Sort      *string   `json:"sort,omitempty"`      // created, updated, comments. Default: created
	Direction *string   `json:"direction,omitempty"` // asc, desc. Default: desc
	Since     *string   `json:"since,omitempty"`     // ISO 8601 timestamp (YYYY-MM-DDTHH:MM:SSZ)
	Limit     *int      `json:"limit,omitempty"`     // Maximum number of issues to return (1-100). Default: 30
	Page      *int      `json:"page,omitempty"`      // Page number of results to fetch. Default: 1
}

type GetRepositoryLicenseParams struct {
	Owner string `json:"repository_id"`
	Repo  string `json:"-"`
}
type GetRepositoryPRsParams struct {
	Owner     string  `json:"repository_id"`
	Repo      string  `json:"-"`
	State     *string `json:"state,omitempty"`     // open, closed, all. Default: open
	Head      *string `json:"head,omitempty"`      // Filter by head user:branch
	Base      *string `json:"base,omitempty"`      // Filter by base branch
	Sort      *string `json:"sort,omitempty"`      // created, updated, popularity, long-running. Default: created
	Direction *string `json:"direction,omitempty"` // asc, desc. Default: desc
	Limit     *int    `json:"limit,omitempty"`     // Maximum number of pull requests to return (1-100). Default: 30
	Page      *int    `json:"page,omitempty"`      // Page number of results to fetch. Default: 1
}
