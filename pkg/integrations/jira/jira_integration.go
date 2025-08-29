package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

const (
	IntegrationActionType_GetIssue                domain.IntegrationActionType = "get_issue"
	IntegrationActionType_GetManyIssues           domain.IntegrationActionType = "get_many_issues"
	IntegrationActionType_CreateIssue             domain.IntegrationActionType = "create_issue"
	IntegrationActionType_UpdateIssue             domain.IntegrationActionType = "update_issue"
	IntegrationActionType_DeleteIssue             domain.IntegrationActionType = "delete_issue"
	IntegrationActionType_GetIssueChangelog       domain.IntegrationActionType = "get_issue_changelog"
	IntegrationActionType_GetIssueStatus          domain.IntegrationActionType = "get_issue_status"
	IntegrationActionType_CreateEmailNotification domain.IntegrationActionType = "create_email_notification"
	// Comment actions
	IntegrationActionType_AddComment      domain.IntegrationActionType = "add_comment"
	IntegrationActionType_GetComment      domain.IntegrationActionType = "get_comment"
	IntegrationActionType_GetManyComments domain.IntegrationActionType = "get_many_comments"
	IntegrationActionType_UpdateComment   domain.IntegrationActionType = "update_comment"
	IntegrationActionType_RemoveComment   domain.IntegrationActionType = "remove_comment"
	// User actions
	IntegrationActionType_GetUser domain.IntegrationActionType = "get_user"
)

const (
	JiraIntegrationPeekable_Projects   domain.IntegrationPeekableType = "projects"
	JiraIntegrationPeekable_IssueTypes domain.IntegrationPeekableType = "issue_types"
	JiraIntegrationPeekable_Priorities domain.IntegrationPeekableType = "priorities"
	JiraIntegrationPeekable_Assignees  domain.IntegrationPeekableType = "assignees"
	JiraIntegrationPeekable_Statuses   domain.IntegrationPeekableType = "statuses"
	JiraIntegrationPeekable_Issues     domain.IntegrationPeekableType = "issues"
)

type JiraIntegrationCreator struct {
	credentialGetter          domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder                    domain.IntegrationParameterBinder
	executorCredentialManager domain.ExecutorCredentialManager
}

func NewJiraIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &JiraIntegrationCreator{
		credentialGetter:          managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		binder:                    deps.ParameterBinder,
		executorCredentialManager: deps.ExecutorCredentialManager,
	}
}

func (c *JiraIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewJiraIntegration(ctx,
		JiraIntegrationDependencies{
			CredentialGetter:          c.credentialGetter,
			ParameterBinder:           c.binder,
			CredentialID:              p.CredentialID,
			ExecutorCredentialManager: c.executorCredentialManager,
		})
}

type JiraIntegration struct {
	binder                    domain.IntegrationParameterBinder
	credentialGetter          domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	executorCredentialManager domain.ExecutorCredentialManager

	jiraClient *jira.Client

	actionManager *domain.IntegrationActionManager
	httpClient    *http.Client
	credentialID  string

	peekFuncs map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error)
}

type JiraIntegrationDependencies struct {
	ParameterBinder           domain.IntegrationParameterBinder
	CredentialID              string
	CredentialGetter          domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	ExecutorCredentialManager domain.ExecutorCredentialManager
}

func NewJiraIntegration(ctx context.Context, deps JiraIntegrationDependencies) (*JiraIntegration, error) {
	integration := &JiraIntegration{
		credentialGetter:          deps.CredentialGetter,
		binder:                    deps.ParameterBinder,
		credentialID:              deps.CredentialID,
		executorCredentialManager: deps.ExecutorCredentialManager,
		actionManager:             domain.NewIntegrationActionManager(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize the Jira client like Discord initializes session
	_, err := integration.initializeJiraClient(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize Jira client during construction")
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_GetIssue, integration.GetIssue).
		AddPerItemMulti(IntegrationActionType_GetManyIssues, integration.GetManyIssues).
		AddPerItem(IntegrationActionType_CreateIssue, integration.CreateIssue).
		AddPerItem(IntegrationActionType_UpdateIssue, integration.UpdateIssue).
		AddPerItem(IntegrationActionType_DeleteIssue, integration.DeleteIssue).
		AddPerItem(IntegrationActionType_GetIssueChangelog, integration.GetIssueChangelog).
		AddPerItem(IntegrationActionType_GetIssueStatus, integration.GetIssueStatus).
		AddPerItem(IntegrationActionType_CreateEmailNotification, integration.CreateEmailNotification).
		// Comment actions
		AddPerItem(IntegrationActionType_AddComment, integration.AddComment).
		AddPerItem(IntegrationActionType_GetComment, integration.GetComment).
		AddPerItemMulti(IntegrationActionType_GetManyComments, integration.GetManyComments).
		AddPerItem(IntegrationActionType_UpdateComment, integration.UpdateComment).
		AddPerItem(IntegrationActionType_RemoveComment, integration.RemoveComment).
		// User actions
		AddPerItem(IntegrationActionType_GetUser, integration.GetUser)

	integration.actionManager = actionManager

	integration.setupPeekFunctions()

	return integration, nil
}

// setupPeekFunctions initializes the peek functions map
func (i *JiraIntegration) setupPeekFunctions() {
	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		JiraIntegrationPeekable_Projects:   i.PeekProjects,
		JiraIntegrationPeekable_IssueTypes: i.PeekIssueTypes,
		JiraIntegrationPeekable_Priorities: i.PeekPriorities,
		JiraIntegrationPeekable_Assignees:  i.PeekAssignees,
		JiraIntegrationPeekable_Statuses:   i.PeekStatuses,
		JiraIntegrationPeekable_Issues:     i.PeekIssues,
	}

	i.peekFuncs = peekFuncs
}

// getOAuthCredentials gets OAuth credentials for Jira using the proper OAuth credential service
func (i *JiraIntegration) getOAuthCredentials(ctx context.Context) (string, string, error) {
	// Use the credential service to get the OAuth account with sensitive data
	// This is the correct way to handle OAuth credentials
	credential, err := i.executorCredentialManager.GetFullCredential(ctx, i.credentialID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get OAuth credentials: %w", err)
	}

	oauthAccount, err := i.executorCredentialManager.GetOAuthAccount(ctx, credential.OAuthAccountID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get OAuth account: %w", err)
	}

	sensitiveData := domain.OAuthAccountSensitiveData{}

	sensitiveDataBytes, err := json.Marshal(credential.DecryptedPayload)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal credential: %w", err)
	}

	err = json.Unmarshal(sensitiveDataBytes, &sensitiveData)
	if err != nil {
		return "", "", fmt.Errorf("failed to unmarshal credential: %w", err)
	}

	// Get the first accessible resource URL from metadata
	accessibleResources, ok := oauthAccount.Metadata["accessible_resources"].([]interface{})
	if !ok || len(accessibleResources) == 0 {
		// Fallback: Try to get accessible resources using the current access token
		retrievedResources, err := i.getAccessibleResourcesWithToken(ctx, sensitiveData.AccessToken)
		if err != nil {
			return "", "", fmt.Errorf("no accessible Jira resources found in OAuth account metadata and failed to retrieve them: %w", err)
		}

		if len(retrievedResources) == 0 {
			return "", "", fmt.Errorf("no accessible Jira resources available for this OAuth account")
		}

		// Update the metadata with the retrieved accessible resources
		oauthAccount.Metadata["accessible_resources"] = retrievedResources

		// Use the retrieved resources for the rest of the function
		accessibleResources = retrievedResources

		// Save the updated metadata back to the OAuth account using SaveAccountByName
		err = i.executorCredentialManager.UpdateOAuthAccountMetadata(ctx, oauthAccount.ID, oauthAccount.Metadata)
		if err != nil {
			// Log error but don't fail the operation
			log.Warn().Err(err).Msg("Failed to update OAuth account metadata")
		}
	}

	// Double-check that we have accessible resources before accessing the first element
	if len(accessibleResources) == 0 {
		return "", "", fmt.Errorf("no accessible Jira resources available after retrieval attempt")
	}

	firstResource, ok := accessibleResources[0].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("invalid resource format in accessible resources")
	}

	baseURL, ok := firstResource["url"].(string)
	if !ok {
		return "", "", fmt.Errorf("no URL found in first accessible resource")
	}

	return sensitiveData.AccessToken, baseURL, nil
}

