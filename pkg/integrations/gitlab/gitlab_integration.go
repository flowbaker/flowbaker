package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/xanzy/go-gitlab"
)

const (
	GitlabActionType_CreateFile domain.IntegrationActionType = "create_file"
	GitlabActionType_DeleteFile domain.IntegrationActionType = "delete_file"
	GitlabActionType_EditFile   domain.IntegrationActionType = "edit_file"
	GitlabActionType_GetFile    domain.IntegrationActionType = "get_file"
	GitlabActionType_ListFiles  domain.IntegrationActionType = "list_files"

	GitlabActionType_CreateIssue        domain.IntegrationActionType = "create_issue"
	GitlabActionType_CreateIssueComment domain.IntegrationActionType = "create_issue_comment"
	GitlabActionType_EditIssue          domain.IntegrationActionType = "edit_issue"
	GitlabActionType_GetIssue           domain.IntegrationActionType = "get_issue"
	GitlabActionType_LockIssue          domain.IntegrationActionType = "lock_issue"

	GitlabActionType_CreateRelease domain.IntegrationActionType = "create_release"
	GitlabActionType_DeleteRelease domain.IntegrationActionType = "delete_release"
	GitlabActionType_GetRelease    domain.IntegrationActionType = "get_release"
	GitlabActionType_ListReleases  domain.IntegrationActionType = "list_releases"
	GitlabActionType_UpdateRelease domain.IntegrationActionType = "update_release"

	GitlabActionType_GetRepository       domain.IntegrationActionType = "get_repository"
	GitlabActionType_GetRepositoryIssues domain.IntegrationActionType = "get_repository_issues"
	GitlabActionType_UserGetRepositories domain.IntegrationActionType = "user_get_repositories"

	GitlabPeekable_Projects domain.IntegrationPeekableType = "projects"
	GitlabPeekable_Branches domain.IntegrationPeekableType = "branches"
)

type GitlabIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewGitlabIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &GitlabIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *GitlabIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewGitlabIntegration(ctx, GitlabIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

type GitlabIntegration struct {
	client           *gitlab.Client
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	actionManager    *domain.IntegrationActionManager
	peekFuncs        map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type GitlabIntegrationDependencies struct {
	CredentialID     string
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewGitlabIntegration(ctx context.Context, deps GitlabIntegrationDependencies) (*GitlabIntegration, error) {
	integration := &GitlabIntegration{
		binder:           deps.ParameterBinder,
		credentialGetter: deps.CredentialGetter,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(GitlabActionType_CreateFile, integration.CreateFile).
		AddPerItem(GitlabActionType_DeleteFile, integration.DeleteFile).
		AddPerItem(GitlabActionType_EditFile, integration.EditFile).
		AddPerItem(GitlabActionType_GetFile, integration.GetFile).
		AddPerItemMulti(GitlabActionType_ListFiles, integration.ListFiles).
		AddPerItem(GitlabActionType_CreateIssue, integration.CreateIssue).
		AddPerItem(GitlabActionType_CreateIssueComment, integration.CreateIssueComment).
		AddPerItem(GitlabActionType_EditIssue, integration.EditIssue).
		AddPerItem(GitlabActionType_GetIssue, integration.GetIssue).
		AddPerItem(GitlabActionType_LockIssue, integration.LockIssue).
		AddPerItem(GitlabActionType_CreateRelease, integration.CreateRelease).
		AddPerItem(GitlabActionType_DeleteRelease, integration.DeleteRelease).
		AddPerItem(GitlabActionType_GetRelease, integration.GetRelease).
		AddPerItemMulti(GitlabActionType_ListReleases, integration.ListReleases).
		AddPerItem(GitlabActionType_UpdateRelease, integration.UpdateRelease).
		AddPerItem(GitlabActionType_GetRepository, integration.GetRepository).
		AddPerItemMulti(GitlabActionType_GetRepositoryIssues, integration.GetRepositoryIssues).
		AddPerItemMulti(GitlabActionType_UserGetRepositories, integration.UserGetRepositories)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error){
		GitlabPeekable_Projects: integration.PeekProjects,
		GitlabPeekable_Branches: integration.PeekBranches,
	}

	integration.actionManager = actionManager
	integration.peekFuncs = peekFuncs

	if deps.CredentialID == "" {
		return nil, fmt.Errorf("credential ID is required for GitLab integration")
	}

	oauthAccount, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get decrypted GitLab OAuth credential: %w", err)
	}

	client, err := gitlab.NewOAuthClient(oauthAccount.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}
	integration.client = client

	return integration, nil
}

func (i *GitlabIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *GitlabIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	if peekFunc, ok := i.peekFuncs[params.PeekableType]; ok {
		return peekFunc(ctx, params)
	}
	return domain.PeekResult{}, fmt.Errorf("unknown peekable type: %s", params.PeekableType)
}

type CreateFileParams struct {
	ProjectID     string `json:"project_id"`
	FilePath      string `json:"file_path"`
	Branch        string `json:"branch"`
	Content       string `json:"content"`
	CommitMessage string `json:"commit_message"`
}

func (i *GitlabIntegration) CreateFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params CreateFileParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.FilePath == "" || params.Branch == "" {
		return nil, fmt.Errorf("project_id, file_path, and branch are required")
	}

	opts := &gitlab.CreateFileOptions{
		Branch:        gitlab.Ptr(params.Branch),
		Content:       gitlab.Ptr(params.Content),
		CommitMessage: gitlab.Ptr(params.CommitMessage),
	}

	file, _, err := i.client.RepositoryFiles.CreateFile(params.ProjectID, params.FilePath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return file, nil
}

type DeleteFileParams struct {
	ProjectID     string `json:"project_id"`
	FilePath      string `json:"file_path"`
	Branch        string `json:"branch"`
	CommitMessage string `json:"commit_message"`
}

func (i *GitlabIntegration) DeleteFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params DeleteFileParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.FilePath == "" || params.Branch == "" {
		return nil, fmt.Errorf("project_id, file_path, and branch are required")
	}

	opts := &gitlab.DeleteFileOptions{
		Branch:        gitlab.Ptr(params.Branch),
		CommitMessage: gitlab.Ptr(params.CommitMessage),
	}

	_, err := i.client.RepositoryFiles.DeleteFile(params.ProjectID, params.FilePath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	return map[string]interface{}{
		"message":   "file deleted successfully",
		"file_path": params.FilePath,
	}, nil
}

type EditFileParams struct {
	ProjectID     string `json:"project_id"`
	FilePath      string `json:"file_path"`
	Branch        string `json:"branch"`
	Content       string `json:"content"`
	CommitMessage string `json:"commit_message"`
}

func (i *GitlabIntegration) EditFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params EditFileParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.FilePath == "" || params.Branch == "" {
		return nil, fmt.Errorf("project_id, file_path, and branch are required")
	}

	opts := &gitlab.UpdateFileOptions{
		Branch:        gitlab.Ptr(params.Branch),
		Content:       gitlab.Ptr(params.Content),
		CommitMessage: gitlab.Ptr(params.CommitMessage),
	}

	file, _, err := i.client.RepositoryFiles.UpdateFile(params.ProjectID, params.FilePath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to edit file: %w", err)
	}

	return file, nil
}

type GetFileParams struct {
	ProjectID string `json:"project_id"`
	FilePath  string `json:"file_path"`
	Ref       string `json:"ref"`
}

func (i *GitlabIntegration) GetFile(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params GetFileParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.FilePath == "" {
		return nil, fmt.Errorf("project_id and file_path are required")
	}

	opts := &gitlab.GetFileOptions{}
	if params.Ref != "" {
		opts.Ref = gitlab.Ptr(params.Ref)
	}

	file, _, err := i.client.RepositoryFiles.GetFile(params.ProjectID, params.FilePath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return file, nil
}

type ListFilesParams struct {
	ProjectID string `json:"project_id"`
	Path      string `json:"path"`
	Ref       string `json:"ref"`
}

func (i *GitlabIntegration) ListFiles(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var params ListFilesParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	opts := &gitlab.ListTreeOptions{}
	if params.Path != "" {
		opts.Path = gitlab.Ptr(params.Path)
	}
	if params.Ref != "" {
		opts.Ref = gitlab.Ptr(params.Ref)
	}

	trees, _, err := i.client.Repositories.ListTree(params.ProjectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	items := make([]domain.Item, 0, len(trees))
	for _, tree := range trees {
		items = append(items, tree)
	}

	return items, nil
}

type CreateIssueParams struct {
	ProjectID   string   `json:"project_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Assignees   []string `json:"assignees"`
	Labels      []string `json:"labels"`
}

func (i *GitlabIntegration) CreateIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params CreateIssueParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.Title == "" {
		return nil, fmt.Errorf("project_id and title are required")
	}

	opts := &gitlab.CreateIssueOptions{
		Title:       gitlab.Ptr(params.Title),
		Description: gitlab.Ptr(params.Description),
	}

	if len(params.Labels) > 0 {
		opts.Labels = gitlab.Ptr(gitlab.LabelOptions(params.Labels))
	}

	issue, _, err := i.client.Issues.CreateIssue(params.ProjectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return issue, nil
}

type CreateIssueCommentParams struct {
	ProjectID string `json:"project_id"`
	IssueIID  string `json:"issue_iid"`
	Body      string `json:"body"`
}

func (i *GitlabIntegration) CreateIssueComment(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params CreateIssueCommentParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.IssueIID == "" || params.Body == "" {
		return nil, fmt.Errorf("project_id, issue_iid, and body are required")
	}

	issueIID, err := strconv.Atoi(params.IssueIID)
	if err != nil {
		return nil, fmt.Errorf("invalid issue_iid: %w", err)
	}

	opts := &gitlab.CreateIssueNoteOptions{
		Body: gitlab.Ptr(params.Body),
	}

	note, _, err := i.client.Notes.CreateIssueNote(params.ProjectID, issueIID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return note, nil
}

type EditIssueParams struct {
	ProjectID   string   `json:"project_id"`
	IssueIID    string   `json:"issue_iid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	State       string   `json:"state"`
	Labels      []string `json:"labels"`
}

func (i *GitlabIntegration) EditIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params EditIssueParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.IssueIID == "" {
		return nil, fmt.Errorf("project_id and issue_iid are required")
	}

	issueIID, err := strconv.Atoi(params.IssueIID)
	if err != nil {
		return nil, fmt.Errorf("invalid issue_iid: %w", err)
	}

	opts := &gitlab.UpdateIssueOptions{}

	if params.Title != "" {
		opts.Title = gitlab.Ptr(params.Title)
	}
	if params.Description != "" {
		opts.Description = gitlab.Ptr(params.Description)
	}
	if params.State != "" {
		opts.StateEvent = gitlab.Ptr(params.State)
	}
	if len(params.Labels) > 0 {
		opts.Labels = gitlab.Ptr(gitlab.LabelOptions(params.Labels))
	}

	issue, _, err := i.client.Issues.UpdateIssue(params.ProjectID, issueIID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to edit issue: %w", err)
	}

	return issue, nil
}

type GetIssueParams struct {
	ProjectID string `json:"project_id"`
	IssueIID  string `json:"issue_iid"`
}

func (i *GitlabIntegration) GetIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params GetIssueParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.IssueIID == "" {
		return nil, fmt.Errorf("project_id and issue_iid are required")
	}

	issueIID, err := strconv.Atoi(params.IssueIID)
	if err != nil {
		return nil, fmt.Errorf("invalid issue_iid: %w", err)
	}

	issue, _, err := i.client.Issues.GetIssue(params.ProjectID, issueIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	return issue, nil
}

type LockIssueParams struct {
	ProjectID string `json:"project_id"`
	IssueIID  string `json:"issue_iid"`
}

func (i *GitlabIntegration) LockIssue(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params LockIssueParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.IssueIID == "" {
		return nil, fmt.Errorf("project_id and issue_iid are required")
	}

	issueIID, err := strconv.Atoi(params.IssueIID)
	if err != nil {
		return nil, fmt.Errorf("invalid issue_iid: %w", err)
	}

	opts := &gitlab.UpdateIssueOptions{
		DiscussionLocked: gitlab.Ptr(true),
	}

	issue, _, err := i.client.Issues.UpdateIssue(params.ProjectID, issueIID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to lock issue: %w", err)
	}

	return issue, nil
}

type CreateReleaseParams struct {
	ProjectID   string `json:"project_id"`
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Ref         string `json:"ref"`
}

func (i *GitlabIntegration) CreateRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params CreateReleaseParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.TagName == "" {
		return nil, fmt.Errorf("project_id and tag_name are required")
	}

	opts := &gitlab.CreateReleaseOptions{
		TagName:     gitlab.Ptr(params.TagName),
		Name:        gitlab.Ptr(params.Name),
		Description: gitlab.Ptr(params.Description),
	}

	if params.Ref != "" {
		opts.Ref = gitlab.Ptr(params.Ref)
	}

	release, _, err := i.client.Releases.CreateRelease(params.ProjectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}

	return release, nil
}

type DeleteReleaseParams struct {
	ProjectID string `json:"project_id"`
	TagName   string `json:"tag_name"`
}

func (i *GitlabIntegration) DeleteRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params DeleteReleaseParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.TagName == "" {
		return nil, fmt.Errorf("project_id and tag_name are required")
	}

	_, _, err := i.client.Releases.DeleteRelease(params.ProjectID, params.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to delete release: %w", err)
	}

	return map[string]interface{}{
		"message":  "release deleted successfully",
		"tag_name": params.TagName,
	}, nil
}

