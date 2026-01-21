package notionintegration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	NotionActionType_AppendBlock          domain.IntegrationActionType = "notion_append_block"
	NotionActionType_GetManyChildBlocks   domain.IntegrationActionType = "notion_get_many_child_blocks"
	NotionActionType_GetDatabase          domain.IntegrationActionType = "notion_get_database"
	NotionActionType_GetManyDatabases     domain.IntegrationActionType = "notion_get_many_databases"
	NotionActionType_SearchDatabase       domain.IntegrationActionType = "notion_search_database"
	NotionActionType_CreateDatabasePage   domain.IntegrationActionType = "notion_create_database_page"
	NotionActionType_GetDatabasePage      domain.IntegrationActionType = "notion_get_database_page"
	NotionActionType_GetManyDatabasePages domain.IntegrationActionType = "notion_get_many_database_pages"
	NotionActionType_UpdateDatabasePage   domain.IntegrationActionType = "notion_update_database_page"
	NotionActionType_ArchivePage          domain.IntegrationActionType = "notion_archive_page"
	NotionActionType_CreatePage           domain.IntegrationActionType = "notion_create_page"
	NotionActionType_SearchPage           domain.IntegrationActionType = "notion_search_page"
	NotionActionType_GetUser              domain.IntegrationActionType = "notion_get_user"
	NotionActionType_GetManyUsers         domain.IntegrationActionType = "notion_get_many_users"

	NotionPeekable_Databases domain.IntegrationPeekableType = "notion_databases"

	NotionAPIVersion = "2022-06-28"
	NotionAPIBaseURL = "https://api.notion.com/v1"
)

type NotionIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewNotionIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &NotionIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
	}
}

