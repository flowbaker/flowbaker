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
)

const (
	IntegrationActionType_CreateIssue   domain.IntegrationActionType = "create_issue"
	IntegrationActionType_DeleteIssue   domain.IntegrationActionType = "delete_issue"
	IntegrationActionType_GetIssue      domain.IntegrationActionType = "get_issue"
	IntegrationActionType_GetManyIssues domain.IntegrationActionType = "get_many_issues"
	IntegrationActionType_UpdateIssue   domain.IntegrationActionType = "update_issue"
	IntegrationActionType_AddComment    domain.IntegrationActionType = "add_comment"
	IntegrationActionType_AddLink       domain.IntegrationActionType = "add_link"
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
	mutation IssueCreate($title: String!, $description: String, $teamId: String!, $priority: Int, $assigneeId: String, $labelIds: [String!], $stateId: String) {
		issueCreate(input: {
			title: $title,
			description: $description,
			teamId: $teamId,
			priority: $priority,
			assigneeId: $assigneeId,
			labelIds: $labelIds,
			stateId: $stateId
		}) {
			success
			issue {
				id
				title
				description
				url
				team {
					id
					name
					key
				}
				state {
					id
					name
					type
				}
				assignee {
					id
					name
					email
				}
				labels {
					nodes {
						id
						name
						color
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
				key
			}
			state {
				id
				name
				type
			}
			assignee {
				id
				name
				email
			}
			labels {
				nodes {
					id
					name
					color
				}
			}
			comments {
				nodes {
					id
					body
					createdAt
					user {
						id
						name
					}
				}
			}
			attachments {
				nodes {
					id
					title
					url
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
					key
				}
				state {
					id
					name
					type
				}
				assignee {
					id
					name
					email
				}
				labels {
					nodes {
						id
						name
						color
					}
				}
			}
		}
	}`

	issueUpdateMutation = `
	mutation IssueUpdate($id: String!, $input: IssueUpdateInput!) {
		issueUpdate(id: $id, input: $input) {
			success
			issue {
				id
				title
				description
				url
				team {
					id
					name
					key
				}
				state {
					id
					name
					type
				}
				assignee {
					id
					name
					email
				}
				labels {
					nodes {
						id
						name
						color
					}
				}
			}
		}
	}`

	commentCreateMutation = `
	mutation CommentCreate($issueId: String!, $body: String!) {
		commentCreate(input: {
			issueId: $issueId,
			body: $body
		}) {
			success
			comment {
				id
				body
				createdAt
				user {
					id
					name
				}
			}
		}
	}`

	attachmentLinkURLMutation = `
	mutation AttachmentLinkURL($issueId: String!, $url: String!, $title: String) {
		attachmentLinkURL(issueId: $issueId, url: $url, title: $title) {
			success
			attachment {
				id
				title
				url
			}
		}
	}`

	teamsQuery = `
	query Teams($first: Int!, $after: String) {
		teams(first: $first, after: $after) {
			nodes {
				id
				name
				key
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
				color
			}
			pageInfo {
				hasNextPage
				endCursor
			}
		}
	}`

	workflowStatesQuery = `
	query WorkflowStates($teamId: String!, $first: Int!, $after: String) {
		team(id: $teamId) {
			states(first: $first, after: $after) {
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
		}
	}`
)

type TeamNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type StateNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type AssigneeNode struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type LabelNode struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CommentNode struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	User      *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
}

type AttachmentNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type LinearIssueResponse struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description *string       `json:"description"`
	URL         string        `json:"url"`
	Team        *TeamNode     `json:"team"`
	State       *StateNode    `json:"state"`
	Assignee    *AssigneeNode `json:"assignee"`
	Labels      struct {
		Nodes []LabelNode `json:"nodes"`
	} `json:"labels"`
}

type LinearIssueDetailResponse struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description *string       `json:"description"`
	URL         string        `json:"url"`
	Team        *TeamNode     `json:"team"`
	State       *StateNode    `json:"state"`
	Assignee    *AssigneeNode `json:"assignee"`
	Labels      struct {
		Nodes []LabelNode `json:"nodes"`
	} `json:"labels"`
	Comments struct {
		Nodes []CommentNode `json:"nodes"`
	} `json:"comments"`
	Attachments struct {
		Nodes []AttachmentNode `json:"nodes"`
	} `json:"attachments"`
}

type LinearCommentResponse struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	User      *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
}

type LinearAttachmentResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type IssueCreateResponse struct {
	IssueCreate struct {
		Success bool                 `json:"success"`
		Issue   *LinearIssueResponse `json:"issue"`
	} `json:"issueCreate"`
}

type IssueDeleteResponse struct {
	IssueDelete struct {
		Success bool `json:"success"`
	} `json:"issueDelete"`
}

type IssueQueryResponse struct {
	Issue *LinearIssueDetailResponse `json:"issue"`
}

type IssuesQueryResponse struct {
	Issues struct {
		Nodes []LinearIssueResponse `json:"nodes"`
	} `json:"issues"`
}

type IssueUpdateResponse struct {
	IssueUpdate struct {
		Success bool                 `json:"success"`
		Issue   *LinearIssueResponse `json:"issue"`
	} `json:"issueUpdate"`
}

type CommentCreateResponse struct {
	CommentCreate struct {
		Success bool                   `json:"success"`
		Comment *LinearCommentResponse `json:"comment"`
	} `json:"commentCreate"`
}

type AttachmentLinkURLResponse struct {
	AttachmentLinkURL struct {
		Success    bool                      `json:"success"`
		Attachment *LinearAttachmentResponse `json:"attachment"`
	} `json:"attachmentLinkURL"`
}

type TeamsQueryResponse struct {
	Teams struct {
		Nodes    []TeamNode `json:"nodes"`
		PageInfo PageInfo   `json:"pageInfo"`
	} `json:"teams"`
}

type UsersQueryResponse struct {
	Users struct {
		Nodes    []AssigneeNode `json:"nodes"`
		PageInfo PageInfo       `json:"pageInfo"`
	} `json:"users"`
}

type LabelsQueryResponse struct {
	IssueLabels struct {
		Nodes    []LabelNode `json:"nodes"`
		PageInfo PageInfo    `json:"pageInfo"`
	} `json:"issueLabels"`
}

type WorkflowStatesQueryResponse struct {
	Team struct {
		States struct {
			Nodes    []StateNode `json:"nodes"`
			PageInfo PageInfo    `json:"pageInfo"`
		} `json:"states"`
	} `json:"team"`
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
	graphqlClient    *graphql.Client
	actionManager    *domain.IntegrationActionManager
	peekFuncs        map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
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
		AddPerItem(IntegrationActionType_UpdateIssue, integration.UpdateIssue).
		AddPerItem(IntegrationActionType_AddComment, integration.AddComment).
		AddPerItem(IntegrationActionType_AddLink, integration.AddLink)

	integration.peekFuncs = map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		LinearIntegrationPeekable_Teams:      integration.PeekTeams,
		LinearIntegrationPeekable_Priorities: integration.PeekPriorities,
		LinearIntegrationPeekable_Users:      integration.PeekUsers,
		LinearIntegrationPeekable_Labels:     integration.PeekLabels,
		LinearIntegrationPeekable_States:     integration.PeekWorkflowStates,
	}

	tokens, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Linear credential: %w", err)
	}

	httpClient := &http.Client{
		Transport: &linearTransport{
			apiKey:    "Bearer " + tokens.AccessToken,
			transport: http.DefaultTransport,
		},
	}

	integration.graphqlClient = graphql.NewClient("https://api.linear.app/graphql", httpClient)

	return integration, nil
}

func (i *LinearIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *LinearIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found for type: %s", params.PeekableType)
	}
	return peekFunc(ctx, params)
}

type CreateIssueParams struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	TeamID      string   `json:"team_id"`
	PriorityID  string   `json:"priority_id"`
	AssigneeID  string   `json:"assignee_id"`
	LabelIDs    []string `json:"label_ids"`
	StateID     string   `json:"state_id"`
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

	vars := map[string]interface{}{
		"title":       p.Title,
		"description": (*string)(nil),
		"teamId":      p.TeamID,
		"priority":    (*int)(nil),
		"assigneeId":  (*string)(nil),
		"labelIds":    []string{},
		"stateId":     (*string)(nil),
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
	if p.StateID != "" {
		vars["stateId"] = p.StateID
	}

	var response IssueCreateResponse
	err = i.graphqlClient.Exec(ctx, issueCreateMutation, &response, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to create Linear issue via GraphQL: %w", err)
	}

	if response.IssueCreate.Issue == nil {
		return nil, fmt.Errorf("failed to create Linear issue: no issue returned")
	}

	return response.IssueCreate.Issue, nil
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

	vars := map[string]interface{}{
		"id": p.IssueID,
	}

	var response IssueDeleteResponse
	err = i.graphqlClient.Exec(ctx, issueDeleteMutation, &response, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to delete Linear issue via GraphQL: %w", err)
	}

	if !response.IssueDelete.Success {
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

	vars := map[string]interface{}{
		"id": p.IssueID,
	}

	var response IssueQueryResponse
	err = i.graphqlClient.Exec(ctx, issueQuery, &response, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to get Linear issue via GraphQL: %w", err)
	}

	if response.Issue == nil || response.Issue.ID == "" {
		return nil, fmt.Errorf("issue with ID %s not found", p.IssueID)
	}

	return response.Issue, nil
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

	vars := make(map[string]interface{})
	if len(issueFilter) > 0 {
		vars["filter"] = issueFilter
	}
	if p.Limit > 0 {
		vars["first"] = p.Limit
	} else {
		vars["first"] = 50
	}

	var response IssuesQueryResponse
	err = i.graphqlClient.Exec(ctx, issuesQuery, &response, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to get many Linear issues via GraphQL: %w", err)
	}

	items := make([]domain.Item, len(response.Issues.Nodes))
	for idx, node := range response.Issues.Nodes {
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

	vars := map[string]interface{}{
		"id":    p.IssueID,
		"input": updateInput,
	}

	var response IssueUpdateResponse
	err = i.graphqlClient.Exec(ctx, issueUpdateMutation, &response, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to update Linear issue via GraphQL: %w", err)
	}

	if response.IssueUpdate.Issue == nil {
		return nil, fmt.Errorf("failed to update Linear issue: no issue returned")
	}

	return response.IssueUpdate.Issue, nil
}

type AddCommentParams struct {
	IssueID string `json:"issue_id"`
	Body    string `json:"body"`
}

func (i *LinearIntegration) AddComment(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := AddCommentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters for AddComment: %w", err)
	}

	if p.IssueID == "" {
		return nil, fmt.Errorf("issue ID cannot be empty for adding comment")
	}
	if p.Body == "" {
		return nil, fmt.Errorf("comment body cannot be empty")
	}

	vars := map[string]interface{}{
		"issueId": p.IssueID,
		"body":    p.Body,
	}

	var response CommentCreateResponse
	err = i.graphqlClient.Exec(ctx, commentCreateMutation, &response, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to add comment to Linear issue via GraphQL: %w", err)
	}

	if response.CommentCreate.Comment == nil {
		return nil, fmt.Errorf("failed to add comment: no comment returned")
	}

	return response.CommentCreate.Comment, nil
}

type AddLinkParams struct {
	IssueID string `json:"issue_id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
}

func (i *LinearIntegration) AddLink(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := AddLinkParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters for AddLink: %w", err)
	}

	if p.IssueID == "" {
		return nil, fmt.Errorf("issue ID cannot be empty for adding link")
	}
	if p.URL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	vars := map[string]interface{}{
		"issueId": p.IssueID,
		"url":     p.URL,
		"title":   (*string)(nil),
	}

	if p.Title != "" {
		vars["title"] = p.Title
	}

	var response AttachmentLinkURLResponse
	err = i.graphqlClient.Exec(ctx, attachmentLinkURLMutation, &response, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to add link to Linear issue via GraphQL: %w", err)
	}

	if response.AttachmentLinkURL.Attachment == nil {
		return nil, fmt.Errorf("failed to add link: no attachment returned")
	}

	return response.AttachmentLinkURL.Attachment, nil
}

func (i *LinearIntegration) PeekTeams(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	cursor := p.Pagination.Cursor

	variables := map[string]interface{}{
		"first": limit,
	}
	if cursor != "" {
		variables["after"] = cursor
	}

	var response TeamsQueryResponse
	err := i.graphqlClient.Exec(ctx, teamsQuery, &response, variables)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear teams via GraphQL: %w", err)
	}

	var results []domain.PeekResultItem
	for _, team := range response.Teams.Nodes {
		results = append(results, domain.PeekResultItem{
			Key:     team.ID,
			Value:   team.ID,
			Content: team.Name,
		})
	}

	return domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextCursor: response.Teams.PageInfo.EndCursor,
			HasMore:    response.Teams.PageInfo.HasNextPage,
		},
	}, nil
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
	cursor := p.Pagination.Cursor

	variables := map[string]interface{}{
		"first": limit,
	}
	if cursor != "" {
		variables["after"] = cursor
	}

	var response UsersQueryResponse
	err := i.graphqlClient.Exec(ctx, usersQuery, &response, variables)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear users: %w", err)
	}

	var results []domain.PeekResultItem
	for _, user := range response.Users.Nodes {
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

	return domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextCursor: response.Users.PageInfo.EndCursor,
			HasMore:    response.Users.PageInfo.HasNextPage,
		},
	}, nil
}