type GetReleaseParams struct {
	ProjectID string `json:"project_id"`
	TagName   string `json:"tag_name"`
}

func (i *GitlabIntegration) GetRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params GetReleaseParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.TagName == "" {
		return nil, fmt.Errorf("project_id and tag_name are required")
	}

	release, _, err := i.client.Releases.GetRelease(params.ProjectID, params.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}

	return release, nil
}

type ListReleasesParams struct {
	ProjectID string `json:"project_id"`
}

func (i *GitlabIntegration) ListReleases(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var params ListReleasesParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	releases, _, err := i.client.Releases.ListReleases(params.ProjectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	items := make([]domain.Item, 0, len(releases))
	for _, release := range releases {
		items = append(items, release)
	}

	return items, nil
}

type UpdateReleaseParams struct {
	ProjectID   string `json:"project_id"`
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (i *GitlabIntegration) UpdateRelease(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params UpdateReleaseParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" || params.TagName == "" {
		return nil, fmt.Errorf("project_id and tag_name are required")
	}

	opts := &gitlab.UpdateReleaseOptions{}

	if params.Name != "" {
		opts.Name = gitlab.Ptr(params.Name)
	}
	if params.Description != "" {
		opts.Description = gitlab.Ptr(params.Description)
	}

	release, _, err := i.client.Releases.UpdateRelease(params.ProjectID, params.TagName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to update release: %w", err)
	}

	return release, nil
}

type GetRepositoryParams struct {
	ProjectID string `json:"project_id"`
}

func (i *GitlabIntegration) GetRepository(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var params GetRepositoryParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	project, _, err := i.client.Projects.GetProject(params.ProjectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return project, nil
}

type GetRepositoryIssuesParams struct {
	ProjectID string `json:"project_id"`
	State     string `json:"state"`
}

func (i *GitlabIntegration) GetRepositoryIssues(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var params GetRepositoryIssuesParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	opts := &gitlab.ListProjectIssuesOptions{}
	if params.State != "" {
		opts.State = gitlab.Ptr(params.State)
	}

	issues, _, err := i.client.Issues.ListProjectIssues(params.ProjectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository issues: %w", err)
	}

	items := make([]domain.Item, 0, len(issues))
	for _, issue := range issues {
		items = append(items, issue)
	}

	return items, nil
}

type UserGetRepositoriesParams struct {
	Visibility string `json:"visibility"`
	OrderBy    string `json:"order_by"`
}

func (i *GitlabIntegration) UserGetRepositories(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var params UserGetRepositoriesParams
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	opts := &gitlab.ListProjectsOptions{
		Membership: gitlab.Ptr(true),
	}

	if params.Visibility != "" {
		opts.Visibility = gitlab.Ptr(gitlab.VisibilityValue(params.Visibility))
	}
	if params.OrderBy != "" {
		opts.OrderBy = gitlab.Ptr(params.OrderBy)
	}

	projects, _, err := i.client.Projects.ListProjects(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repositories: %w", err)
	}

	items := make([]domain.Item, 0, len(projects))
	for _, project := range projects {
		items = append(items, project)
	}

	return items, nil
}

func (i *GitlabIntegration) PeekProjects(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 100)
	offset := params.Pagination.Offset
	page := (offset / limit) + 1

	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: limit,
			Page:    page,
		},
		Membership: gitlab.Ptr(true),
	}

	projects, resp, err := i.client.Projects.ListProjects(opts)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list projects: %w", err)
	}

	var results []domain.PeekResultItem
	for _, project := range projects {
		results = append(results, domain.PeekResultItem{
			Key:     project.PathWithNamespace,
			Value:   project.PathWithNamespace,
			Content: project.Name,
		})
	}

	hasMore := resp.NextPage > 0
	nextOffset := offset + len(results)

	return domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextOffset: nextOffset,
			HasMore:    hasMore,
		},
	}, nil
}

