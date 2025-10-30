package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/hasura/go-graphql-client"
	"github.com/rs/zerolog/log"
)

const (
	IntegrationActionType_CreateIssue   domain.IntegrationActionType = "create_issue"
	IntegrationActionType_DeleteIssue   domain.IntegrationActionType = "delete_issue"
	IntegrationActionType_GetIssue      domain.IntegrationActionType = "get_issue"
	IntegrationActionType_GetManyIssues domain.IntegrationActionType = "get_many_issues"
	IntegrationActionType_UpdateIssue   domain.IntegrationActionType = "update_issue"
)

const (
	LinearIntegrationPeekable_Teams      domain.IntegrationPeekableType = "teams"
	LinearIntegrationPeekable_Priorities domain.IntegrationPeekableType = "priorities"
	LinearIntegrationPeekable_Users      domain.IntegrationPeekableType = "users"
	LinearIntegrationPeekable_Labels     domain.IntegrationPeekableType = "labels"
	LinearIntegrationPeekable_States     domain.IntegrationPeekableType = "states"
)

const (
	issueCreateMutation = `
	mutation IssueCreate($title: String!, $description: String, $teamId: String!, $priority: Int, $assigneeId: String, $labelIds: [String!]) {
		issueCreate(input: {
			title: $title,
			description: $description,
			teamId: $teamId,
			priority: $priority,
			assigneeId: $assigneeId,
			labelIds: $labelIds
		}) {
			issue {
				id
				title
				description
				url
				team {
					id
					name
				}
				state {
					id
					name
				}
				assignee {
					id
					name
				}
				labels {
					nodes {
						id
						name
					}
				}
			}
		}
	}`

	issueDeleteMutation = `
	mutation IssueDelete($id: String!) {
		issueDelete(id: $id) {
			success
		}
	}`

	issueQuery = `
	query Issue($id: String!) {
		issue(id: $id) {
			id
			title
			description
			url
			team {
				id
				name
			}
			state {
				id
				name
			}
			assignee {
				id
				name
			}
			labels {
				nodes {
					id
						name
					}
				}
			}
		}
	}`

	issuesQuery = `
	query Issues($filter: IssueFilter, $first: Int) {
		issues(filter: $filter, first: $first) {
			nodes {
				id
				title
				description
				url
				team {
					id
					name
				}
				state {
					id
					name
				}
				assignee {
					id
					name
				}
				labels {
					nodes {
						id
						name
					}
				}
			}
		}
	}`

	issueUpdateMutation = `
	mutation IssueUpdate($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) {
			issue {
				id
				title
				description
				url
				team {
					id
					name
				}
				state {
					id
					name
				}
				assignee {
					id
					name
				}
				labels {
					nodes {
						id
						name
					}
				}
			}
		}
	}`

	teamsQuery = `
	query Teams($first: Int!, $after: String) {
		teams(first: $first, after: $after) {
			nodes {
				id
				name
			}
			pageInfo {
				hasNextPage
				endCursor
			}
		}
	}`

	usersQuery = `
	query Users($first: Int!, $after: String) {
		users(first: $first, after: $after) {
			nodes {
				id
				name
				email
			}
			pageInfo {
				hasNextPage
				endCursor
			}
		}
	}`

	issueLabelsQuery = `
	query IssueLabels($first: Int!, $after: String) {
		issueLabels(first: $first, after: $after) {
			nodes {
				id
				name
			}
			pageInfo {
				hasNextPage
				endCursor
			}
		}
	}`

	workflowStatesQuery = `
	query WorkflowStates($teamId: String!, $first: Int!, $after: String) {
		workflowStates(filter: { team: { id: { eq: $teamId } } }, first: $first, after: $after) {
			nodes {
				id
				name
				type
			}
			pageInfo {
				hasNextPage
				endCursor
			}
		}
	}`
)

type LinearIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder           domain.IntegrationParameterBinder
}

func NewLinearIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &LinearIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

func (c *LinearIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewLinearIntegration(ctx, LinearIntegrationDependencies{
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
	})
}

type LinearIntegration struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder           domain.IntegrationParameterBinder

	graphqlClient *graphql.Client

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type linearTransport struct {
	apiKey    string
	transport http.RoundTripper
}