// getAccessibleResourcesWithToken retrieves accessible resources using an access token
func (i *JiraIntegration) getAccessibleResourcesWithToken(ctx context.Context, accessToken string) ([]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.atlassian.com/oauth/token/accessible-resources", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get accessible resources: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var resources []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, err
	}

	return resources, nil
}

// getCloudIDFromMetadata extracts the cloud ID from OAuth account metadata
func (i *JiraIntegration) getCloudIDFromMetadata(ctx context.Context) (string, error) {
	credential, err := i.executorCredentialManager.GetFullCredential(ctx, i.credentialID)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth credentials: %w", err)
	}

	oauthAccount, err := i.executorCredentialManager.GetOAuthAccount(ctx, credential.OAuthAccountID)
	if err != nil {
		return "", fmt.Errorf("failed to get OAuth account: %w", err)
	}

	// Parse the structured metadata
	metadataBytes, err := json.Marshal(oauthAccount.Metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var jiraMetadata JiraOAuthMetadata
	if err := json.Unmarshal(metadataBytes, &jiraMetadata); err != nil {
		return "", fmt.Errorf("failed to unmarshal Jira metadata: %w", err)
	}

	// First, check if user selected a specific cloud ID during OAuth
	if jiraMetadata.SelectedCloudID != "" {
		log.Info().Str("selected_cloud_id", jiraMetadata.SelectedCloudID).Msg("Using user-selected cloud ID from OAuth")
		return jiraMetadata.SelectedCloudID, nil
	}

	// Fallback to first accessible resource if no specific cloud ID was selected
	if len(jiraMetadata.AccessibleResources) > 0 {
		cloudID := jiraMetadata.AccessibleResources[0].ID
		log.Info().Str("fallback_cloud_id", cloudID).Msg("Using first accessible resource as fallback")
		return cloudID, nil
	}

	// If no resources in metadata, try to retrieve them directly
	sensitiveData := domain.OAuthAccountSensitiveData{}
	sensitiveDataBytes, err := json.Marshal(credential.DecryptedPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal credential: %w", err)
	}

	err = json.Unmarshal(sensitiveDataBytes, &sensitiveData)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal credential: %w", err)
	}

	retrievedResources, err := i.getAccessibleResourcesWithToken(ctx, sensitiveData.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve accessible resources for cloud ID: %w", err)
	}

	if len(retrievedResources) == 0 {
		return "", fmt.Errorf("no accessible resources available for cloud ID")
	}

	// Parse the first retrieved resource
	var jiraResources []JiraAccessibleResource
	resourcesBytes, err := json.Marshal(retrievedResources)
	if err != nil {
		return "", fmt.Errorf("failed to marshal retrieved resources: %w", err)
	}

	err = json.Unmarshal(resourcesBytes, &jiraResources)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal retrieved resources: %w", err)
	}

	if len(jiraResources) == 0 {
		return "", fmt.Errorf("no valid Jira resources found")
	}

	cloudID := jiraResources[0].ID
	log.Info().Str("fallback_cloud_id", cloudID).Msg("Using first retrieved resource as fallback")
	return cloudID, nil
}

// initializeJiraClient creates and configures the Jira client
func (i *JiraIntegration) initializeJiraClient(ctx context.Context) (*jira.Client, error) {
	accessToken, _, err := i.getOAuthCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth credentials: %w", err)
	}

	// Extract cloud ID from accessible resources metadata
	cloudID, err := i.getCloudIDFromMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud ID: %w", err)
	}

	// Create OAuth transport
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	})

	httpClient := oauth2.NewClient(ctx, tokenSource)

	// For OAuth 2.0 (3LO), use the Atlassian API gateway directly
	// The go-jira library will append the API paths to this base URL
	apiBaseURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s", cloudID)

	// Create the Jira client with the OAuth client
	client, err := jira.NewClient(httpClient, apiBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}

	i.jiraClient = client
	return client, nil
}