func (i *GitlabIntegration) PeekBranches(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var payload map[string]interface{}
	if len(params.PayloadJSON) > 0 {
		if err := json.Unmarshal(params.PayloadJSON, &payload); err != nil {
			return domain.PeekResult{}, fmt.Errorf("failed to parse payload JSON: %w", err)
		}
	}

	var projectID string
	if pid, ok := payload["project_id"]; ok {
		if pidStr, ok := pid.(string); ok && pidStr != "" {
			projectID = pidStr
		}
	}

	if projectID == "" {
		return domain.PeekResult{Result: []domain.PeekResultItem{}}, nil
	}

	limit := params.GetLimitWithMax(20, 100)
	offset := params.Pagination.Offset
	page := (offset / limit) + 1

	opts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: limit,
			Page:    page,
		},
	}

	branches, resp, err := i.client.Branches.ListBranches(projectID, opts)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list branches: %w", err)
	}

	var results []domain.PeekResultItem
	for _, branch := range branches {
		results = append(results, domain.PeekResultItem{
			Key:     branch.Name,
			Value:   branch.Name,
			Content: branch.Name,
		})
	}

	hasMore := resp.NextPage > 0
	nextOffset := offset + len(results)

	return domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextOffset: nextOffset,
			HasMore:    hasMore,
		},
	}, nil
}