func (c *NotionIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewNotionIntegration(ctx, NotionIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

type NotionIntegration struct {
	accessToken      string
	httpClient       *http.Client
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	actionManager    *domain.IntegrationActionManager
	peekFuncs        map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type NotionIntegrationDependencies struct {
	CredentialID     string
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

func NewNotionIntegration(ctx context.Context, deps NotionIntegrationDependencies) (*NotionIntegration, error) {
	integration := &NotionIntegration{
		binder:           deps.ParameterBinder,
		credentialGetter: deps.CredentialGetter,
		httpClient:       &http.Client{},
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(NotionActionType_AppendBlock, integration.AppendBlock).
		AddPerItemMulti(NotionActionType_GetManyChildBlocks, integration.GetManyChildBlocks).
		AddPerItem(NotionActionType_GetDatabase, integration.GetDatabase).
		AddPerItemMulti(NotionActionType_GetManyDatabases, integration.GetManyDatabases).
		AddPerItemMulti(NotionActionType_SearchDatabase, integration.SearchDatabase).
		AddPerItem(NotionActionType_CreateDatabasePage, integration.CreateDatabasePage).
		AddPerItem(NotionActionType_GetDatabasePage, integration.GetDatabasePage).
		AddPerItemMulti(NotionActionType_GetManyDatabasePages, integration.GetManyDatabasePages).
		AddPerItem(NotionActionType_UpdateDatabasePage, integration.UpdateDatabasePage).
		AddPerItem(NotionActionType_ArchivePage, integration.ArchivePage).
		AddPerItem(NotionActionType_CreatePage, integration.CreatePage).
		AddPerItemMulti(NotionActionType_SearchPage, integration.SearchPage).
		AddPerItem(NotionActionType_GetUser, integration.GetUser).
		AddPerItemMulti(NotionActionType_GetManyUsers, integration.GetManyUsers)

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		NotionPeekable_Databases: integration.PeekDatabases,
	}

	integration.actionManager = actionManager
	integration.peekFuncs = peekFuncs

	if deps.CredentialID == "" {
		return nil, fmt.Errorf("credential ID is required for Notion integration")
	}

	oauthAccount, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get decrypted Notion OAuth credential: %w", err)
	}

	integration.accessToken = oauthAccount.AccessToken

	return integration, nil
}

func (i *NotionIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *NotionIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function %s not found for Notion integration", params.PeekableType)
	}
	return peekFunc(ctx, params)
}

func (i *NotionIntegration) makeRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, NotionAPIBaseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+i.accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", NotionAPIVersion)

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Notion API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type AppendBlockParams struct {
	BlockID  string `json:"block_id"`
	Children string `json:"children"`
}

func (i *NotionIntegration) AppendBlock(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := AppendBlockParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.BlockID == "" {
		return nil, fmt.Errorf("block_id is required")
	}
	if params.Children == "" {
		return nil, fmt.Errorf("children blocks are required")
	}

	var childrenArray []interface{}
	if err := json.Unmarshal([]byte(params.Children), &childrenArray); err != nil {
		return nil, fmt.Errorf("failed to parse children JSON: %w", err)
	}

	reqBody := map[string]interface{}{
		"children": childrenArray,
	}

	respBody, err := i.makeRequest(ctx, "PATCH", fmt.Sprintf("/blocks/%s/children", params.BlockID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to append block: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type GetManyChildBlocksParams struct {
	BlockID     string  `json:"block_id"`
	StartCursor *string `json:"start_cursor,omitempty"`
	PageSize    *int    `json:"page_size,omitempty"`
}

func (i *NotionIntegration) GetManyChildBlocks(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetManyChildBlocksParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.BlockID == "" {
		return nil, fmt.Errorf("block_id is required")
	}

	endpoint := fmt.Sprintf("/blocks/%s/children?page_size=100", params.BlockID)
	if params.StartCursor != nil && *params.StartCursor != "" {
		endpoint += "&start_cursor=" + *params.StartCursor
	}
	if params.PageSize != nil && *params.PageSize > 0 {
		endpoint = fmt.Sprintf("/blocks/%s/children?page_size=%d", params.BlockID, *params.PageSize)
	}

	respBody, err := i.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get child blocks: %w", err)
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	items := make([]domain.Item, 0, len(result.Results))
	for _, block := range result.Results {
		items = append(items, block)
	}

	return items, nil
}

type GetDatabaseParams struct {
	DatabaseID string `json:"database_id"`
}

func (i *NotionIntegration) GetDatabase(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetDatabaseParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.DatabaseID == "" {
		return nil, fmt.Errorf("database_id is required")
	}

	respBody, err := i.makeRequest(ctx, "GET", fmt.Sprintf("/databases/%s", params.DatabaseID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type GetManyDatabasesParams struct {
	Query       *string `json:"query,omitempty"`
	StartCursor *string `json:"start_cursor,omitempty"`
	PageSize    *int    `json:"page_size,omitempty"`
}

func (i *NotionIntegration) GetManyDatabases(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetManyDatabasesParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	reqBody := map[string]interface{}{
		"filter": map[string]interface{}{
			"value":    "database",
			"property": "object",
		},
		"page_size": 100,
	}

	if params.Query != nil && *params.Query != "" {
		reqBody["query"] = *params.Query
	}
	if params.StartCursor != nil && *params.StartCursor != "" {
		reqBody["start_cursor"] = *params.StartCursor
	}
	if params.PageSize != nil && *params.PageSize > 0 {
		reqBody["page_size"] = *params.PageSize
	}

	respBody, err := i.makeRequest(ctx, "POST", "/search", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to search databases: %w", err)
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	items := make([]domain.Item, 0, len(result.Results))
	for _, db := range result.Results {
		items = append(items, db)
	}

	return items, nil
}

type SearchDatabaseParams struct {
	DatabaseID  string  `json:"database_id"`
	Filter      string  `json:"filter,omitempty"`
	Sorts       string  `json:"sorts,omitempty"`
	StartCursor *string `json:"start_cursor,omitempty"`
	PageSize    *int    `json:"page_size,omitempty"`
}

func (i *NotionIntegration) SearchDatabase(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := SearchDatabaseParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.DatabaseID == "" {
		return nil, fmt.Errorf("database_id is required")
	}

	reqBody := map[string]interface{}{
		"page_size": 100,
	}

	if params.Filter != "" {
		var filter interface{}
		if err := json.Unmarshal([]byte(params.Filter), &filter); err != nil {
			return nil, fmt.Errorf("failed to parse filter JSON: %w", err)
		}
		reqBody["filter"] = filter
	}

	if params.Sorts != "" {
		var sorts interface{}
		if err := json.Unmarshal([]byte(params.Sorts), &sorts); err != nil {
			return nil, fmt.Errorf("failed to parse sorts JSON: %w", err)
		}
		reqBody["sorts"] = sorts
	}

	if params.StartCursor != nil && *params.StartCursor != "" {
		reqBody["start_cursor"] = *params.StartCursor
	}
	if params.PageSize != nil && *params.PageSize > 0 {
		reqBody["page_size"] = *params.PageSize
	}

	respBody, err := i.makeRequest(ctx, "POST", fmt.Sprintf("/databases/%s/query", params.DatabaseID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	items := make([]domain.Item, 0, len(result.Results))
	for _, page := range result.Results {
		items = append(items, page)
	}

	return items, nil
}

func (i *NotionIntegration) GetManyDatabasePages(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	return i.SearchDatabase(ctx, input, item)
}

type CreateDatabasePageParams struct {
	DatabaseID string `json:"database_id"`
	Properties string `json:"properties"`
	Children   string `json:"children,omitempty"`
}

func (i *NotionIntegration) CreateDatabasePage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreateDatabasePageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.DatabaseID == "" {
		return nil, fmt.Errorf("database_id is required")
	}
	if params.Properties == "" {
		return nil, fmt.Errorf("properties are required")
	}

	var properties interface{}
	if err := json.Unmarshal([]byte(params.Properties), &properties); err != nil {
		return nil, fmt.Errorf("failed to parse properties JSON: %w. Make sure your JSON matches Notion property format", err)
	}

	reqBody := map[string]interface{}{
		"parent": map[string]interface{}{
			"type":        "database_id",
			"database_id": params.DatabaseID,
		},
		"properties": properties,
	}

	if params.Children != "" {
		var children interface{}
		if err := json.Unmarshal([]byte(params.Children), &children); err != nil {
			return nil, fmt.Errorf("failed to parse children JSON: %w", err)
		}
		reqBody["children"] = children
	}

	respBody, err := i.makeRequest(ctx, "POST", "/pages", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create database page: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type GetDatabasePageParams struct {
	PageID string `json:"page_id"`
}

func (i *NotionIntegration) GetDatabasePage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetDatabasePageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.PageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}

	respBody, err := i.makeRequest(ctx, "GET", fmt.Sprintf("/pages/%s", params.PageID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get database page: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type UpdateDatabasePageParams struct {
	PageID     string `json:"page_id"`
	Properties string `json:"properties,omitempty"`
	Archived   *bool  `json:"archived,omitempty"`
}

func (i *NotionIntegration) UpdateDatabasePage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := UpdateDatabasePageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.PageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}

	reqBody := map[string]interface{}{}

	if params.Properties != "" {
		var properties interface{}
		if err := json.Unmarshal([]byte(params.Properties), &properties); err != nil {
			return nil, fmt.Errorf("failed to parse properties JSON: %w", err)
		}
		reqBody["properties"] = properties
	}

	if params.Archived != nil {
		reqBody["archived"] = *params.Archived
	}

	respBody, err := i.makeRequest(ctx, "PATCH", fmt.Sprintf("/pages/%s", params.PageID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to update database page: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type ArchivePageParams struct {
	PageID string `json:"page_id"`
}

func (i *NotionIntegration) ArchivePage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := ArchivePageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.PageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}

	reqBody := map[string]interface{}{
		"archived": true,
	}

	respBody, err := i.makeRequest(ctx, "PATCH", fmt.Sprintf("/pages/%s", params.PageID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to archive page: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type CreatePageParams struct {
	ParentPageID     *string `json:"parent_page_id,omitempty"`
	ParentDatabaseID *string `json:"parent_database_id,omitempty"`
	Properties       string  `json:"properties"`
	Children         string  `json:"children,omitempty"`
}

func (i *NotionIntegration) CreatePage(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := CreatePageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if (params.ParentPageID == nil || *params.ParentPageID == "") && (params.ParentDatabaseID == nil || *params.ParentDatabaseID == "") {
		return nil, fmt.Errorf("either parent_page_id or parent_database_id is required")
	}
	if params.Properties == "" {
		return nil, fmt.Errorf("properties are required")
	}

	var properties interface{}
	if err := json.Unmarshal([]byte(params.Properties), &properties); err != nil {
		return nil, fmt.Errorf("failed to parse properties JSON: %w", err)
	}

	reqBody := map[string]interface{}{
		"properties": properties,
	}

	if params.ParentPageID != nil && *params.ParentPageID != "" {
		reqBody["parent"] = map[string]interface{}{
			"type":    "page_id",
			"page_id": *params.ParentPageID,
		}
	} else if params.ParentDatabaseID != nil && *params.ParentDatabaseID != "" {
		reqBody["parent"] = map[string]interface{}{
			"type":        "database_id",
			"database_id": *params.ParentDatabaseID,
		}
	}

	if params.Children != "" {
		var children interface{}
		if err := json.Unmarshal([]byte(params.Children), &children); err != nil {
			return nil, fmt.Errorf("failed to parse children JSON: %w", err)
		}
		reqBody["children"] = children
	}

	respBody, err := i.makeRequest(ctx, "POST", "/pages", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type SearchPageParams struct {
	Query       *string `json:"query,omitempty"`
	StartCursor *string `json:"start_cursor,omitempty"`
	PageSize    *int    `json:"page_size,omitempty"`
}

func (i *NotionIntegration) SearchPage(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := SearchPageParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	reqBody := map[string]interface{}{
		"filter": map[string]interface{}{
			"value":    "page",
			"property": "object",
		},
		"page_size": 100,
	}

	if params.Query != nil && *params.Query != "" {
		reqBody["query"] = *params.Query
	}

	if params.StartCursor != nil && *params.StartCursor != "" {
		reqBody["start_cursor"] = *params.StartCursor
	}

	if params.PageSize != nil && *params.PageSize > 0 {
		reqBody["page_size"] = *params.PageSize
	}

	respBody, err := i.makeRequest(ctx, "POST", "/search", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to search pages: %w", err)
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	items := make([]domain.Item, 0, len(result.Results))
	for _, page := range result.Results {
		items = append(items, page)
	}

	return items, nil
}

type GetUserParams struct {
	UserID string `json:"user_id"`
}

func (i *NotionIntegration) GetUser(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetUserParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	respBody, err := i.makeRequest(ctx, "GET", fmt.Sprintf("/users/%s", params.UserID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

type GetManyUsersParams struct {
	StartCursor *string `json:"start_cursor,omitempty"`
	PageSize    *int    `json:"page_size,omitempty"`
}

func (i *NotionIntegration) GetManyUsers(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := GetManyUsersParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	endpoint := "/users?page_size=100"
	if params.StartCursor != nil && *params.StartCursor != "" {
		endpoint += "&start_cursor=" + *params.StartCursor
	}
	if params.PageSize != nil && *params.PageSize > 0 {
		endpoint = fmt.Sprintf("/users?page_size=%d", *params.PageSize)
	}

	respBody, err := i.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	items := make([]domain.Item, 0, len(result.Results))
	for _, user := range result.Results {
		items = append(items, user)
	}

	return items, nil
}

func (i *NotionIntegration) PeekDatabases(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	type databasePeekItem struct {
		ID    string `json:"id"`
		Title []struct {
			PlainText string `json:"plain_text"`
		} `json:"title"`
	}
	limit := params.GetLimitWithMax(20, 100)

	reqBody := map[string]interface{}{
		"filter": map[string]interface{}{
			"value":    "database",
			"property": "object",
		},
		"page_size": limit,
	}

	if params.Pagination.Cursor != "" {
		reqBody["start_cursor"] = params.Pagination.Cursor
	}

	respBody, err := i.makeRequest(ctx, "POST", "/search", reqBody)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to peek databases: %w", err)
	}

	var result struct {
		Results    []databasePeekItem `json:"results"`
		HasMore    bool               `json:"has_more"`
		NextCursor string             `json:"next_cursor,omitempty"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to parse response: %w", err)
	}

	items := make([]domain.PeekResultItem, 0, len(result.Results))
	for _, db := range result.Results {
		label := db.ID
		if len(db.Title) > 0 && db.Title[0].PlainText != "" {
			label = db.Title[0].PlainText
		}

		items = append(items, domain.PeekResultItem{
			Key:     label,
			Value:   db.ID,
			Content: label,
		})
	}

	return domain.PeekResult{
		Result: items,
		Pagination: domain.PaginationMetadata{
			HasMore:    result.HasMore,
			NextCursor: result.NextCursor,
		},
	}, nil
}