func (i *JiraIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

// Helper method to create authenticated HTTP request using OAuth 2.0 API gateway
func (i *JiraIntegration) createAuthenticatedRequest(ctx context.Context, credentialID, method, url string, body io.Reader) (*http.Request, error) {
	accessToken, _, err := i.getOAuthCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth credentials: %w", err)
	}

	// Get cloud ID for OAuth 2.0 API gateway
	cloudID, err := i.getCloudIDFromMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud ID: %w", err)
	}

	// For OAuth 2.0 (3LO), all requests must go through api.atlassian.com
	// Convert the Jira API path to the OAuth 2.0 gateway format
	fullURL := fmt.Sprintf("https://api.atlassian.com/ex/jira/%s%s", cloudID, url)

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	// Create OAuth header
	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	return req, nil
}

// Parameter structs
type GetIssueParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
}

type GetManyIssuesParams struct {
	CredentialID string `json:"credential_id"`
	JQLQuery     string `json:"jql_query"`
	MaxResults   string `json:"max_results"`
}

type CreateIssueParams struct {
	CredentialID string `json:"credential_id"`
	ProjectKey   string `json:"project_key"`
	IssueType    string `json:"issue_type"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	Priority     string `json:"priority"`
	Assignee     string `json:"assignee"`
	ParentKey    string `json:"parent_key"` // For subtasks
}

type UpdateIssueParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	Priority     string `json:"priority"`
	Assignee     string `json:"assignee"`
	Status       string `json:"status"`
}

type DeleteIssueParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
}

type GetIssueChangelogParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
}

type GetIssueStatusParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
}

type CreateEmailNotificationParams struct {
	CredentialID string   `json:"credential_id"`
	IssueKey     string   `json:"issue_key"`
	Recipients   []string `json:"recipients"`
	Subject      string   `json:"subject"`
	Message      string   `json:"message"`
}

// Comment parameter structs
type AddCommentParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
	Body         string `json:"body"`
}

type GetCommentParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
	CommentID    string `json:"comment_id"`
}

type GetManyCommentsParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
	MaxResults   string `json:"max_results"`
}

type UpdateCommentParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
	CommentID    string `json:"comment_id"`
	Body         string `json:"body"`
}

type RemoveCommentParams struct {
	CredentialID string `json:"credential_id"`
	IssueKey     string `json:"issue_key"`
	CommentID    string `json:"comment_id"`
}

// User parameter structs
type GetUserParams struct {
	CredentialID string `json:"credential_id"`
	AccountID    string `json:"account_id"`
	Username     string `json:"username"`
	EmailAddress string `json:"email_address"`
}

// Action implementations
func (i *JiraIntegration) GetIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" {
		return nil, fmt.Errorf("issue key is required")
	}

	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}

	issue, _, err := i.jiraClient.Issue.Get(p.IssueKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	return issue, nil
}

func (i *JiraIntegration) GetManyIssues(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetManyIssuesParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.JQLQuery) == "" {
		return nil, fmt.Errorf("JQL query is required")
	}

	maxResults := 50
	if p.MaxResults != "" {
		if parsed, err := strconv.Atoi(p.MaxResults); err == nil {
			maxResults = parsed
			if maxResults > 100 {
				maxResults = 100
			}
			if maxResults < 1 {
				maxResults = 1
			}
		}
	}

	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}

	searchOptions := &jira.SearchOptions{
		MaxResults: maxResults,
	}

	issues, _, err := i.jiraClient.Issue.Search(p.JQLQuery, searchOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	// Convert issues to []domain.Item
	items := make([]domain.Item, len(issues))
	for i, issue := range issues {
		items[i] = issue
	}

	return items, nil
}

func (i *JiraIntegration) CreateIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ProjectKey) == "" || strings.TrimSpace(p.IssueType) == "" || strings.TrimSpace(p.Summary) == "" {
		return nil, fmt.Errorf("project key, issue type, and summary are required")
	}

	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}

	// Build issue using go-jira types
	issue := &jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: p.ProjectKey,
			},
			Type: jira.IssueType{
				Name: p.IssueType,
			},
			Summary: p.Summary,
		},
	}

	if p.Description != "" {
		issue.Fields.Description = p.Description
	}

	if p.Priority != "" && p.Priority != "_no_priorities_" && p.Priority != "_no_global_priorities_" {
		issue.Fields.Priority = &jira.Priority{
			Name: p.Priority,
		}
	}

	if p.Assignee != "" {
		// For Jira Cloud, we need to use AccountID instead of EmailAddress
		// The PeekAssignees function returns email addresses, so we need to convert
		if strings.Contains(p.Assignee, "@") {
			// It's an email address, need to find the account ID
			accountIDs, err := i.getUserAccountIds(ctx, p.CredentialID, []string{p.Assignee})
			if err == nil && len(accountIDs) > 0 {
				issue.Fields.Assignee = &jira.User{
					AccountID: accountIDs[0],
				}
			} else {
				// Fallback: try using it as AccountID directly
				issue.Fields.Assignee = &jira.User{
					AccountID: p.Assignee,
				}
			}
		} else {
			// Assume it's already an AccountID
			issue.Fields.Assignee = &jira.User{
				AccountID: p.Assignee,
			}
		}
	}

	// Handle parent issue for subtasks
	// Only set parent if issue type is a subtask variant
	if p.ParentKey != "" && isSubtaskType(p.IssueType) {
		issue.Fields.Parent = &jira.Parent{
			Key: p.ParentKey,
		}
	}

	createdIssue, _, err := i.jiraClient.Issue.Create(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return createdIssue, nil
}

func (i *JiraIntegration) UpdateIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" {
		return nil, fmt.Errorf("issue key is required")
	}

	// Get or create Jira client (Discord pattern)
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira client: %w", err)
	}

	// Build update fields
	updateFields := &jira.IssueFields{}
	hasUpdates := false

	if p.Summary != "" {
		updateFields.Summary = p.Summary
		hasUpdates = true
	}

	if p.Description != "" {
		updateFields.Description = p.Description
		hasUpdates = true
	}

	if p.Priority != "" && p.Priority != "_no_priorities_" && p.Priority != "_no_global_priorities_" {
		updateFields.Priority = &jira.Priority{
			Name: p.Priority,
		}
		hasUpdates = true
	}

	if p.Assignee != "" {
		// For Jira Cloud, we need to use AccountID instead of EmailAddress
		// The PeekAssignees function returns email addresses, so we need to convert
		if strings.Contains(p.Assignee, "@") {
			// It's an email address, need to find the account ID
			accountIDs, err := i.getUserAccountIds(ctx, p.CredentialID, []string{p.Assignee})
			if err == nil && len(accountIDs) > 0 {
				updateFields.Assignee = &jira.User{
					AccountID: accountIDs[0],
				}
			} else {
				// Fallback: try using it as AccountID directly
				updateFields.Assignee = &jira.User{
					AccountID: p.Assignee,
				}
			}
		} else {
			// Assume it's already an AccountID
			updateFields.Assignee = &jira.User{
				AccountID: p.Assignee,
			}
		}
		hasUpdates = true
	}

	// Update fields if there are any
	if hasUpdates {
		updateIssue := &jira.Issue{
			Key:    p.IssueKey,
			Fields: updateFields,
		}
		_, _, err = i.jiraClient.Issue.Update(updateIssue)
		if err != nil {
			return nil, fmt.Errorf("failed to update issue fields: %w", err)
		}
	}

	// Handle status updates using transitions (like Discord pattern)
	if p.Status != "" && p.Status != "_no_statuses_" {
		err := i.updateIssueStatusWithClient(i.jiraClient, p.IssueKey, p.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to update issue status: %w", err)
		}
	}

	return map[string]interface{}{
		"success": true,
		"message": "Issue updated successfully",
	}, nil
}

func (i *JiraIntegration) DeleteIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" {
		return nil, fmt.Errorf("issue key is required")
	}

	// Get or create Jira client (Discord pattern)
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira client: %w", err)
	}

	_, err = i.jiraClient.Issue.Delete(p.IssueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to delete issue: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"message": "Issue deleted successfully",
	}, nil
}

func (i *JiraIntegration) GetIssueChangelog(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetIssueChangelogParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" {
		return nil, fmt.Errorf("issue key is required")
	}

	req, err := i.createAuthenticatedRequest(ctx, p.CredentialID, "GET",
		fmt.Sprintf("/rest/api/3/issue/%s?expand=changelog", p.IssueKey), nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get issue changelog: %s - %s", resp.Status, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Extract just the changelog part
	if changelog, exists := result["changelog"]; exists {
		return changelog, nil
	}

	// Return nil if no changelog found
	return nil, nil
}

func (i *JiraIntegration) GetIssueStatus(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetIssueStatusParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" {
		return nil, fmt.Errorf("issue key is required")
	}

	req, err := i.createAuthenticatedRequest(ctx, p.CredentialID, "GET",
		fmt.Sprintf("/rest/api/3/issue/%s?fields=status", p.IssueKey), nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get issue status: %s - %s", resp.Status, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Extract just the status part
	if fields, exists := result["fields"].(map[string]interface{}); exists {
		if status, exists := fields["status"]; exists {
			return status, nil
		}
	}

	return map[string]interface{}{
		"status": "unknown",
	}, nil
}

func (i *JiraIntegration) CreateEmailNotification(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}

	p := CreateEmailNotificationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" || len(p.Recipients) == 0 ||
		strings.TrimSpace(p.Subject) == "" || strings.TrimSpace(p.Message) == "" {
		return nil, fmt.Errorf("issue key, recipients, subject, and message are required")
	}

	// Parse recipients (email addresses from tag input)
	var cleanEmails []string
	for _, recipient := range p.Recipients {
		if clean := strings.TrimSpace(recipient); clean != "" && strings.Contains(clean, "@") {
			cleanEmails = append(cleanEmails, clean)
		}
	}

	if len(cleanEmails) == 0 {
		return nil, fmt.Errorf("at least one valid email address is required")
	}

	// Convert emails to accountIds
	userAccountIds, err := i.getUserAccountIds(ctx, p.CredentialID, cleanEmails)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve user account IDs: %w", err)
	}

	// Build users array with accountId format
	var users []map[string]interface{}
	for _, accountId := range userAccountIds {
		users = append(users, map[string]interface{}{
			"accountId": accountId,
		})
	}

	notificationData := map[string]interface{}{
		"subject":  p.Subject,
		"textBody": p.Message,
		"to": map[string]interface{}{
			"users": users,
		},
	}

	bodyBytes, err := json.Marshal(notificationData)
	if err != nil {
		return nil, err
	}

	req, err := i.createAuthenticatedRequest(ctx, p.CredentialID, "POST",
		fmt.Sprintf("/rest/api/3/issue/%s/notify", p.IssueKey), bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create email notification: %s - %s", resp.Status, string(bodyBytes))
	}

	return map[string]interface{}{
		"success":    true,
		"message":    "Email notification sent successfully",
		"recipients": cleanEmails,
	}, nil
}

// Comment Action implementations
func (i *JiraIntegration) AddComment(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := AddCommentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" || strings.TrimSpace(p.Body) == "" {
		return nil, fmt.Errorf("issue key and body are required")
	}

	// Get or create Jira client
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira client: %w", err)
	}

	// Create comment using go-jira library
	comment := &jira.Comment{
		Body: p.Body,
	}

	createdComment, _, err := i.jiraClient.Issue.AddComment(p.IssueKey, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment: %w", err)
	}

	return createdComment, nil
}

func (i *JiraIntegration) GetComment(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetCommentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" || strings.TrimSpace(p.CommentID) == "" {
		return nil, fmt.Errorf("issue key and comment ID are required")
	}

	// For individual comment retrieval, we need to use manual HTTP request
	// as go-jira doesn't have a direct method for getting a single comment
	req, err := i.createAuthenticatedRequest(ctx, p.CredentialID, "GET",
		fmt.Sprintf("/rest/api/3/issue/%s/comment/%s", p.IssueKey, p.CommentID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get comment: %s - %s", resp.Status, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func (i *JiraIntegration) GetManyComments(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetManyCommentsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" {
		return nil, fmt.Errorf("issue key is required")
	}

	// Parse max results for Jira API pagination
	maxResults := 50 // Default
	if p.MaxResults != "" {
		if parsed, err := strconv.Atoi(p.MaxResults); err == nil {
			maxResults = parsed
			if maxResults > 5000 { // Jira's maximum limit
				maxResults = 5000
			}
		}
	}

	// Use direct API call to support pagination instead of go-jira library's expand
	// which doesn't support maxResults for comments
	apiURL := fmt.Sprintf("/rest/api/3/issue/%s/comment?maxResults=%d", p.IssueKey, maxResults)
	req, err := i.createAuthenticatedRequest(ctx, p.CredentialID, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get comments: %s - %s", resp.Status, string(bodyBytes))
	}

	var commentsResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&commentsResponse); err != nil {
		return nil, err
	}

	// Extract comments from response
	if comments, ok := commentsResponse["comments"].([]interface{}); ok {
		items := make([]domain.Item, len(comments))
		for i, comment := range comments {
			items[i] = comment
		}
		return items, nil
	}

	// Return empty slice if no comments found
	return []domain.Item{}, nil
}

func (i *JiraIntegration) UpdateComment(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateCommentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" || strings.TrimSpace(p.CommentID) == "" || strings.TrimSpace(p.Body) == "" {
		return nil, fmt.Errorf("issue key, comment ID, and body are required")
	}

	// Get or create Jira client
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira client: %w", err)
	}

	// Create comment with updated body
	comment := &jira.Comment{
		ID:   p.CommentID,
		Body: p.Body,
	}

	updatedComment, _, err := i.jiraClient.Issue.UpdateComment(p.IssueKey, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	return updatedComment, nil
}

func (i *JiraIntegration) RemoveComment(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := RemoveCommentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.IssueKey) == "" || strings.TrimSpace(p.CommentID) == "" {
		return nil, fmt.Errorf("issue key and comment ID are required")
	}

	// Get or create Jira client
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira client: %w", err)
	}

	// Delete comment using go-jira library
	err = i.jiraClient.Issue.DeleteComment(p.IssueKey, p.CommentID)
	if err != nil {
		return nil, fmt.Errorf("failed to remove comment: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"message": "Comment removed successfully",
	}, nil
}

// User Action implementations

func (i *JiraIntegration) GetUser(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetUserParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Get or create Jira client
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira client: %w", err)
	}

	var user *jira.User
	var response *jira.Response

	if p.AccountID != "" {
		// Get user by account ID using go-jira library
		user, response, err = i.jiraClient.User.GetByAccountID(p.AccountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user by account ID: %w", err)
		}
	} else if p.Username != "" {
		// Get user by username using go-jira library
		user, response, err = i.jiraClient.User.Get(p.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to get user by username: %w", err)
		}
	} else if p.EmailAddress != "" {
		// Search for user by email using go-jira library with enhanced error handling
		users, _, searchErr := i.jiraClient.User.Find(p.EmailAddress)
		if searchErr != nil {
			// If direct search fails, try alternative search method
			// Fallback: Use direct HTTP request for user search
			searchURL := fmt.Sprintf("/rest/api/3/user/search?query=%s", url.QueryEscape(p.EmailAddress))
			req, reqErr := i.createAuthenticatedRequest(ctx, i.credentialID, "GET", searchURL, nil)
			if reqErr != nil {
				return nil, fmt.Errorf("failed to create user search request: %w", reqErr)
			}

			resp, respErr := i.httpClient.Do(req)
			if respErr != nil {
				return nil, fmt.Errorf("failed to search user by email: %w", respErr)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				return nil, fmt.Errorf("failed to search user by email: %s - %s", resp.Status, string(bodyBytes))
			}

			var searchResults []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
				return nil, fmt.Errorf("failed to decode user search results: %w", err)
			}

			// Find exact email match in search results
			for _, userResult := range searchResults {
				if email, ok := userResult["emailAddress"].(string); ok && email == p.EmailAddress {
					return userResult, nil
				}
			}
			return nil, fmt.Errorf("user not found with email: %s", p.EmailAddress)
		}

		// Find exact email match from go-jira search
		for _, u := range users {
			if u.EmailAddress == p.EmailAddress {
				return u, nil
			}
		}
		return nil, fmt.Errorf("user not found with email: %s", p.EmailAddress)
	} else {
		return nil, fmt.Errorf("account ID, username, or email address is required")
	}

	// Check if user was found
	if response != nil && response.StatusCode == 404 {
		return nil, fmt.Errorf("user not found")
	}

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// Peek interface implementation
func (i *JiraIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

// PeekProjects fetches available projects from Jira
func (i *JiraIntegration) PeekProjects(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	req, err := i.createAuthenticatedRequest(ctx, i.credentialID, "GET", "/rest/api/3/project", nil)
	if err != nil {
		return domain.PeekResult{}, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return domain.PeekResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return domain.PeekResult{}, fmt.Errorf("failed to get projects: %s - %s", resp.Status, string(bodyBytes))
	}

	var projects []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return domain.PeekResult{}, err
	}

	var results []domain.PeekResultItem
	for _, project := range projects {
		key, keyOk := project["key"].(string)
		name, nameOk := project["name"].(string)

		if keyOk && nameOk {
			results = append(results, domain.PeekResultItem{
				Key:     key,
				Value:   key,
				Content: fmt.Sprintf("%s (%s)", name, key),
			})
		}
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekIssueTypesParams struct {
	ProjectKey string `json:"project_key"`
}

// PeekIssueTypes fetches available issue types for a specific project
func (i *JiraIntegration) PeekIssueTypes(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	params := PeekIssueTypesParams{}
	err := json.Unmarshal(p.PayloadJSON, &params)
	if err != nil {
		return domain.PeekResult{}, err
	}

	if params.ProjectKey == "" {
		return domain.PeekResult{}, fmt.Errorf("project key is required")
	}

	// Get issue types for the specific project
	apiURL := fmt.Sprintf("/rest/api/3/project/%s", params.ProjectKey)
	req, err := i.createAuthenticatedRequest(ctx, i.credentialID, "GET", apiURL, nil)
	if err != nil {
		return domain.PeekResult{}, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return domain.PeekResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return domain.PeekResult{}, fmt.Errorf("failed to get project details: %s - %s", resp.Status, string(bodyBytes))
	}

	var project map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return domain.PeekResult{}, err
	}

	var results []domain.PeekResultItem

	// Extract issue types from project details
	if issueTypes, exists := project["issueTypes"].([]interface{}); exists {
		for _, issueTypeInterface := range issueTypes {
			if issueType, ok := issueTypeInterface.(map[string]interface{}); ok {
				name, nameOk := issueType["name"].(string)

				if nameOk {
					results = append(results, domain.PeekResultItem{
						Key:     name,
						Value:   name,
						Content: name,
					})
				}
			}
		}
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekPrioritiesParams struct {
	ProjectKey string `json:"project_key"`
}

// PeekPriorities fetches available priorities from Jira
func (i *JiraIntegration) PeekPriorities(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	params := PeekPrioritiesParams{}
	if err := json.Unmarshal(p.PayloadJSON, &params); err != nil {
		params.ProjectKey = "" // Fallback to global priorities
	}

	resp, err := i.fetchPrioritiesResponse(ctx, params.ProjectKey)
	if err != nil {
		return domain.PeekResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return domain.PeekResult{}, fmt.Errorf("failed to get priorities: %s - %s", resp.Status, string(bodyBytes))
	}

	var results []domain.PeekResultItem
	if params.ProjectKey != "" {
		results, err = i.extractProjectSpecificPriorities(resp.Body)
	} else {
		results, err = i.extractGlobalPriorities(resp.Body)
	}

	if err != nil {
		return domain.PeekResult{}, err
	}

	return domain.PeekResult{Result: results}, nil
}

// fetchPrioritiesResponse gets the HTTP response for priorities based on project key
func (i *JiraIntegration) fetchPrioritiesResponse(ctx context.Context, projectKey string) (*http.Response, error) {
	var apiURL string
	if projectKey != "" {
		apiURL = fmt.Sprintf("/rest/api/3/issue/createmeta?projectKeys=%s&expand=projects.issuetypes.fields", url.QueryEscape(projectKey))
	} else {
		apiURL = "/rest/api/3/priority"
	}

	req, err := i.createAuthenticatedRequest(ctx, i.credentialID, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	return i.httpClient.Do(req)
}

// extractProjectSpecificPriorities extracts priorities from create meta response
func (i *JiraIntegration) extractProjectSpecificPriorities(body io.Reader) ([]domain.PeekResultItem, error) {
	// Limit response size to prevent DoS
	limitedReader := io.LimitReader(body, 10*1024*1024) // 10MB max

	var createMeta map[string]interface{}
	if err := json.NewDecoder(limitedReader).Decode(&createMeta); err != nil {
		return nil, err
	}

	prioritySet := make(map[string]bool)
	projects := i.getProjects(createMeta)

	// Limit processing to prevent excessive iterations
	const maxProjects = 10
	const maxIssueTypesPerProject = 20
	const maxPrioritiesPerIssueType = 50

	processedProjects := 0
	for _, project := range projects {
		if processedProjects >= maxProjects {
			break
		}
		processedProjects++

		issueTypes := i.getIssueTypes(project)
		processedIssueTypes := 0
		for _, issueType := range issueTypes {
			if processedIssueTypes >= maxIssueTypesPerProject {
				break
			}
			processedIssueTypes++

			priorities := i.getPrioritiesFromIssueType(issueType)
			processedPriorities := 0
			for _, priority := range priorities {
				if processedPriorities >= maxPrioritiesPerIssueType {
					break
				}
				processedPriorities++
				prioritySet[priority] = true
			}
		}
	}

	results := i.convertPrioritySetToResults(prioritySet)

	// If no priorities found, add a helpful message
	if len(results) == 0 {
		results = append(results, domain.PeekResultItem{
			Key:     "_no_priorities_",
			Value:   "",
			Content: "No priorities available (field may be disabled for this project)",
		})
	}

	return results, nil
}

// extractGlobalPriorities extracts priorities from global priorities response
func (i *JiraIntegration) extractGlobalPriorities(body io.Reader) ([]domain.PeekResultItem, error) {
	// Limit response size to prevent DoS
	limitedReader := io.LimitReader(body, 1*1024*1024) // 1MB max for priorities

	var priorities []map[string]interface{}
	if err := json.NewDecoder(limitedReader).Decode(&priorities); err != nil {
		return nil, err
	}

	var results []domain.PeekResultItem
	// Limit results to prevent excessive memory usage
	const maxGlobalPriorities = 100
	processed := 0

	for _, priority := range priorities {
		if processed >= maxGlobalPriorities {
			break
		}
		if name, ok := priority["name"].(string); ok {
			results = append(results, domain.PeekResultItem{
				Key:     name,
				Value:   name,
				Content: name,
			})
			processed++
		}
	}

	// If no global priorities found, add a helpful message
	if len(results) == 0 {
		results = append(results, domain.PeekResultItem{
			Key:     "_no_global_priorities_",
			Value:   "",
			Content: "No global priorities found (Jira admin may have disabled priorities)",
		})
	}

	return results, nil
}

// Helper functions for clean data extraction
func (i *JiraIntegration) getProjects(createMeta map[string]interface{}) []map[string]interface{} {
	projects, ok := createMeta["projects"].([]interface{})
	if !ok {
		return nil
	}

	var result []map[string]interface{}
	for _, projectInterface := range projects {
		if project, ok := projectInterface.(map[string]interface{}); ok {
			result = append(result, project)
		}
	}
	return result
}

func (i *JiraIntegration) getIssueTypes(project map[string]interface{}) []map[string]interface{} {
	issueTypes, ok := project["issuetypes"].([]interface{})
	if !ok {
		return nil
	}

	var result []map[string]interface{}
	for _, issueTypeInterface := range issueTypes {
		if issueType, ok := issueTypeInterface.(map[string]interface{}); ok {
			result = append(result, issueType)
		}
	}
	return result
}

func (i *JiraIntegration) getPrioritiesFromIssueType(issueType map[string]interface{}) []string {
	fields, ok := issueType["fields"].(map[string]interface{})
	if !ok {
		return nil
	}

	priorityField, ok := fields["priority"].(map[string]interface{})
	if !ok {
		return nil
	}

	allowedValues, ok := priorityField["allowedValues"].([]interface{})
	if !ok {
		return nil
	}

	var priorities []string
	for _, priorityInterface := range allowedValues {
		if priority, ok := priorityInterface.(map[string]interface{}); ok {
			if name, ok := priority["name"].(string); ok {
				priorities = append(priorities, name)
			}
		}
	}
	return priorities
}

func (i *JiraIntegration) convertPrioritySetToResults(prioritySet map[string]bool) []domain.PeekResultItem {
	var results []domain.PeekResultItem
	for name := range prioritySet {
		results = append(results, domain.PeekResultItem{
			Key:     name,
			Value:   name,
			Content: name,
		})
	}
	return results
}

type PeekAssigneesParams struct {
	ProjectKey string `json:"project_key"`
	IssueKey   string `json:"issue_key"`
}

// PeekAssignees fetches assignable users for a specific project
func (i *JiraIntegration) PeekAssignees(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	params := PeekAssigneesParams{}
	err := json.Unmarshal(p.PayloadJSON, &params)
	if err != nil {
		// If we can't parse params, try to get all assignable users
		params.ProjectKey = ""
		params.IssueKey = ""
	}

	var projectKey string

	// Try to get project key from issue key if project key is not available
	if params.ProjectKey != "" {
		projectKey = params.ProjectKey
	} else if params.IssueKey != "" {
		// Extract project key from issue key (e.g., PROJ-123 -> PROJ)
		parts := strings.Split(params.IssueKey, "-")
		if len(parts) > 0 {
			projectKey = parts[0]
		}
	}

	var apiURL string
	if projectKey != "" {
		// Get assignable users for the specific project
		apiURL = fmt.Sprintf("/rest/api/3/user/assignable/search?project=%s&maxResults=50", url.QueryEscape(projectKey))
	} else {
		// Fallback: use email domain search to get real users (most emails have @)
		apiURL = "/rest/api/3/user/search?query=@&maxResults=50"
	}

	req, err := i.createAuthenticatedRequest(ctx, i.credentialID, "GET", apiURL, nil)
	if err != nil {
		return domain.PeekResult{}, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return domain.PeekResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return domain.PeekResult{}, fmt.Errorf("failed to get assignable users: %s - %s", resp.Status, string(bodyBytes))
	}

	var users []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return domain.PeekResult{}, err
	}

	var results []domain.PeekResultItem
	for _, user := range users {
		emailAddress, emailOk := user["emailAddress"].(string)
		displayName, displayNameOk := user["displayName"].(string)

		// Filter out apps/bots - they typically don't have email addresses or have different patterns
		if emailOk && displayNameOk && emailAddress != "" && strings.Contains(emailAddress, "@") {
			results = append(results, domain.PeekResultItem{
				Key:     emailAddress,
				Value:   emailAddress,
				Content: fmt.Sprintf("%s (%s)", displayName, emailAddress),
			})
		}
	}

	// If no users found but we have items, show a helpful message
	if len(results) == 0 && len(users) > 0 {
		results = append(results, domain.PeekResultItem{
			Key:     "_no_assignable_users_",
			Value:   "",
			Content: "No assignable users found (may be apps instead of users)",
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekStatusesParams struct {
	IssueKey string `json:"issue_key"`
}

// PeekStatuses fetches available status transitions for a specific issue
func (i *JiraIntegration) PeekStatuses(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	params := PeekStatusesParams{}
	err := json.Unmarshal(p.PayloadJSON, &params)
	if err != nil {
		// If we can't parse params, get global statuses
		params.IssueKey = ""
	}

	var apiURL string
	if params.IssueKey != "" {
		// Get available transitions for the specific issue
		apiURL = fmt.Sprintf("/rest/api/3/issue/%s/transitions", params.IssueKey)
	} else {
		// Fallback to global statuses
		apiURL = "/rest/api/3/status"
	}

	req, err := i.createAuthenticatedRequest(ctx, i.credentialID, "GET", apiURL, nil)
	if err != nil {
		return domain.PeekResult{}, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return domain.PeekResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return domain.PeekResult{}, fmt.Errorf("failed to get statuses: %s - %s", resp.Status, string(bodyBytes))
	}

	// Limit response size to prevent DoS
	limitedReader := io.LimitReader(resp.Body, 1*1024*1024) // 1MB max for statuses

	var results []domain.PeekResultItem

	if params.IssueKey != "" {
		// Parse transitions response for issue-specific statuses
		var transitionsResp map[string]interface{}
		if err := json.NewDecoder(limitedReader).Decode(&transitionsResp); err != nil {
			return domain.PeekResult{}, err
		}

		if transitions, ok := transitionsResp["transitions"].([]interface{}); ok {
			for _, transitionInterface := range transitions {
				if transition, ok := transitionInterface.(map[string]interface{}); ok {
					if to, exists := transition["to"].(map[string]interface{}); exists {
						if statusName, nameExists := to["name"].(string); nameExists {
							results = append(results, domain.PeekResultItem{
								Key:     statusName,
								Value:   statusName,
								Content: statusName,
							})
						}
					}
				}
			}
		}
	} else {
		// Parse global statuses response
		var statuses []map[string]interface{}
		if err := json.NewDecoder(limitedReader).Decode(&statuses); err != nil {
			return domain.PeekResult{}, err
		}

		// Limit results to prevent excessive memory usage
		const maxStatuses = 100
		processed := 0

		for _, status := range statuses {
			if processed >= maxStatuses {
				break
			}
			if name, ok := status["name"].(string); ok {
				results = append(results, domain.PeekResultItem{
					Key:     name,
					Value:   name,
					Content: name,
				})
				processed++
			}
		}
	}

	// If no statuses found, add a helpful message
	if len(results) == 0 {
		if params.IssueKey != "" {
			results = append(results, domain.PeekResultItem{
				Key:     "_no_transitions_",
				Value:   "",
				Content: "No status transitions available for this issue",
			})
		} else {
			results = append(results, domain.PeekResultItem{
				Key:     "_no_statuses_",
				Value:   "",
				Content: "No statuses found (Jira admin may have disabled status access)",
			})
		}
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

// updateIssueStatusWithClient updates an issue's status using go-jira client transitions
func (i *JiraIntegration) updateIssueStatusWithClient(client *jira.Client, issueKey, targetStatus string) error {
	// Step 1: Get available transitions for the issue
	transitions, _, err := i.jiraClient.Issue.GetTransitions(issueKey)
	if err != nil {
		return fmt.Errorf("failed to get transitions: %w", err)
	}

	// Step 2: Find the transition that leads to the target status
	var targetTransitionID string
	for _, transition := range transitions {
		if transition.To.Name == targetStatus {
			targetTransitionID = transition.ID
			break
		}
	}

	if targetTransitionID == "" {
		return fmt.Errorf("no valid transition found to status '%s'. Available transitions need to be checked", targetStatus)
	}

	// Step 3: Execute the transition using go-jira DoTransition
	_, err = i.jiraClient.Issue.DoTransition(issueKey, targetTransitionID)
	if err != nil {
		return fmt.Errorf("failed to execute transition: %w", err)
	}

	return nil
}

type PeekIssuesParams struct {
	ProjectKey string `json:"project_key"`
}

// isSubtaskType checks if the given issue type is a subtask variant
func isSubtaskType(issueType string) bool {
	lowerType := strings.ToLower(issueType)
	return lowerType == "sub-task" || lowerType == "subtask" || lowerType == "sub task"
}

// PeekIssues fetches existing issues from a project that can be used as parent issues
func (i *JiraIntegration) PeekIssues(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	params := PeekIssuesParams{}
	err := json.Unmarshal(p.PayloadJSON, &params)
	if err != nil {
		// If we can't parse params, return empty result
		return domain.PeekResult{}, fmt.Errorf("project key is required to fetch issues")
	}

	if params.ProjectKey == "" {
		return domain.PeekResult{}, fmt.Errorf("project key is required to fetch issues")
	}

	// Use JQL to fetch issues from the project, excluding subtasks
	// Note: We exclude subtasks by checking if parent field is empty
	jql := fmt.Sprintf("project = %s AND parent is EMPTY ORDER BY created DESC", params.ProjectKey)

	req, err := i.createAuthenticatedRequest(ctx, i.credentialID, "GET",
		fmt.Sprintf("/rest/api/3/search?jql=%s&maxResults=50&fields=key,summary,issuetype", url.QueryEscape(jql)), nil)
	if err != nil {
		return domain.PeekResult{}, err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return domain.PeekResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return domain.PeekResult{}, fmt.Errorf("failed to search issues: %s - %s", resp.Status, string(bodyBytes))
	}

	var searchResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return domain.PeekResult{}, err
	}

	results := i.extractIssuesFromSearchResult(searchResult)

	if len(results) == 0 {
		results = append(results, domain.PeekResultItem{
			Key:     "_no_issues_",
			Value:   "",
			Content: "No issues found in this project",
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

// getUserAccountIds converts email addresses to Jira user account IDs using the go-jira library.
func (i *JiraIntegration) getUserAccountIds(ctx context.Context, credentialID string, emails []string) ([]string, error) {
	// Get or create Jira client
	if i.jiraClient == nil {
		return nil, fmt.Errorf("Jira client not initialized")
	}

	var accountIds []string

	for _, email := range emails {
		// Search for user by email using go-jira library
		users, _, err := i.jiraClient.User.Find(email)
		if err != nil {
			continue // Try the next email
		}

		// Find the exact email match from the search results
		for _, user := range users {
			if user.EmailAddress == email && user.AccountID != "" {
				accountIds = append(accountIds, user.AccountID)
				break // Found the user, move to the next email
			}
		}
	}

	if len(accountIds) == 0 {
		return nil, fmt.Errorf("no valid users found for the provided email addresses. Make sure the emails exist in Jira and are accessible by the API user")
	}

	return accountIds, nil
}

// extractIssuesFromSearchResult processes Jira search results and converts them to PeekResultItem format
func (i *JiraIntegration) extractIssuesFromSearchResult(searchResult map[string]interface{}) []domain.PeekResultItem {
	issues, ok := searchResult["issues"].([]interface{})
	if !ok {
		return []domain.PeekResultItem{}
	}

	var results []domain.PeekResultItem
	for _, issueInterface := range issues {
		if issueItem := i.convertIssueToResultItem(issueInterface); issueItem != nil {
			results = append(results, *issueItem)
		}
	}

	return results
}

// convertIssueToResultItem converts a single issue interface to a PeekResultItem
func (i *JiraIntegration) convertIssueToResultItem(issueInterface interface{}) *domain.PeekResultItem {
	issue, ok := issueInterface.(map[string]interface{})
	if !ok {
		return nil
	}

	key := i.extractIssueKey(issue)
	if key == "" {
		return nil
	}

	fields := i.extractIssueFields(issue)
	if fields == nil {
		return nil
	}

	summary := i.extractStringSafely(fields, "summary")
	issueType := i.extractIssueTypeName(fields)
	displayText := i.formatIssueDisplayText(key, summary, issueType)

	return &domain.PeekResultItem{
		Key:     key,
		Value:   key,
		Content: displayText,
	}
}

// extractIssueKey safely extracts the issue key from an issue map
func (i *JiraIntegration) extractIssueKey(issue map[string]interface{}) string {
	key, ok := issue["key"].(string)
	if !ok {
		return ""
	}
	return key
}

// extractIssueFields safely extracts the fields map from an issue
func (i *JiraIntegration) extractIssueFields(issue map[string]interface{}) map[string]interface{} {
	fields, ok := issue["fields"].(map[string]interface{})
	if !ok {
		return nil
	}
	return fields
}

// extractStringSafely safely extracts a string value from a map
func (i *JiraIntegration) extractStringSafely(data map[string]interface{}, key string) string {
	value, _ := data[key].(string)
	return value
}

// extractIssueTypeName safely extracts the issue type name from fields
func (i *JiraIntegration) extractIssueTypeName(fields map[string]interface{}) string {
	issueTypeObj, ok := fields["issuetype"].(map[string]interface{})
	if !ok {
		return ""
	}
	return i.extractStringSafely(issueTypeObj, "name")
}

// formatIssueDisplayText creates a formatted display text for an issue
func (i *JiraIntegration) formatIssueDisplayText(key, summary, issueType string) string {
	if issueType != "" {
		return fmt.Sprintf("%s - [%s] %s", key, issueType, summary)
	}
	return fmt.Sprintf("%s - %s", key, summary)
}