func (i *LinearIntegration) PeekLabels(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	cursor := p.Pagination.Cursor

	variables := map[string]interface{}{
		"first": limit,
	}
	if cursor != "" {
		variables["after"] = cursor
	}

	var response LabelsQueryResponse
	err := i.graphqlClient.Exec(ctx, issueLabelsQuery, &response, variables)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear labels via GraphQL: %w", err)
	}

	var results []domain.PeekResultItem
	for _, label := range response.IssueLabels.Nodes {
		results = append(results, domain.PeekResultItem{
			Key:     label.ID,
			Value:   label.ID,
			Content: label.Name,
		})
	}

	return domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextCursor: response.IssueLabels.PageInfo.EndCursor,
			HasMore:    response.IssueLabels.PageInfo.HasNextPage,
		},
	}, nil
}

type PeekWorkflowStatesParams struct {
	TeamID string `json:"team_id"`
}

func (i *LinearIntegration) PeekWorkflowStates(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	cursor := p.Pagination.Cursor

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

	vars := map[string]interface{}{
		"teamId": params.TeamID,
		"first":  limit,
	}
	if cursor != "" {
		vars["after"] = cursor
	}

	var response WorkflowStatesQueryResponse
	err := i.graphqlClient.Exec(ctx, workflowStatesQuery, &response, vars)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to fetch Linear workflow states via GraphQL: %w", err)
	}

	var results []domain.PeekResultItem
	for _, state := range response.Team.States.Nodes {
		results = append(results, domain.PeekResultItem{
			Key:     state.ID,
			Value:   state.ID,
			Content: fmt.Sprintf("%s (%s)", state.Name, state.Type),
		})
	}

	return domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextCursor: response.Team.States.PageInfo.EndCursor,
			HasMore:    response.Team.States.PageInfo.HasNextPage,
		},
	}, nil
}
