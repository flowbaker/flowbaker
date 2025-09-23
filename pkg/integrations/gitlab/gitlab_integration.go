package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLabIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[GitLabCredential]
}

func NewGitLabIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &GitLabIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[GitLabCredential](deps.ExecutorCredentialManager),
	}
}

func (c *GitLabIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewGitLabIntegration(ctx, GitLabIntegrationDependencies{
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
	})
}

type GitLabIntegration struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[GitLabCredential]
	credential       GitLabCredential
	client           *gitlab.Client
	actionManager    *domain.IntegrationActionManager
	peekFuncs        map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type GitLabCredential struct {
	APIToken string `json:"api_token"`
	BaseURL  string `json:"base_url"`
}

type GitLabIntegrationDependencies struct {
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialID     string
	CredentialGetter domain.CredentialGetter[GitLabCredential]
}

func NewGitLabIntegration(ctx context.Context, deps GitLabIntegrationDependencies) (*GitLabIntegration, error) {
	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	if credential.BaseURL == "" {
		credential.BaseURL = "https://gitlab.com"
	}

	var client *gitlab.Client
	if credential.BaseURL == "https://gitlab.com" {
		client, err = gitlab.NewClient(credential.APIToken)
	} else {
		client, err = gitlab.NewClient(credential.APIToken, gitlab.WithBaseURL(credential.BaseURL))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	integration := &GitLabIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
		credential:       credential,
		client:           client,
		actionManager:    domain.NewIntegrationActionManager(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_ListProjects, integration.ListProjects).
		AddPerItem(IntegrationActionType_GetProject, integration.GetProject).
		AddPerItem(IntegrationActionType_CreateIssue, integration.CreateIssue).
		AddPerItem(IntegrationActionType_UpdateIssue, integration.UpdateIssue).
		AddPerItem(IntegrationActionType_ListIssues, integration.ListIssues).
		AddPerItem(IntegrationActionType_AddIssueComment, integration.AddIssueComment).
		AddPerItem(IntegrationActionType_CreateMergeRequest, integration.CreateMergeRequest).
		AddPerItem(IntegrationActionType_ListMergeRequests, integration.ListMergeRequests).
		AddPerItem(IntegrationActionType_GetFileContent, integration.GetFileContent).
		AddPerItem(IntegrationActionType_CreateFile, integration.CreateFile).
		AddPerItem(IntegrationActionType_TriggerPipeline, integration.TriggerPipeline).
		AddPerItem(IntegrationActionType_ListBranches, integration.ListBranches)

	integration.actionManager = actionManager

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		GitLabPeekable_Projects: integration.PeekProjects,
		GitLabPeekable_Branches: integration.PeekBranches,
		GitLabPeekable_Users:    integration.PeekUsers,
	}

	integration.peekFuncs = peekFuncs

	log.Info().Msg("GitLab integration created")

	return integration, nil
}