func (t *linearTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", t.apiKey)
	return t.transport.RoundTrip(req)
}

type LinearIntegrationDependencies struct {
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialID     string
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewLinearIntegration(ctx context.Context, deps LinearIntegrationDependencies) (*LinearIntegration, error) {
	integration := &LinearIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
		actionManager:    domain.NewIntegrationActionManager(),
	}

	integration.actionManager = domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_CreateIssue, integration.CreateIssue).
		AddPerItem(IntegrationActionType_DeleteIssue, integration.DeleteIssue).
		AddPerItem(IntegrationActionType_GetIssue, integration.GetIssue).
		AddPerItemMulti(IntegrationActionType_GetManyIssues, integration.GetManyIssues).
		AddPerItem(IntegrationActionType_UpdateIssue, integration.UpdateIssue)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		LinearIntegrationPeekable_Teams:         integration.PeekTeams,
		LinearIntegrationPeekable_Priorities:    integration.PeekPriorities,
		LinearIntegrationPeekable_Users:         integration.PeekUsers,
		LinearIntegrationPeekable_Labels:        integration.PeekLabels,
		LinearIntegrationPeekable_States:        integration.PeekWorkflowStates,
		LinearIntegrationPeekable_ResourceTypes: integration.PeekResourceTypes,
	}
	integration.peekFuncs = peekFuncs

	tokens, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Linear credential: %w", err)
	}

	accessToken := tokens.AccessToken

	httpClient := &http.Client{
		Transport: &linearTransport{
			apiKey:    "Bearer " + accessToken,
			transport: http.DefaultTransport,
		},
	}

	integration.graphqlClient = graphql.NewClient("https://api.linear.app/graphql", httpClient)

	return integration, nil
}

func (i *LinearIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type CreateIssueParams struct {
	CredentialID string   `json:"credential_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	TeamID       string   `json:"team_id"`
	PriorityID   string   `json:"priority_id"`
	AssigneeID   string   `json:"assignee_id"`
	LabelIDs     []string `json:"label_ids"`
}

func (i *LinearIntegration) CreateIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters for CreateIssue: %w", err)
	}
	if p.Title == "" {
		return nil, fmt.Errorf("issue title cannot be empty")
	}
	if p.TeamID == "" {
		return nil, fmt.Errorf("team ID cannot be empty")
	}
	var mutationResponse struct {
		IssueCreate struct {
			Issue struct {
				ID          string  `json:"id"`
				Title       string  `json:"title"`
				Description *string `json:"description"`
				URL         string  `json:"url"`
				Team        struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"team"`
				State struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"state"`
				Assignee *struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"assignee"`
				Labels struct {
					Nodes []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}
	vars := map[string]interface{}{
		"title":       p.Title,
		"description": (*string)(nil),
		"teamId":      p.TeamID,
		"priority":    (*int)(nil),
		"assigneeId":  (*string)(nil),
		"labelIds":    []string{},
	}
	if p.Description != "" {
		vars["description"] = p.Description
	}
	if p.PriorityID != "" {
		priorityInt, err := strconv.Atoi(p.PriorityID)
		if err != nil {
			return nil, fmt.Errorf("invalid priority ID format: %s. Must be an integer.", p.PriorityID)
		}
		vars["priority"] = priorityInt
	}
	if p.AssigneeID != "" {
		vars["assigneeId"] = p.AssigneeID
	}
	if len(p.LabelIDs) > 0 {
		vars["labelIds"] = p.LabelIDs
	}
	err = i.graphqlClient.Exec(ctx, issueCreateMutation, &mutationResponse, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to create Linear issue via GraphQL: %w", err)
	}
	return mutationResponse.IssueCreate.Issue, nil
}

type DeleteIssueParams struct {
	IssueID string `json:"issue_id"`
}

func (i *LinearIntegration) DeleteIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters for DeleteIssue: %w", err)
	}
	if p.IssueID == "" {
		return nil, fmt.Errorf("issue ID cannot be empty for deletion")
	}
	var mutationResponse struct {
		IssueDelete struct {
			Success bool `json:"success"`
		} `json:"issueDelete"`
	}
	vars := map[string]interface{}{
		"id": p.IssueID,
	}
	err = i.graphqlClient.Exec(ctx, issueDeleteMutation, &mutationResponse, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to delete Linear issue via GraphQL: %w", err)
	}
	if !mutationResponse.IssueDelete.Success {
		return nil, fmt.Errorf("Linear API reported unsuccessful deletion for issue ID: %s", p.IssueID)
	}
	return map[string]interface{}{"success": true, "deleted_issue_id": p.IssueID}, nil
}

