package startupswatchintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

type StartupsWatchCredential struct {
	Token string `json:"token"`
}

type StartupsWatchIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[StartupsWatchCredential]
}

func NewStartupsWatchIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &StartupsWatchIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[StartupsWatchCredential](deps.ExecutorCredentialManager),
	}
}

func (c *StartupsWatchIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewStartupsWatchIntegration(ctx, StartupsWatchIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

type StartupsWatchIntegration struct {
	httpClient    *http.Client
	token         string
	binder        domain.IntegrationParameterBinder
	actionManager *domain.IntegrationActionManager
}

type StartupsWatchIntegrationDependencies struct {
	CredentialID     string
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[StartupsWatchCredential]
}

type StartupsResponse struct {
	Startups []any `json:"startups"`
}

func NewStartupsWatchIntegration(ctx context.Context, deps StartupsWatchIntegrationDependencies) (*StartupsWatchIntegration, error) {
	integration := &StartupsWatchIntegration{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		binder: deps.ParameterBinder,
	}

	if deps.CredentialID != "" {
		credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get decrypted API credential: %w", err)
		}

		integration.token = credential.Token
	}

	// Setup action manager with 12 endpoints
	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(StartupsWatchActionType_GetStartup, integration.GetStartup).
		AddPerItemMulti(StartupsWatchActionType_ListStartups, integration.ListStartups).
		AddPerItemMulti(StartupsWatchActionType_SearchStartups, integration.SearchStartups).
		AddPerItem(StartupsWatchActionType_GetPerson, integration.GetPerson).
		AddPerItemMulti(StartupsWatchActionType_ListPeople, integration.ListPeople).
		AddPerItemMulti(StartupsWatchActionType_SearchPeople, integration.SearchPeople).
		AddPerItem(StartupsWatchActionType_GetInvestor, integration.GetInvestor).
		AddPerItemMulti(StartupsWatchActionType_ListInvestors, integration.ListInvestors).
		AddPerItemMulti(StartupsWatchActionType_SearchInvestors, integration.SearchInvestors).
		AddPerItemMulti(StartupsWatchActionType_ListInvestments, integration.ListInvestments).
		AddPerItemMulti(StartupsWatchActionType_ListAcquisitions, integration.ListAcquisitions).
		AddPerItemMulti(StartupsWatchActionType_ListEvents, integration.ListEvents)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *StartupsWatchIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *StartupsWatchIntegration) makeRequest(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	if i.token == "" {
		return nil, fmt.Errorf("API key is required")
	}

	baseURL := "https://startups.watch/api"

	u, err := url.Parse(baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	query := u.Query()
	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authentication-Token", i.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FlowBaker-Executor/1.0")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

type GetStartupParams struct {
	StartupID string `json:"startup_id"`
}

type ListStartupsParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

type SearchStartupsParams struct {
	Query string `json:"query"`
	Page  int    `json:"page,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type GetPersonParams struct {
	PersonID string `json:"person_id"`
}

type ListPeopleParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

type SearchPeopleParams struct {
	Query string `json:"query"`
	Page  int    `json:"page,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type GetInvestorParams struct {
	InvestorID string `json:"investor_id"`
}

type ListInvestorsParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

type SearchInvestorsParams struct {
	Query string `json:"query"`
	Page  int    `json:"page,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type ListInvestmentsParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

type ListAcquisitionsParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

type ListEventsParams struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

func (i *StartupsWatchIntegration) GetStartup(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetStartupParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.StartupID == "" {
		return nil, fmt.Errorf("startup_id is required")
	}

	response, err := i.makeRequest(ctx, "/startups/"+params.StartupID, nil)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (i *StartupsWatchIntegration) ListStartups(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListStartupsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	queryParams := map[string]string{}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/startups", queryParams)
	if err != nil {
		return nil, err
	}

	var startupsResp StartupsResponse
	if err := json.Unmarshal(response, &startupsResp); err != nil {
		fmt.Printf("DEBUG: API Response that failed to unmarshal: %s\n", string(response))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	items := make([]domain.Item, len(startupsResp.Startups))
	for i, startup := range startupsResp.Startups {
		items[i] = domain.Item(startup)
	}

	return items, nil
}

func (i *StartupsWatchIntegration) SearchStartups(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := SearchStartupsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	queryParams := map[string]string{
		"q": params.Query,
	}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/startups/search", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}

func (i *StartupsWatchIntegration) GetPerson(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetPersonParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.PersonID == "" {
		return nil, fmt.Errorf("person_id is required")
	}

	response, err := i.makeRequest(ctx, "/people/"+params.PersonID, nil)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (i *StartupsWatchIntegration) ListPeople(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListPeopleParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	queryParams := map[string]string{}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/people", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}

func (i *StartupsWatchIntegration) SearchPeople(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := SearchPeopleParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	queryParams := map[string]string{
		"q": params.Query,
	}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/people/search", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}

func (i *StartupsWatchIntegration) GetInvestor(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	params := GetInvestorParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.InvestorID == "" {
		return nil, fmt.Errorf("investor_id is required")
	}

	response, err := i.makeRequest(ctx, "/investors/"+params.InvestorID, nil)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (i *StartupsWatchIntegration) ListInvestors(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListInvestorsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	queryParams := map[string]string{}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/investors", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}

func (i *StartupsWatchIntegration) SearchInvestors(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := SearchInvestorsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	queryParams := map[string]string{
		"q": params.Query,
	}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/investors/search", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}

func (i *StartupsWatchIntegration) ListInvestments(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListInvestmentsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	queryParams := map[string]string{}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/investments", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}

func (i *StartupsWatchIntegration) ListAcquisitions(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListAcquisitionsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	queryParams := map[string]string{}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/acquisitions", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}

func (i *StartupsWatchIntegration) ListEvents(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	params := ListEventsParams{}
	if err := i.binder.BindToStruct(ctx, item, &params, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	queryParams := map[string]string{}
	if params.Page > 0 {
		queryParams["page"] = strconv.Itoa(params.Page)
	}
	if params.Limit > 0 {
		queryParams["limit"] = strconv.Itoa(params.Limit)
	}

	response, err := i.makeRequest(ctx, "/events", queryParams)
	if err != nil {
		return nil, err
	}

	return []domain.Item{response}, nil
}