func (i *GitLabIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

// Parameter structs
type ListProjectsParams struct {
	Owned   bool   `json:"owned"`
	Starred bool   `json:"starred"`
	Search  string `json:"search"`
}

type GetProjectParams struct {
	ProjectID string `json:"project_id"`
}

type CreateIssueParams struct {
	ProjectID   string   `json:"project_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	AssigneeIDs []string `json:"assignee_ids"`
	Labels      []string `json:"labels"`
	MilestoneID int      `json:"milestone_id"`
}

type UpdateIssueParams struct {
	ProjectID   string   `json:"project_id"`
	IssueIID    int      `json:"issue_iid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	StateEvent  string   `json:"state_event"`
	AssigneeIDs []string `json:"assignee_ids"`
	Labels      []string `json:"labels"`
}

type ListIssuesParams struct {
	ProjectID  string   `json:"project_id"`
	State      string   `json:"state"`
	AssigneeID int      `json:"assignee_id"`
	AuthorID   int      `json:"author_id"`
	Labels     []string `json:"labels"`
}

type AddIssueCommentParams struct {
	ProjectID string `json:"project_id"`
	IssueIID  int    `json:"issue_iid"`
	Body      string `json:"body"`
}

type CreateMergeRequestParams struct {
	ProjectID    string   `json:"project_id"`
	Title        string   `json:"title"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Description  string   `json:"description"`
	AssigneeIDs  []string `json:"assignee_ids"`
	ReviewerIDs  []string `json:"reviewer_ids"`
}

type ListMergeRequestsParams struct {
	ProjectID    string `json:"project_id"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

type GetFileContentParams struct {
	ProjectID string `json:"project_id"`
	FilePath  string `json:"file_path"`
	Ref       string `json:"ref"`
}

type CreateFileParams struct {
	ProjectID     string `json:"project_id"`
	FilePath      string `json:"file_path"`
	Content       string `json:"content"`
	CommitMessage string `json:"commit_message"`
	Branch        string `json:"branch"`
}

type TriggerPipelineParams struct {
	ProjectID string            `json:"project_id"`
	Ref       string            `json:"ref"`
	Variables map[string]string `json:"variables"`
}

type ListBranchesParams struct {
	ProjectID string `json:"project_id"`
	Search    string `json:"search"`
}

// Helper function to convert string to project ID
func (i *GitLabIntegration) getProjectID(projectID string) interface{} {
	// Try to parse as integer, if fails use as string (namespace/project)
	if id, err := strconv.Atoi(projectID); err == nil {
		return id
	}
	return projectID
}

// Helper function to convert string slice to int slice for assignee IDs
func stringSliceToIntSlice(strings []string) []int {
	var ints []int
	for _, s := range strings {
		if i, err := strconv.Atoi(s); err == nil {
			ints = append(ints, i)
		}
	}
	return ints
}

// Action implementations
func (i *GitLabIntegration) ListProjects(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListProjectsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	if p.Owned {
		opt.Owned = newBoolPtr(true)
	}

	if p.Starred {
		opt.Starred = newBoolPtr(true)
	}
	if p.Search != "" {
		opt.Search = newStringPtr(p.Search)
	}

	projects, _, err := i.client.Projects.ListProjects(opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return projects, nil
}

func (i *GitLabIntegration) GetProject(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetProjectParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	project, _, err := i.client.Projects.GetProject(i.getProjectID(p.ProjectID), &gitlab.GetProjectOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

func (i *GitLabIntegration) CreateIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.CreateIssueOptions{
		Title: newStringPtr(p.Title),
	}

	if p.Description != "" {
		opt.Description = newStringPtr(p.Description)
	}
	if len(p.AssigneeIDs) > 0 {
		assigneeIDs := stringSliceToIntSlice(p.AssigneeIDs)
		opt.AssigneeIDs = &assigneeIDs
	}
	if len(p.Labels) > 0 {
		labelOptions := make([]string, len(p.Labels))
		copy(labelOptions, p.Labels)

		opt.Labels = (*gitlab.LabelOptions)(&labelOptions)
	}
	if p.MilestoneID > 0 {
		opt.MilestoneID = newIntPtr(p.MilestoneID)
	}

	issue, _, err := i.client.Issues.CreateIssue(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return issue, nil
}

func (i *GitLabIntegration) UpdateIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.UpdateIssueOptions{}

	if p.Title != "" {
		opt.Title = newStringPtr(p.Title)
	}
	if p.Description != "" {
		opt.Description = newStringPtr(p.Description)
	}
	if p.StateEvent != "" {
		opt.StateEvent = newStringPtr(p.StateEvent)
	}
	if len(p.AssigneeIDs) > 0 {
		assigneeIDs := stringSliceToIntSlice(p.AssigneeIDs)
		opt.AssigneeIDs = &assigneeIDs
	}
	if len(p.Labels) > 0 {
		labelOptions := make([]string, len(p.Labels))
		copy(labelOptions, p.Labels)
		opt.Labels = (*gitlab.LabelOptions)(&labelOptions)
	}

	issue, _, err := i.client.Issues.UpdateIssue(i.getProjectID(p.ProjectID), p.IssueIID, opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	return issue, nil
}

func (i *GitLabIntegration) ListIssues(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListIssuesParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.ListProjectIssuesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	if p.State != "" && p.State != "all" {
		opt.State = newStringPtr(p.State)
	}
	if p.AssigneeID > 0 {
		opt.AssigneeID = newIntPtr(p.AssigneeID)
	}
	if p.AuthorID > 0 {
		opt.AuthorID = newIntPtr(p.AuthorID)
	}
	if len(p.Labels) > 0 {
		labelStrings := make([]string, len(p.Labels))
		copy(labelStrings, p.Labels)
		opt.Labels = (*gitlab.LabelOptions)(&labelStrings)
	}

	issues, _, err := i.client.Issues.ListProjectIssues(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	return issues, nil
}

func (i *GitLabIntegration) AddIssueComment(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := AddIssueCommentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.CreateIssueNoteOptions{
		Body: newStringPtr(p.Body),
	}

	note, _, err := i.client.Notes.CreateIssueNote(i.getProjectID(p.ProjectID), p.IssueIID, opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to add issue comment: %w", err)
	}

	return note, nil
}

func (i *GitLabIntegration) CreateMergeRequest(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateMergeRequestParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.CreateMergeRequestOptions{
		Title:        newStringPtr(p.Title),
		SourceBranch: newStringPtr(p.SourceBranch),
		TargetBranch: newStringPtr(p.TargetBranch),
	}

	if p.Description != "" {
		opt.Description = newStringPtr(p.Description)
	}
	if len(p.AssigneeIDs) > 0 {
		assigneeIDs := stringSliceToIntSlice(p.AssigneeIDs)
		opt.AssigneeIDs = &assigneeIDs
	}
	if len(p.ReviewerIDs) > 0 {
		reviewerIDs := stringSliceToIntSlice(p.ReviewerIDs)
		opt.ReviewerIDs = &reviewerIDs
	}

	mr, _, err := i.client.MergeRequests.CreateMergeRequest(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %w", err)
	}

	return mr, nil
}

func (i *GitLabIntegration) ListMergeRequests(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListMergeRequestsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	if p.State != "" && p.State != "all" {
		opt.State = newStringPtr(p.State)
	}
	if p.SourceBranch != "" {
		opt.SourceBranch = newStringPtr(p.SourceBranch)
	}
	if p.TargetBranch != "" {
		opt.TargetBranch = newStringPtr(p.TargetBranch)
	}

	mrs, _, err := i.client.MergeRequests.ListProjectMergeRequests(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list merge requests: %w", err)
	}

	return mrs, nil
}

func (i *GitLabIntegration) GetFileContent(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetFileContentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Ref == "" {
		p.Ref = "main"
	}

	opt := &gitlab.GetFileOptions{
		Ref: newStringPtr(p.Ref),
	}

	file, _, err := i.client.RepositoryFiles.GetFile(i.getProjectID(p.ProjectID), p.FilePath, opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}

	return file, nil
}

func (i *GitLabIntegration) CreateFile(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateFileParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Branch == "" {
		p.Branch = "main"
	}

	opt := &gitlab.CreateFileOptions{
		Branch:        newStringPtr(p.Branch),
		Content:       newStringPtr(p.Content),
		CommitMessage: newStringPtr(p.CommitMessage),
	}

	file, _, err := i.client.RepositoryFiles.CreateFile(i.getProjectID(p.ProjectID), p.FilePath, opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return file, nil
}

func (i *GitLabIntegration) TriggerPipeline(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := TriggerPipelineParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.CreatePipelineOptions{
		Ref: newStringPtr(p.Ref),
	}

	if len(p.Variables) > 0 {
		variables := make([]*gitlab.PipelineVariableOptions, len(p.Variables))

		for key, value := range p.Variables {
			variables = append(variables, &gitlab.PipelineVariableOptions{
				Key:   newStringPtr(key),
				Value: newStringPtr(value),
			})
		}

		opt.Variables = &variables
	}

	pipeline, _, err := i.client.Pipelines.CreatePipeline(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to trigger pipeline: %w", err)
	}

	return pipeline, nil
}

func (i *GitLabIntegration) ListBranches(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListBranchesParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	opt := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	if p.Search != "" {
		opt.Search = newStringPtr(p.Search)
	}

	branches, _, err := i.client.Branches.ListBranches(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	return branches, nil
}

// Peek function implementations
func (i *GitLabIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

func (i *GitLabIntegration) PeekProjects(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	opt := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	projects, _, err := i.client.Projects.ListProjects(opt, gitlab.WithContext(ctx))
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list projects for peek: %w", err)
	}

	var results []domain.PeekResultItem
	for _, project := range projects {
		results = append(results, domain.PeekResultItem{
			Key:     strconv.Itoa(project.ID),
			Value:   strconv.Itoa(project.ID),
			Content: fmt.Sprintf("%s (%s)", project.Name, project.PathWithNamespace),
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekBranchesParams struct {
	ProjectID string `json:"project_id"`
}

func (i *GitLabIntegration) PeekBranches(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	p := PeekBranchesParams{}
	err := json.Unmarshal(params.PayloadJSON, &p)
	if err != nil {
		return domain.PeekResult{}, err
	}

	opt := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	branches, _, err := i.client.Branches.ListBranches(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list branches for peek: %w", err)
	}

	var results []domain.PeekResultItem
	for _, branch := range branches {
		results = append(results, domain.PeekResultItem{
			Key:     branch.Name,
			Value:   branch.Name,
			Content: branch.Name,
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekUsersParams struct {
	ProjectID string `json:"project_id"`
}

func (i *GitLabIntegration) PeekUsers(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	p := PeekUsersParams{}
	err := json.Unmarshal(params.PayloadJSON, &p)
	if err != nil {
		return domain.PeekResult{}, err
	}

	opt := &gitlab.ListProjectMembersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	members, _, err := i.client.ProjectMembers.ListAllProjectMembers(i.getProjectID(p.ProjectID), opt, gitlab.WithContext(ctx))
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list project members for peek: %w", err)
	}

	var results []domain.PeekResultItem
	for _, member := range members {
		results = append(results, domain.PeekResultItem{
			Key:     strconv.Itoa(member.ID),
			Value:   strconv.Itoa(member.ID),
			Content: fmt.Sprintf("%s (@%s)", member.Name, member.Username),
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

func newBoolPtr(b bool) *bool {
	return &b
}

func newIntPtr(i int) *int {
	return &i
}

func newStringPtr(s string) *string {
	return &s
}