type GetIssueParams struct {
	IssueID string `json:"issue_id"`
}

func (i *LinearIntegration) GetIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters for GetIssue: %w", err)
	}
	if p.IssueID == "" {
		return nil, fmt.Errorf("issue ID cannot be empty for retrieval")
	}
	var queryResponse struct {
		Issue struct {
			ID          string  `json:"id"`
			Title       string  `json:"title"`
			Description *string `json:"description"`
			URL         string  `json:"url"`
			Team        struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"team"`
			State struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"state"`
			Assignee *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"assignee"`
			Labels struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"labels"`
		} `json:"issue"`
	}
	vars := map[string]interface{}{
		"id": p.IssueID,
	}
	log.Info().Msgf("Attempting to get Linear issue with ID: %s", p.IssueID)
	err = i.graphqlClient.Exec(ctx, issueQuery, &queryResponse, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to get Linear issue via GraphQL: %w", err)
	}
	if queryResponse.Issue.ID == "" {
		return nil, fmt.Errorf("issue with ID %s not found", p.IssueID)
	}
	return queryResponse.Issue, nil
}

type GetManyIssuesParams struct {
	TeamID     string   `json:"team_id"`
	AssigneeID string   `json:"assignee_id"`
	LabelIDs   []string `json:"label_ids"`
	Query      string   `json:"query"`
	Limit      int      `json:"limit"`
}

func (i *LinearIntegration) GetManyIssues(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetManyIssuesParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters for GetManyIssues: %w", err)
	}
	issueFilter := make(map[string]interface{})
	if p.TeamID != "" {
		issueFilter["team"] = map[string]interface{}{
			"id": map[string]interface{}{"eq": p.TeamID},
		}
	}
	if p.AssigneeID != "" {
		issueFilter["assignee"] = map[string]interface{}{
			"id": map[string]interface{}{"eq": p.AssigneeID},
		}
	}
	if len(p.LabelIDs) > 0 {
		issueFilter["labels"] = map[string]interface{}{
			"some": map[string]interface{}{
				"id": map[string]interface{}{"in": p.LabelIDs},
			},
		}
	}
	if p.Query != "" {
		issueFilter["title"] = map[string]interface{}{"containsIgnoreCase": p.Query}
	}
	var queryResponse struct {
		Issues struct {
			Nodes []struct {
				ID          string  `json:"id"`
				Title       string  `json:"title"`
				Description *string `json:"description"`
				URL         string  `json:"url"`
				Team        struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"team"`
				State struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"state"`
				Assignee *struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"assignee"`
				Labels struct {
					Nodes []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"nodes"`
		} `json:"issues"`
	}
	vars := make(map[string]interface{})
	if len(issueFilter) > 0 {
		vars["filter"] = issueFilter
	}
	if p.Limit > 0 {
		vars["first"] = p.Limit
	} else {
		vars["first"] = 50
	}
	log.Info().Msgf("Attempting to get many Linear issues with filter: %+v", issueFilter)
	err = i.graphqlClient.Exec(ctx, issuesQuery, &queryResponse, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to get many Linear issues via GraphQL: %w", err)
	}
	items := make([]domain.Item, len(queryResponse.Issues.Nodes))
	for idx, node := range queryResponse.Issues.Nodes {
		items[idx] = node
	}
	return items, nil
}

type UpdateIssueParams struct {
	IssueID     string   `json:"issue_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	TeamID      string   `json:"team_id"`
	PriorityID  string   `json:"priority_id"`
	AssigneeID  string   `json:"assignee_id"`
	LabelIDs    []string `json:"label_ids"`
	StateID     string   `json:"state_id"`
}

func (i *LinearIntegration) UpdateIssue(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateIssueParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters for UpdateIssue: %w", err)
	}
	if p.IssueID == "" {
		return nil, fmt.Errorf("issue ID cannot be empty for update")
	}
	updateInput := make(map[string]interface{})
	if p.Title != "" {
		updateInput["title"] = p.Title
	}
	if p.Description != "" {
		updateInput["description"] = p.Description
	}
	if p.TeamID != "" {
		updateInput["teamId"] = p.TeamID
	}
	if p.PriorityID != "" {
		priorityInt, err := strconv.Atoi(p.PriorityID)
		if err != nil {
			return nil, fmt.Errorf("invalid priority ID format for update: %s. Must be an integer.", p.PriorityID)
		}
		updateInput["priority"] = priorityInt
	}
	if p.AssigneeID != "" {
		updateInput["assigneeId"] = p.AssigneeID
	}
	if params.IntegrationParams.Settings["label_ids"] != nil {
		if len(p.LabelIDs) == 0 {
			updateInput["labelIds"] = []string{}
		} else {
			updateInput["labelIds"] = p.LabelIDs
		}
	}
	if p.StateID != "" {
		updateInput["stateId"] = p.StateID
	}
	if len(updateInput) == 0 {
		return nil, fmt.Errorf("no update parameters provided for issue ID: %s", p.IssueID)
	}
	var mutationResponse struct {
		IssueUpdate struct {
			Issue struct {
				ID          string  `json:"id"`
				Title       string  `json:"title"`
				Description *string `json:"description"`
				URL         string  `json:"url"`
				Team        struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"team"`
				State struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"state"`
				Assignee *struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"assignee"`
				Labels struct {
					Nodes []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"issue"`
		} `json:"issueUpdate"`
	}
	vars := map[string]interface{}{
		"id":    p.IssueID,
		"input": updateInput,
	}
	err = i.graphqlClient.Exec(ctx, issueUpdateMutation, &mutationResponse, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to update Linear issue via GraphQL: %w", err)
	}
	return mutationResponse.IssueUpdate.Issue, nil
}

func (i *LinearIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found for type: %s", params.PeekableType)
	}

	return peekFunc(ctx, params)
}

func (i *LinearIntegration) PeekTeams(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	cursor := p.GetCursor()

	var queryResponse struct {
		Teams struct {
			Nodes []struct {
				ID   string
				Name string
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"teams"`
	}

	variables := map[string]interface{}{
		"first": limit,
	}
	if cursor != "" {
		variables["after"] = cursor
	}

	err := i.graphqlClient.Exec(ctx, teamsQuery, &queryResponse, variables)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear teams via GraphQL: %w", err)
	}

	var results []domain.PeekResultItem
	for _, team := range queryResponse.Teams.Nodes {
		results = append(results, domain.PeekResultItem{
			Key:     team.ID,
			Value:   team.ID,
			Content: team.Name,
		})
	}

	hasMore := queryResponse.Teams.PageInfo.HasNextPage
	nextCursor := queryResponse.Teams.PageInfo.EndCursor

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			Cursor:  nextCursor,
			HasMore: hasMore,
		},
	}

	result.SetCursor(nextCursor)
	result.SetHasMore(hasMore)

	return result, nil
}

func (i *LinearIntegration) PeekPriorities(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	priorities := []struct {
		ID   string
		Name string
	}{
		{ID: "0", Name: "No priority"},
		{ID: "1", Name: "Urgent"},
		{ID: "2", Name: "High"},
		{ID: "3", Name: "Medium"},
		{ID: "4", Name: "Low"},
	}

	var results []domain.PeekResultItem
	for _, priority := range priorities {
		results = append(results, domain.PeekResultItem{
			Key:     priority.ID,
			Value:   priority.ID,
			Content: priority.Name,
		})
	}

	return domain.PeekResult{Result: results}, nil
}

func (i *LinearIntegration) PeekUsers(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {

	limit := p.GetLimitWithMax(20, 100)
	cursor := p.GetCursor()

	var queryResponse struct {
		Users struct {
			Nodes []struct {
				ID    string
				Name  string
				Email string
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"users"`
	}

	variables := map[string]interface{}{
		"first": limit,
	}
	if cursor != "" {
		variables["after"] = cursor
	}

	err := i.graphqlClient.Exec(ctx, usersQuery, &queryResponse, variables)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear users: %w", err)
	}

	var results []domain.PeekResultItem
	for _, user := range queryResponse.Users.Nodes {
		content := user.Name
		if user.Email != "" {
			content = fmt.Sprintf("%s (%s)", user.Name, user.Email)
		}
		results = append(results, domain.PeekResultItem{
			Key:     user.ID,
			Value:   user.ID,
			Content: content,
		})
	}

	hasMore := queryResponse.Users.PageInfo.HasNextPage
	nextCursor := queryResponse.Users.PageInfo.EndCursor

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			Cursor:  nextCursor,
			HasMore: hasMore,
		},
	}

	result.SetCursor(nextCursor)
	result.SetHasMore(hasMore)

	return result, nil
}

func (i *LinearIntegration) PeekLabels(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {

	limit := p.GetLimitWithMax(20, 100)
	cursor := p.GetCursor()

	var queryResponse struct {
		IssueLabels struct {
			Nodes []struct {
				ID   string
				Name string
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"issueLabels"`
	}

	variables := map[string]interface{}{
		"first": limit,
	}
	if cursor != "" {
		variables["after"] = cursor
	}

	err := i.graphqlClient.Exec(ctx, issueLabelsQuery, &queryResponse, variables)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear labels via GraphQL: %w", err)
	}

	var results []domain.PeekResultItem
	for _, label := range queryResponse.IssueLabels.Nodes {
		results = append(results, domain.PeekResultItem{
			Key:     label.ID,
			Value:   label.ID,
			Content: label.Name,
		})
	}

	hasMore := queryResponse.IssueLabels.PageInfo.HasNextPage
	nextCursor := queryResponse.IssueLabels.PageInfo.EndCursor

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			Cursor:  nextCursor,
			HasMore: hasMore,
		},
	}

	result.SetCursor(nextCursor)
	result.SetHasMore(hasMore)

	return result, nil
}

type PeekWorkflowStatesParams struct {
	TeamID string `json:"team_id"`
}

func (i *LinearIntegration) PeekWorkflowStates(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	cursor := p.GetCursor()

	params := PeekWorkflowStatesParams{}
	if len(p.PayloadJSON) > 0 {
		err := json.Unmarshal(p.PayloadJSON, &params)
		if err != nil {
			return domain.PeekResult{}, fmt.Errorf("failed to unmarshal peek workflow states params: %w", err)
		}
	}

	if params.TeamID == "" {
		return domain.PeekResult{}, fmt.Errorf("team_id is required to peek workflow states")
	}

	var queryResponse struct {
		WorkflowStates struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"workflowStates"`
	}

	vars := map[string]interface{}{
		"teamId": params.TeamID,
		"first":  limit,
	}
	if cursor != "" {
		vars["after"] = cursor
	}

	err := i.graphqlClient.Exec(ctx, workflowStatesQuery, &queryResponse, vars)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear workflow states via GraphQL: %w", err)
	}

	var results []domain.PeekResultItem
	for _, state := range queryResponse.WorkflowStates.Nodes {
		results = append(results, domain.PeekResultItem{
			Key:     state.ID,
			Value:   state.ID,
			Content: fmt.Sprintf("%s (%s)", state.Name, state.Type),
		})
	}

	hasMore := queryResponse.WorkflowStates.PageInfo.HasNextPage
	nextCursor := queryResponse.WorkflowStates.PageInfo.EndCursor

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			Cursor:  nextCursor,
			HasMore: hasMore,
		},
	}

	result.SetCursor(nextCursor)
	result.SetHasMore(hasMore)

	return result, nil
}

func (i *LinearIntegration) PeekResourceTypes(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	types := []struct {
		Key   string
		Label string
	}{
		{"Issue", "Issue"},
		{"Comment", "Comment"},
		{"Cycle", "Cycle"},
		{"IssueLabel", "Issue Label"},
		{"Reaction", "Reaction"},
		{"Project", "Project"},
	}
	var results []domain.PeekResultItem
	for _, t := range types {
		results = append(results, domain.PeekResultItem{
			Key:     t.Key,
			Value:   t.Key,
			Content: t.Label,
		})
	}
	return domain.PeekResult{Result: results}, nil
}
