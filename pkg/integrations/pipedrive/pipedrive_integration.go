package pipedriveintegration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	PipedriveIntegrationActionType_CreateDeal         domain.IntegrationActionType = "create_deal"
	PipedriveIntegrationActionType_UpdateDeal         domain.IntegrationActionType = "update_deal"
	PipedriveIntegrationActionType_GetDeal            domain.IntegrationActionType = "get_deal"
	PipedriveIntegrationActionType_DeleteDeal         domain.IntegrationActionType = "delete_deal"
	PipedriveIntegrationActionType_ListDeals          domain.IntegrationActionType = "list_deals"
	PipedriveIntegrationActionType_CreatePerson       domain.IntegrationActionType = "create_person"
	PipedriveIntegrationActionType_UpdatePerson       domain.IntegrationActionType = "update_person"
	PipedriveIntegrationActionType_GetPerson          domain.IntegrationActionType = "get_person"
	PipedriveIntegrationActionType_DeletePerson       domain.IntegrationActionType = "delete_person"
	PipedriveIntegrationActionType_ListPersons        domain.IntegrationActionType = "list_persons"
	PipedriveIntegrationActionType_CreateOrganization domain.IntegrationActionType = "create_organization"
	PipedriveIntegrationActionType_UpdateOrganization domain.IntegrationActionType = "update_organization"
	PipedriveIntegrationActionType_GetOrganization    domain.IntegrationActionType = "get_organization"
	PipedriveIntegrationActionType_DeleteOrganization domain.IntegrationActionType = "delete_organization"
	PipedriveIntegrationActionType_ListOrganizations  domain.IntegrationActionType = "list_organizations"
	PipedriveIntegrationActionType_CreateActivity     domain.IntegrationActionType = "create_activity"
	PipedriveIntegrationActionType_UpdateActivity     domain.IntegrationActionType = "update_activity"
	PipedriveIntegrationActionType_GetActivity        domain.IntegrationActionType = "get_activity"
	PipedriveIntegrationActionType_DeleteActivity     domain.IntegrationActionType = "delete_activity"
	PipedriveIntegrationActionType_ListActivities     domain.IntegrationActionType = "list_activities"
	PipedriveIntegrationActionType_CreateNote         domain.IntegrationActionType = "create_note"
	PipedriveIntegrationActionType_UpdateNote         domain.IntegrationActionType = "update_note"
	PipedriveIntegrationActionType_GetNote            domain.IntegrationActionType = "get_note"
	PipedriveIntegrationActionType_DeleteNote         domain.IntegrationActionType = "delete_note"
	PipedriveIntegrationActionType_ListNotes          domain.IntegrationActionType = "list_notes"
	PipedriveIntegrationActionType_CreateProduct      domain.IntegrationActionType = "create_product"
	PipedriveIntegrationActionType_UpdateProduct      domain.IntegrationActionType = "update_product"
	PipedriveIntegrationActionType_GetProduct         domain.IntegrationActionType = "get_product"
	PipedriveIntegrationActionType_DeleteProduct      domain.IntegrationActionType = "delete_product"
	PipedriveIntegrationActionType_ListProducts       domain.IntegrationActionType = "list_products"
	PipedriveIntegrationActionType_CreateLead         domain.IntegrationActionType = "create_lead"
	PipedriveIntegrationActionType_UpdateLead         domain.IntegrationActionType = "update_lead"
	PipedriveIntegrationActionType_GetLead            domain.IntegrationActionType = "get_lead"
	PipedriveIntegrationActionType_DeleteLead         domain.IntegrationActionType = "delete_lead"
	PipedriveIntegrationActionType_ListLeads          domain.IntegrationActionType = "list_leads"

	PipedriveIntegrationPeekable_Pipelines     domain.IntegrationPeekableType = "pipelines"
	PipedriveIntegrationPeekable_Stages        domain.IntegrationPeekableType = "stages"
	PipedriveIntegrationPeekable_Users         domain.IntegrationPeekableType = "users"
	PipedriveIntegrationPeekable_Persons       domain.IntegrationPeekableType = "persons"
	PipedriveIntegrationPeekable_Organizations domain.IntegrationPeekableType = "organizations"
	PipedriveIntegrationPeekable_Currencies    domain.IntegrationPeekableType = "currencies"
	PipedriveIntegrationPeekable_Deals         domain.IntegrationPeekableType = "deals"
	PipedriveIntegrationPeekable_Activities    domain.IntegrationPeekableType = "activities"
	PipedriveIntegrationPeekable_ActivityTypes domain.IntegrationPeekableType = "activity_types"
	PipedriveIntegrationPeekable_Products      domain.IntegrationPeekableType = "products"
	PipedriveIntegrationPeekable_Projects      domain.IntegrationPeekableType = "projects"
	PipedriveIntegrationPeekable_Leads         domain.IntegrationPeekableType = "leads"
	PipedriveIntegrationPeekable_Labels        domain.IntegrationPeekableType = "labels"
)

type PipedriveIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[PipedriveCredential]
}

func NewPipedriveIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &PipedriveIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[PipedriveCredential](deps.ExecutorCredentialManager),
	}
}

func (c *PipedriveIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewPipedriveIntegration(ctx, PipedriveIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

type PipedriveIntegration struct {
	apiToken string
	binder   domain.IntegrationParameterBinder

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type PipedriveCredential struct {
	APIToken string `json:"api_token"`
}

type PipedriveIntegrationDependencies struct {
	CredentialID     string
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[PipedriveCredential]
}

func NewPipedriveIntegration(ctx context.Context, deps PipedriveIntegrationDependencies) (*PipedriveIntegration, error) {
	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	integration := &PipedriveIntegration{
		apiToken: credential.APIToken,
		binder:   deps.ParameterBinder,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(PipedriveIntegrationActionType_CreateDeal, integration.CreateDeal).
		AddPerItem(PipedriveIntegrationActionType_UpdateDeal, integration.UpdateDeal).
		AddPerItem(PipedriveIntegrationActionType_GetDeal, integration.GetDeal).
		AddPerItem(PipedriveIntegrationActionType_DeleteDeal, integration.DeleteDeal).
		AddPerItemMulti(PipedriveIntegrationActionType_ListDeals, integration.ListDeals).
		AddPerItem(PipedriveIntegrationActionType_CreatePerson, integration.CreatePerson).
		AddPerItem(PipedriveIntegrationActionType_UpdatePerson, integration.UpdatePerson).
		AddPerItem(PipedriveIntegrationActionType_GetPerson, integration.GetPerson).
		AddPerItem(PipedriveIntegrationActionType_DeletePerson, integration.DeletePerson).
		AddPerItemMulti(PipedriveIntegrationActionType_ListPersons, integration.ListPersons).
		AddPerItem(PipedriveIntegrationActionType_CreateOrganization, integration.CreateOrganization).
		AddPerItem(PipedriveIntegrationActionType_UpdateOrganization, integration.UpdateOrganization).
		AddPerItem(PipedriveIntegrationActionType_GetOrganization, integration.GetOrganization).
		AddPerItem(PipedriveIntegrationActionType_DeleteOrganization, integration.DeleteOrganization).
		AddPerItemMulti(PipedriveIntegrationActionType_ListOrganizations, integration.ListOrganizations).
		AddPerItem(PipedriveIntegrationActionType_CreateActivity, integration.CreateActivity).
		AddPerItem(PipedriveIntegrationActionType_UpdateActivity, integration.UpdateActivity).
		AddPerItem(PipedriveIntegrationActionType_GetActivity, integration.GetActivity).
		AddPerItem(PipedriveIntegrationActionType_DeleteActivity, integration.DeleteActivity).
		AddPerItemMulti(PipedriveIntegrationActionType_ListActivities, integration.ListActivities).
		AddPerItem(PipedriveIntegrationActionType_CreateNote, integration.CreateNote).
		AddPerItem(PipedriveIntegrationActionType_UpdateNote, integration.UpdateNote).
		AddPerItem(PipedriveIntegrationActionType_GetNote, integration.GetNote).
		AddPerItem(PipedriveIntegrationActionType_DeleteNote, integration.DeleteNote).
		AddPerItem(PipedriveIntegrationActionType_ListNotes, integration.ListNotes).
		AddPerItem(PipedriveIntegrationActionType_CreateProduct, integration.CreateProduct).
		AddPerItem(PipedriveIntegrationActionType_UpdateProduct, integration.UpdateProduct).
		AddPerItem(PipedriveIntegrationActionType_GetProduct, integration.GetProduct).
		AddPerItem(PipedriveIntegrationActionType_DeleteProduct, integration.DeleteProduct).
		AddPerItem(PipedriveIntegrationActionType_ListProducts, integration.ListProducts).
		AddPerItem(PipedriveIntegrationActionType_CreateLead, integration.CreateLead).
		AddPerItem(PipedriveIntegrationActionType_UpdateLead, integration.UpdateLead).
		AddPerItem(PipedriveIntegrationActionType_GetLead, integration.GetLead).
		AddPerItem(PipedriveIntegrationActionType_DeleteLead, integration.DeleteLead).
		AddPerItem(PipedriveIntegrationActionType_ListLeads, integration.ListLeads)

	integration.actionManager = actionManager

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		PipedriveIntegrationPeekable_Pipelines:     integration.PeekPipelines,
		PipedriveIntegrationPeekable_Stages:        integration.PeekStages,
		PipedriveIntegrationPeekable_Users:         integration.PeekUsers,
		PipedriveIntegrationPeekable_Persons:       integration.PeekPersons,
		PipedriveIntegrationPeekable_Organizations: integration.PeekOrganizations,
		PipedriveIntegrationPeekable_Currencies:    integration.PeekCurrencies,
		PipedriveIntegrationPeekable_Deals:         integration.PeekDeals,
		PipedriveIntegrationPeekable_Activities:    integration.PeekActivities,
		PipedriveIntegrationPeekable_ActivityTypes: integration.PeekActivityTypes,
		PipedriveIntegrationPeekable_Products:      integration.PeekProducts,
		PipedriveIntegrationPeekable_Projects:      integration.PeekProjects,
		PipedriveIntegrationPeekable_Leads:         integration.PeekLeads,
		PipedriveIntegrationPeekable_Labels:        integration.PeekLabels,
	}

	integration.peekFuncs = peekFuncs

	return integration, nil
}

func (i *PipedriveIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *PipedriveIntegration) makeRequestV1(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	return i.makeRequest(ctx, method, endpoint, body, PipedriveVersionV1)
}

func (i *PipedriveIntegration) makeRequestV2(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	return i.makeRequest(ctx, method, endpoint, body, PipedriveVersionV2)
}

type PipedriveVersion string

const (
	PipedriveVersionV1 PipedriveVersion = "v1"
	PipedriveVersionV2 PipedriveVersion = "v2"
)

func (i *PipedriveIntegration) makeRequest(ctx context.Context, method, endpoint string, body any, version PipedriveVersion) ([]byte, error) {
	separator := "?"
	if strings.Contains(endpoint, "?") {
		separator = "&"
	}

	var url string
	if version == PipedriveVersionV1 {
		url = fmt.Sprintf("https://api.pipedrive.com/%s%s%sapi_token=%s", version, endpoint, separator, i.apiToken)
	} else {
		url = fmt.Sprintf("https://api.pipedrive.com/api/v2%s%sapi_token=%s", endpoint, separator, i.apiToken)
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pipedrive API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type CreateDealParams struct {
	Title      string `json:"title"`
	Value      string `json:"value,omitempty"`
	Currency   string `json:"currency,omitempty"`
	UserID     int    `json:"user_id,omitempty"`
	PersonID   int    `json:"person_id,omitempty"`
	OrgID      int    `json:"org_id,omitempty"`
	PipelineID int    `json:"pipeline_id,omitempty"`
	StageID    int    `json:"stage_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

func (i *PipedriveIntegration) CreateDeal(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateDealParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	respBody, err := i.makeRequestV2(ctx, "POST", "/deals", p)
	if err != nil {
		return nil, fmt.Errorf("failed to create deal: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type UpdateDealParams struct {
	ID                int    `json:"id"`
	Title             string `json:"title,omitempty"`
	Value             int    `json:"value,omitempty"`
	Currency          string `json:"currency,omitempty"`
	OwnerID           int    `json:"owner_id,omitempty"`
	PersonID          int    `json:"person_id,omitempty"`
	OrgID             int    `json:"org_id,omitempty"`
	PipelineID        int    `json:"pipeline_id,omitempty"`
	StageID           int    `json:"stage_id,omitempty"`
	Status            string `json:"status,omitempty"`
	LostReason        string `json:"lost_reason,omitempty"`
	VisibleTo         int    `json:"visible_to,omitempty"`
	ExpectedCloseDate string `json:"expected_close_date,omitempty"`
	Label             []int  `json:"label,omitempty"`
	Channel           int    `json:"channel,omitempty"`
	ChannelID         string `json:"channel_id,omitempty"`
	CloseTime         string `json:"close_time,omitempty"`
}

type UpdateDealRequest struct {
	Title             string `json:"title,omitempty"`
	Value             int    `json:"value,omitempty"`
	Currency          string `json:"currency,omitempty"`
	OwnerID           int    `json:"owner_id,omitempty"`
	PersonID          int    `json:"person_id,omitempty"`
	OrgID             int    `json:"org_id,omitempty"`
	PipelineID        int    `json:"pipeline_id,omitempty"`
	StageID           int    `json:"stage_id,omitempty"`
	Status            string `json:"status,omitempty"`
	LostReason        string `json:"lost_reason,omitempty"`
	VisibleTo         int    `json:"visible_to,omitempty"`
	ExpectedCloseDate string `json:"expected_close_date,omitempty"`
	Label             []int  `json:"label,omitempty"`
	Channel           int    `json:"channel,omitempty"`
	ChannelID         string `json:"channel_id,omitempty"`
	CloseTime         string `json:"close_time,omitempty"`
}

func (i *PipedriveIntegration) UpdateDeal(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateDealParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	request := UpdateDealRequest{
		Title:             p.Title,
		Value:             p.Value,
		Currency:          p.Currency,
		OwnerID:           p.OwnerID,
		PersonID:          p.PersonID,
		OrgID:             p.OrgID,
		PipelineID:        p.PipelineID,
		StageID:           p.StageID,
		Status:            p.Status,
		LostReason:        p.LostReason,
		VisibleTo:         p.VisibleTo,
		ExpectedCloseDate: p.ExpectedCloseDate,
		Label:             p.Label,
		Channel:           p.Channel,
		CloseTime:         p.CloseTime,
		ChannelID:         p.ChannelID,
	}

	endpoint := fmt.Sprintf("/deals/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "PATCH", endpoint, request)
	if err != nil {
		return nil, fmt.Errorf("failed to update deal: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type GetDealParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) GetDeal(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetDealParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/deals/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get deal: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type DeleteDealParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) DeleteDeal(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteDealParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/deals/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete deal: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type ListDealsParams struct {
	OwnerID       int      `json:"owner_id,omitempty"`
	PersonID      int      `json:"person_id,omitempty"`
	OrgID         int      `json:"org_id,omitempty"`
	PipelineID    int      `json:"pipeline_id,omitempty"`
	StageID       int      `json:"stage_id,omitempty"`
	Status        string   `json:"status,omitempty"`
	UpdatedSince  string   `json:"updated_since,omitempty"`
	UpdatedUntil  string   `json:"updated_until,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"`
	SortDirection string   `json:"sort_direction,omitempty"`
	IncludeFields []string `json:"include_fields,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
}

type ListDealsResponse struct {
	Data []map[string]interface{} `json:"data"`
}

func (i *PipedriveIntegration) ListDeals(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := ListDealsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	queryParams := url.Values{}

	if p.OwnerID > 0 {
		queryParams.Add("owner_id", strconv.Itoa(p.OwnerID))
	}
	if p.PersonID > 0 {
		queryParams.Add("person_id", strconv.Itoa(p.PersonID))
	}
	if p.OrgID > 0 {
		queryParams.Add("org_id", strconv.Itoa(p.OrgID))
	}
	if p.PipelineID > 0 {
		queryParams.Add("pipeline_id", strconv.Itoa(p.PipelineID))
	}
	if p.StageID > 0 {
		queryParams.Add("stage_id", strconv.Itoa(p.StageID))
	}
	if p.Status != "" {
		queryParams.Add("status", p.Status)
	}
	if p.UpdatedSince != "" {
		queryParams.Add("updated_since", p.UpdatedSince)
	}
	if p.UpdatedUntil != "" {
		queryParams.Add("updated_until", p.UpdatedUntil)
	}
	if p.SortBy != "" {
		queryParams.Add("sort_by", p.SortBy)
	}
	if p.SortDirection != "" {
		queryParams.Add("sort_direction", p.SortDirection)
	}
	if len(p.IncludeFields) > 0 {
		for _, field := range p.IncludeFields {
			queryParams.Add("include_fields", field)
		}
	}

	if p.Limit > 0 {
		queryParams.Add("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		queryParams.Add("cursor", p.Cursor)
	}

	endpoint := "/deals"
	if len(queryParams) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())
	}

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list deals: %w", err)
	}

	var response ListDealsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var items []domain.Item
	for _, deal := range response.Data {
		items = append(items, deal)
	}

	return items, nil
}

type CreatePersonParams struct {
	Name   string   `json:"name"`
	Emails []string `json:"emails,omitempty"`
	Phones []string `json:"phones,omitempty"`
	OrgID  int      `json:"org_id,omitempty"`
}

type CreatePersonRequest struct {
	Name   string              `json:"name"`
	Emails []CreatePersonEmail `json:"emails,omitempty"`
	Phones []CreatePersonPhone `json:"phones,omitempty"`
	OrgID  int                 `json:"org_id,omitempty"`
}

type CreatePersonPhone struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary,omitempty"`
	Label   string `json:"label,omitempty"`
}

type CreatePersonEmail struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary,omitempty"`
	Label   string `json:"label,omitempty"`
}

func (i *PipedriveIntegration) CreatePerson(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreatePersonParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	phones := []CreatePersonPhone{}
	for _, phone := range p.Phones {
		phones = append(phones, CreatePersonPhone{
			Value: phone,
		})
	}

	emails := []CreatePersonEmail{}
	for _, email := range p.Emails {
		emails = append(emails, CreatePersonEmail{
			Value: email,
		})
	}

	request := CreatePersonRequest{
		Name:   p.Name,
		Emails: emails,
		Phones: phones,
		OrgID:  p.OrgID,
	}

	respBody, err := i.makeRequestV2(ctx, "POST", "/persons", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create person: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type UpdatePersonParams struct {
	ID     int      `json:"id"`
	Name   string   `json:"name,omitempty"`
	Emails []string `json:"email,omitempty"`
	Phones []string `json:"phone,omitempty"`
	OrgID  int      `json:"org_id,omitempty"`
}

type UpdatePersonRequest struct {
	Name   string   `json:"name,omitempty"`
	Emails []string `json:"email,omitempty"`
	Phones []string `json:"phone,omitempty"`
	OrgID  int      `json:"org_id,omitempty"`
}

func (i *PipedriveIntegration) UpdatePerson(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdatePersonParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/persons/%d", p.ID)

	request := UpdatePersonRequest{
		Name:   p.Name,
		Emails: p.Emails,
		Phones: p.Phones,
		OrgID:  p.OrgID,
	}

	respBody, err := i.makeRequestV2(ctx, "PATCH", endpoint, request)
	if err != nil {
		return nil, fmt.Errorf("failed to update person: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type GetPersonParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) GetPerson(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetPersonParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/persons/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get person: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type CreateOrganizationParams struct {
	Name      string `json:"name"`
	OwnerID   int    `json:"owner_id,omitempty"`
	VisibleTo int    `json:"visible_to,omitempty"`
	Address   string `json:"address,omitempty"`
}

type CreateOrganizationRequest struct {
	Name      string  `json:"name"`
	OwnerID   int     `json:"owner_id,omitempty"`
	VisibleTo int     `json:"visible_to,omitempty"`
	Address   Address `json:"address,omitempty"`
}

type Address struct {
	Value string `json:"value,omitempty"`
}

type OrganizationOutputItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	AddTime    string `json:"add_time,omitempty"`
	UpdateTime string `json:"update_time,omitempty"`
}

func (i *PipedriveIntegration) CreateOrganization(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateOrganizationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	address := Address{
		Value: p.Address,
	}

	request := CreateOrganizationRequest{
		Name:      p.Name,
		OwnerID:   p.OwnerID,
		VisibleTo: p.VisibleTo,
		Address:   address,
	}
	respBody, err := i.makeRequestV2(ctx, "POST", "/organizations", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type GetOrganizationParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) GetOrganization(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetOrganizationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/organizations/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type UpdateOrganizationParams struct {
	ID        int    `json:"id"`
	Name      string `json:"name,omitempty"`
	OwnerID   int    `json:"owner_id,omitempty"`
	VisibleTo int    `json:"visible_to,omitempty"`
	Address   string `json:"address,omitempty"`
}

type UpdateOrganizationRequest struct {
	Name      string  `json:"name,omitempty"`
	OwnerID   int     `json:"owner_id,omitempty"`
	VisibleTo int     `json:"visible_to,omitempty"`
	Address   Address `json:"address,omitempty"`
}

func (i *PipedriveIntegration) UpdateOrganization(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateOrganizationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	address := Address{
		Value: p.Address,
	}

	endpoint := fmt.Sprintf("/organizations/%d", p.ID)
	request := UpdateOrganizationRequest{
		Name:      p.Name,
		OwnerID:   p.OwnerID,
		VisibleTo: p.VisibleTo,
		Address:   address,
	}
	respBody, err := i.makeRequestV2(ctx, "PATCH", endpoint, request)
	if err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type DeletePersonParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) DeletePerson(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeletePersonParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/persons/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete person: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type ListPersonsParams struct {
	QueryMode     string   `json:"query_mode,omitempty"`
	FilterID      int      `json:"filter_id,omitempty"`
	IDs           string   `json:"ids,omitempty"`
	OwnerID       int      `json:"owner_id,omitempty"`
	OrgID         int      `json:"org_id,omitempty"`
	UpdatedSince  string   `json:"updated_since,omitempty"`
	UpdatedUntil  string   `json:"updated_until,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"`
	SortDirection string   `json:"sort_direction,omitempty"`
	IncludeFields []string `json:"include_fields,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
}

type ListPersonsResponse struct {
	Data []map[string]interface{} `json:"data"`
}

func (i *PipedriveIntegration) ListPersons(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := ListPersonsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	queryParams := url.Values{}

	if p.OwnerID > 0 {
		queryParams.Add("owner_id", strconv.Itoa(p.OwnerID))
	}
	if p.OrgID > 0 {
		queryParams.Add("org_id", strconv.Itoa(p.OrgID))
	}
	if p.UpdatedSince != "" {
		queryParams.Add("updated_since", p.UpdatedSince)
	}
	if p.UpdatedUntil != "" {
		queryParams.Add("updated_until", p.UpdatedUntil)
	}
	if p.SortBy != "" {
		queryParams.Add("sort_by", p.SortBy)
	}
	if p.SortDirection != "" {
		queryParams.Add("sort_direction", p.SortDirection)
	}
	if len(p.IncludeFields) > 0 {
		for _, field := range p.IncludeFields {
			queryParams.Add("include_fields", field)
		}
	}

	if p.Limit > 0 {
		queryParams.Add("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		queryParams.Add("cursor", p.Cursor)
	}

	endpoint := "/persons"
	if len(queryParams) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())
	}

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list persons: %w", err)
	}

	var response ListPersonsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var items []domain.Item
	for _, person := range response.Data {
		items = append(items, person)
	}

	return items, nil
}

type DeleteOrganizationParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) DeleteOrganization(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteOrganizationParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/organizations/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete organization: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type ListOrganizationsParams struct {
	OwnerID       int      `json:"owner_id,omitempty"`
	UpdatedSince  string   `json:"updated_since,omitempty"`
	UpdatedUntil  string   `json:"updated_until,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"`
	SortDirection string   `json:"sort_direction,omitempty"`
	IncludeFields []string `json:"include_fields,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
}

type ListOrganizationsResponse struct {
	Data []map[string]interface{} `json:"data"`
}

func (i *PipedriveIntegration) ListOrganizations(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := ListOrganizationsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	queryParams := url.Values{}

	if p.OwnerID > 0 {
		queryParams.Add("owner_id", strconv.Itoa(p.OwnerID))
	}
	if p.UpdatedSince != "" {
		queryParams.Add("updated_since", p.UpdatedSince)
	}
	if p.UpdatedUntil != "" {
		queryParams.Add("updated_until", p.UpdatedUntil)
	}
	if p.SortBy != "" {
		queryParams.Add("sort_by", p.SortBy)
	}
	if p.SortDirection != "" {
		queryParams.Add("sort_direction", p.SortDirection)
	}
	if len(p.IncludeFields) > 0 {
		for _, field := range p.IncludeFields {
			queryParams.Add("include_fields", field)
		}
	}

	if p.Limit > 0 {
		queryParams.Add("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		queryParams.Add("cursor", p.Cursor)
	}

	endpoint := "/organizations"
	if len(queryParams) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())
	}

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}

	var response ListOrganizationsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var items []domain.Item
	for _, org := range response.Data {
		items = append(items, org)
	}

	return items, nil
}

type CreateActivityParams struct {
	Subject           string `json:"subject"`
	Type              string `json:"type,omitempty"`
	OwnerID           int    `json:"owner_id,omitempty"`
	DealID            int    `json:"deal_id,omitempty"`
	OrgID             int    `json:"org_id,omitempty"`
	ProjectID         int    `json:"project_id,omitempty"`
	DueDate           string `json:"due_date,omitempty"`
	DueTime           string `json:"due_time,omitempty"`
	Duration          string `json:"duration,omitempty"`
	Busy              bool   `json:"busy,omitempty"`
	Done              bool   `json:"done,omitempty"`
	PublicDescription string `json:"public_description,omitempty"`
	Note              string `json:"note,omitempty"`
	ParticipantID     int    `json:"participant_id,omitempty"`
}

type CreateActivityRequest struct {
	Subject           string        `json:"subject"`
	Type              string        `json:"type,omitempty"`
	OwnerID           int           `json:"owner_id,omitempty"`
	DealID            int           `json:"deal_id,omitempty"`
	OrgID             int           `json:"org_id,omitempty"`
	ProjectID         int           `json:"project_id,omitempty"`
	DueDate           string        `json:"due_date,omitempty"`
	DueTime           string        `json:"due_time,omitempty"`
	Duration          string        `json:"duration,omitempty"`
	Busy              bool          `json:"busy,omitempty"`
	Done              bool          `json:"done,omitempty"`
	PublicDescription string        `json:"public_description,omitempty"`
	Note              string        `json:"note,omitempty"`
	Participants      []Participant `json:"participants,omitempty"`
}

type Participant struct {
	PersonID int  `json:"person_id,omitempty"`
	Primary  bool `json:"primary,omitempty"`
}

func (i *PipedriveIntegration) CreateActivity(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateActivityParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	participants := []Participant{}
	if p.ParticipantID > 0 {
		participants = append(participants, Participant{
			PersonID: p.ParticipantID,
			Primary:  true,
		})
	}
	request := CreateActivityRequest{
		Subject:           p.Subject,
		Type:              p.Type,
		OwnerID:           p.OwnerID,
		DealID:            p.DealID,
		OrgID:             p.OrgID,
		ProjectID:         p.ProjectID,
		DueDate:           p.DueDate,
		DueTime:           p.DueTime,
		Duration:          p.Duration,
		Busy:              p.Busy,
		Done:              p.Done,
		PublicDescription: p.PublicDescription,
		Note:              p.Note,
		Participants:      participants,
	}

	respBody, err := i.makeRequestV2(ctx, "POST", "/activities", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type UpdateActivityParams struct {
	ID                int    `json:"id"`
	Subject           string `json:"subject,omitempty"`
	Type              string `json:"type,omitempty"`
	OwnerID           int    `json:"owner_id,omitempty"`
	DealID            int    `json:"deal_id,omitempty"`
	OrgID             int    `json:"org_id,omitempty"`
	ProjectID         int    `json:"project_id,omitempty"`
	DueDate           string `json:"due_date,omitempty"`
	DueTime           string `json:"due_time,omitempty"`
	Duration          string `json:"duration,omitempty"`
	Busy              bool   `json:"busy,omitempty"`
	Done              bool   `json:"done,omitempty"`
	PublicDescription string `json:"public_description,omitempty"`
	Note              string `json:"note,omitempty"`
	ParticipantID     int    `json:"participant_id,omitempty"`
}

type UpdateActivityRequest struct {
	Subject           string        `json:"subject,omitempty"`
	Type              string        `json:"type,omitempty"`
	OwnerID           int           `json:"owner_id,omitempty"`
	DealID            int           `json:"deal_id,omitempty"`
	OrgID             int           `json:"org_id,omitempty"`
	ProjectID         int           `json:"project_id,omitempty"`
	DueDate           string        `json:"due_date,omitempty"`
	DueTime           string        `json:"due_time,omitempty"`
	Duration          string        `json:"duration,omitempty"`
	Busy              bool          `json:"busy,omitempty"`
	Done              bool          `json:"done,omitempty"`
	PublicDescription string        `json:"public_description,omitempty"`
	Note              string        `json:"note,omitempty"`
	Participants      []Participant `json:"participants,omitempty"`
}

func (i *PipedriveIntegration) UpdateActivity(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateActivityParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	participants := []Participant{}
	if p.ParticipantID > 0 {
		participants = append(participants, Participant{
			PersonID: p.ParticipantID,
			Primary:  true,
		})
	}
	request := UpdateActivityRequest{
		Subject:           p.Subject,
		Type:              p.Type,
		OwnerID:           p.OwnerID,
		DealID:            p.DealID,
		OrgID:             p.OrgID,
		ProjectID:         p.ProjectID,
		DueDate:           p.DueDate,
		DueTime:           p.DueTime,
		Duration:          p.Duration,
		Busy:              p.Busy,
		Done:              p.Done,
		PublicDescription: p.PublicDescription,
		Note:              p.Note,
		Participants:      participants,
	}

	endpoint := fmt.Sprintf("/activities/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "PATCH", endpoint, request)
	if err != nil {
		return nil, fmt.Errorf("failed to update activity: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type GetActivityParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) GetActivity(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetActivityParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/activities/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get activity: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type DeleteActivityParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) DeleteActivity(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteActivityParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/activities/%d", p.ID)
	respBody, err := i.makeRequestV2(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete activity: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type ListActivitiesParams struct {
	OwnerID       int      `json:"owner_id,omitempty"`
	DealID        int      `json:"deal_id,omitempty"`
	PersonID      int      `json:"person_id,omitempty"`
	OrgID         int      `json:"org_id,omitempty"`
	Done          string   `json:"done,omitempty"`
	UpdatedSince  string   `json:"updated_since,omitempty"`
	UpdatedUntil  string   `json:"updated_until,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"`
	SortDirection string   `json:"sort_direction,omitempty"`
	IncludeFields []string `json:"include_fields,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
}

type ListActivitiesResponse struct {
	Data []map[string]interface{} `json:"data"`
}

func (i *PipedriveIntegration) ListActivities(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := ListActivitiesParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	queryParams := url.Values{}

	if p.OwnerID > 0 {
		queryParams.Add("owner_id", strconv.Itoa(p.OwnerID))
	}
	if p.DealID > 0 {
		queryParams.Add("deal_id", strconv.Itoa(p.DealID))
	}
	if p.PersonID > 0 {
		queryParams.Add("person_id", strconv.Itoa(p.PersonID))
	}
	if p.OrgID > 0 {
		queryParams.Add("org_id", strconv.Itoa(p.OrgID))
	}
	if p.Done != "" && p.Done != "all" {
		queryParams.Add("done", p.Done)
	}
	if p.UpdatedSince != "" {
		queryParams.Add("updated_since", p.UpdatedSince)
	}
	if p.UpdatedUntil != "" {
		queryParams.Add("updated_until", p.UpdatedUntil)
	}
	if p.SortBy != "" {
		queryParams.Add("sort_by", p.SortBy)
	}
	if p.SortDirection != "" {
		queryParams.Add("sort_direction", p.SortDirection)
	}
	if len(p.IncludeFields) > 0 {
		for _, field := range p.IncludeFields {
			queryParams.Add("include_fields", field)
		}
	}

	if p.Limit > 0 {
		queryParams.Add("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		queryParams.Add("cursor", p.Cursor)
	}

	endpoint := "/activities"
	if len(queryParams) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())
	}

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list activities: %w", err)
	}

	var response ListActivitiesResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var items []domain.Item
	for _, activity := range response.Data {
		items = append(items, activity)
	}

	return items, nil
}

type CreateNoteParams struct {
	Content  string `json:"content"`
	DealID   int    `json:"deal_id,omitempty"`
	PersonID int    `json:"person_id,omitempty"`
	OrgID    int    `json:"org_id,omitempty"`
	LeadID   string `json:"lead_id,omitempty"`
}

// BELOW FUNCTIONS ARE NOT USED IN THE CURRENT VERSION OF THE INTEGRATION!!!

func (i *PipedriveIntegration) CreateNote(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateNoteParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	respBody, err := i.makeRequestV1(ctx, "POST", "/notes", p)
	if err != nil {
		return nil, fmt.Errorf("failed to create note: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type UpdateNoteParams struct {
	ID       int    `json:"id"`
	Content  string `json:"content,omitempty"`
	DealID   int    `json:"deal_id,omitempty"`
	PersonID int    `json:"person_id,omitempty"`
	OrgID    int    `json:"org_id,omitempty"`
}

func (i *PipedriveIntegration) UpdateNote(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateNoteParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/notes/%d", p.ID)
	respBody, err := i.makeRequestV1(ctx, "PUT", endpoint, p)
	if err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type GetNoteParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) GetNote(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetNoteParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/notes/%d", p.ID)
	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type DeleteNoteParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) DeleteNote(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteNoteParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/notes/%d", p.ID)
	respBody, err := i.makeRequestV1(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete note: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type ListNotesParams struct {
	Limit    int `json:"limit,omitempty"`
	Start    int `json:"start,omitempty"`
	DealID   int `json:"deal_id,omitempty"`
	PersonID int `json:"person_id,omitempty"`
	OrgID    int `json:"org_id,omitempty"`
}

func (i *PipedriveIntegration) ListNotes(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListNotesParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := "/notes"
	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type CreateProductParams struct {
	Name   string  `json:"name"`
	Code   string  `json:"code,omitempty"`
	Unit   string  `json:"unit,omitempty"`
	Tax    float64 `json:"tax,omitempty"`
	Prices []struct {
		Price    float64 `json:"price"`
		Currency string  `json:"currency"`
		Cost     float64 `json:"cost,omitempty"`
	} `json:"prices,omitempty"`
}

func (i *PipedriveIntegration) CreateProduct(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateProductParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	respBody, err := i.makeRequestV1(ctx, "POST", "/products", p)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type UpdateProductParams struct {
	ID     int     `json:"id"`
	Name   string  `json:"name,omitempty"`
	Code   string  `json:"code,omitempty"`
	Unit   string  `json:"unit,omitempty"`
	Tax    float64 `json:"tax,omitempty"`
	Prices []struct {
		Price    float64 `json:"price"`
		Currency string  `json:"currency"`
		Cost     float64 `json:"cost,omitempty"`
	} `json:"prices,omitempty"`
}

func (i *PipedriveIntegration) UpdateProduct(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateProductParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/products/%d", p.ID)
	respBody, err := i.makeRequestV1(ctx, "PUT", endpoint, p)
	if err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type GetProductParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) GetProduct(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetProductParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/products/%d", p.ID)
	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type DeleteProductParams struct {
	ID int `json:"id"`
}

func (i *PipedriveIntegration) DeleteProduct(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteProductParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/products/%d", p.ID)
	respBody, err := i.makeRequestV1(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete product: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type ListProductsParams struct {
	Limit int `json:"limit,omitempty"`
	Start int `json:"start,omitempty"`
}

func (i *PipedriveIntegration) ListProducts(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListProductsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := "/products"
	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type CreateLeadParams struct {
	Title    string `json:"title"`
	PersonID int    `json:"person_id,omitempty"`
	OrgID    int    `json:"organization_id,omitempty"`
	Value    struct {
		Amount   float64 `json:"amount,omitempty"`
		Currency string  `json:"currency,omitempty"`
	} `json:"value,omitempty"`
	LabelIDs []string `json:"label_ids,omitempty"`
}

func (i *PipedriveIntegration) CreateLead(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateLeadParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	respBody, err := i.makeRequestV1(ctx, "POST", "/leads", p)
	if err != nil {
		return nil, fmt.Errorf("failed to create lead: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type UpdateLeadParams struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	PersonID int    `json:"person_id,omitempty"`
	OrgID    int    `json:"organization_id,omitempty"`
	Value    struct {
		Amount   float64 `json:"amount,omitempty"`
		Currency string  `json:"currency,omitempty"`
	} `json:"value,omitempty"`
	LabelIDs []string `json:"label_ids,omitempty"`
}

func (i *PipedriveIntegration) UpdateLead(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateLeadParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/leads/%s", p.ID)
	respBody, err := i.makeRequestV1(ctx, "PATCH", endpoint, p)
	if err != nil {
		return nil, fmt.Errorf("failed to update lead: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type GetLeadParams struct {
	ID string `json:"id"`
}

func (i *PipedriveIntegration) GetLead(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetLeadParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/leads/%s", p.ID)
	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get lead: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type DeleteLeadParams struct {
	ID string `json:"id"`
}

func (i *PipedriveIntegration) DeleteLead(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteLeadParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("/leads/%s", p.ID)
	respBody, err := i.makeRequestV1(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete lead: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

type ListLeadsParams struct {
	Limit int `json:"limit,omitempty"`
	Start int `json:"start,omitempty"`
}

func (i *PipedriveIntegration) ListLeads(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListLeadsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	endpoint := "/leads"
	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list leads: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

func (i *PipedriveIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

type PeekPipelinesResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
	AdditionalData struct {
		NextCursor string `json:"next_cursor"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekPipelines(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	cursor := params.Pagination.Cursor

	endpoint := "/pipelines"
	queryParams := url.Values{}
	queryParams.Add("limit", strconv.Itoa(limit))
	if cursor != "" {
		queryParams.Add("cursor", cursor)
	}
	endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get pipelines: %w", err)
	}

	var response PeekPipelinesResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, pipeline := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", pipeline.ID),
			Value:   fmt.Sprintf("%d", pipeline.ID),
			Content: pipeline.Name,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	result.Pagination.Cursor = response.AdditionalData.NextCursor
	result.Pagination.NextCursor = response.AdditionalData.NextCursor
	result.Pagination.HasMore = response.AdditionalData.NextCursor != ""

	return result, nil
}

type PeekStagesResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
	AdditionalData struct {
		NextCursor string `json:"next_cursor"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekStages(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	cursor := params.Pagination.Cursor

	endpoint := "/stages"
	queryParams := url.Values{}
	queryParams.Add("limit", strconv.Itoa(limit))
	if cursor != "" {
		queryParams.Add("cursor", cursor)
	}
	endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get stages: %w", err)
	}

	var response PeekStagesResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, stage := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", stage.ID),
			Value:   fmt.Sprintf("%d", stage.ID),
			Content: stage.Name,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	result.Pagination.Cursor = response.AdditionalData.NextCursor
	result.Pagination.NextCursor = response.AdditionalData.NextCursor
	result.Pagination.HasMore = response.AdditionalData.NextCursor != ""

	return result, nil
}

type PeekUsersResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"data"`
	AdditionalData struct {
		MoreItemsInCollection bool `json:"more_items_in_collection"`
		NextStart             int  `json:"next_start"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekUsers(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	offset := params.Pagination.Offset

	endpoint := fmt.Sprintf("/users?start=%d&limit=%d", offset, limit)

	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get users: %w", err)
	}

	var response PeekUsersResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, user := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", user.ID),
			Value:   fmt.Sprintf("%d", user.ID),
			Content: fmt.Sprintf("%s (%s)", user.Name, user.Email),
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	if response.AdditionalData.MoreItemsInCollection {
		result.Pagination.NextOffset = response.AdditionalData.NextStart
	}
	result.Pagination.HasMore = response.AdditionalData.MoreItemsInCollection

	return result, nil
}

type PeekPersonsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email []struct {
			Value string `json:"value"`
		} `json:"email"`
	} `json:"data"`
	AdditionalData struct {
		NextCursor string `json:"next_cursor"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekPersons(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	cursor := params.Pagination.Cursor

	endpoint := "/persons"
	queryParams := url.Values{}
	queryParams.Add("limit", strconv.Itoa(limit))
	if cursor != "" {
		queryParams.Add("cursor", cursor)
	}
	endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get persons: %w", err)
	}

	var response PeekPersonsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, person := range response.Data {
		content := person.Name
		if len(person.Email) > 0 && person.Email[0].Value != "" {
			content = fmt.Sprintf("%s (%s)", person.Name, person.Email[0].Value)
		}
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", person.ID),
			Value:   fmt.Sprintf("%d", person.ID),
			Content: content,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	result.Pagination.Cursor = response.AdditionalData.NextCursor
	result.Pagination.NextCursor = response.AdditionalData.NextCursor
	result.Pagination.HasMore = response.AdditionalData.NextCursor != ""

	return result, nil
}

type PeekOrganizationsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
	AdditionalData struct {
		NextCursor string `json:"next_cursor"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekOrganizations(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	cursor := params.Pagination.Cursor

	endpoint := "/organizations"
	queryParams := url.Values{}
	queryParams.Add("limit", strconv.Itoa(limit))
	if cursor != "" {
		queryParams.Add("cursor", cursor)
	}
	endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get organizations: %w", err)
	}

	var response PeekOrganizationsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, org := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", org.ID),
			Value:   fmt.Sprintf("%d", org.ID),
			Content: org.Name,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	result.Pagination.Cursor = response.AdditionalData.NextCursor
	result.Pagination.NextCursor = response.AdditionalData.NextCursor
	result.Pagination.HasMore = response.AdditionalData.NextCursor != ""

	return result, nil
}

type PeekCurrenciesResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	} `json:"data"`
	AdditionalData struct {
		MoreItemsInCollection bool `json:"more_items_in_collection"`
		NextStart             int  `json:"next_start"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekCurrencies(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	offset := params.Pagination.Offset

	endpoint := fmt.Sprintf("/currencies?start=%d&limit=%d", offset, limit)

	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get currencies: %w", err)
	}

	var response PeekCurrenciesResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, currency := range response.Data {
		content := fmt.Sprintf("%s (%s)", currency.Code, currency.Name)
		if currency.Symbol != "" {
			content = fmt.Sprintf("%s - %s (%s)", currency.Code, currency.Name, currency.Symbol)
		}
		results = append(results, domain.PeekResultItem{
			Key:     currency.Code,
			Value:   currency.Code,
			Content: content,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	if response.AdditionalData.MoreItemsInCollection {
		result.Pagination.NextOffset = response.AdditionalData.NextStart
	}
	result.Pagination.HasMore = response.AdditionalData.MoreItemsInCollection

	return result, nil
}

type PeekDealsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	} `json:"data"`
	AdditionalData struct {
		NextCursor string `json:"next_cursor"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekDeals(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	cursor := params.Pagination.Cursor

	endpoint := "/deals"
	queryParams := url.Values{}
	queryParams.Add("limit", strconv.Itoa(limit))
	if cursor != "" {
		queryParams.Add("cursor", cursor)
	}
	endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get deals: %w", err)
	}

	var response PeekDealsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, deal := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", deal.ID),
			Value:   fmt.Sprintf("%d", deal.ID),
			Content: deal.Title,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	result.Pagination.Cursor = response.AdditionalData.NextCursor
	result.Pagination.NextCursor = response.AdditionalData.NextCursor
	result.Pagination.HasMore = response.AdditionalData.NextCursor != ""

	return result, nil
}

type PeekActivitiesResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID      int    `json:"id"`
		Subject string `json:"subject"`
	} `json:"data"`
	AdditionalData struct {
		NextCursor string `json:"next_cursor"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekActivities(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	cursor := params.Pagination.Cursor

	endpoint := "/activities"
	queryParams := url.Values{}
	queryParams.Add("limit", strconv.Itoa(limit))
	if cursor != "" {
		queryParams.Add("cursor", cursor)
	}
	endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get activities: %w", err)
	}

	var response PeekActivitiesResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, activity := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", activity.ID),
			Value:   fmt.Sprintf("%d", activity.ID),
			Content: activity.Subject,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	result.Pagination.Cursor = response.AdditionalData.NextCursor
	result.Pagination.NextCursor = response.AdditionalData.NextCursor
	result.Pagination.HasMore = response.AdditionalData.NextCursor != ""

	return result, nil
}

type PeekActivityTypesResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		KeyString string `json:"key_string"`
		Name      string `json:"name"`
	} `json:"data"`
	AdditionalData struct {
		MoreItemsInCollection bool `json:"more_items_in_collection"`
		NextStart             int  `json:"next_start"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekActivityTypes(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	offset := params.Pagination.Offset

	endpoint := fmt.Sprintf("/activityTypes?start=%d&limit=%d", offset, limit)

	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get activity types: %w", err)
	}

	var response PeekActivityTypesResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, activityType := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     activityType.KeyString,
			Value:   activityType.KeyString,
			Content: activityType.Name,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	if response.AdditionalData.MoreItemsInCollection {
		result.Pagination.NextOffset = response.AdditionalData.NextStart
	}
	result.Pagination.HasMore = response.AdditionalData.MoreItemsInCollection

	return result, nil
}

type PeekProductsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
	AdditionalData struct {
		NextCursor string `json:"next_cursor"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekProducts(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	cursor := params.Pagination.Cursor

	endpoint := "/products"
	queryParams := url.Values{}
	queryParams.Add("limit", strconv.Itoa(limit))
	if cursor != "" {
		queryParams.Add("cursor", cursor)
	}
	endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())

	respBody, err := i.makeRequestV2(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get products: %w", err)
	}

	var response PeekProductsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, product := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", product.ID),
			Value:   fmt.Sprintf("%d", product.ID),
			Content: product.Name,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	result.Pagination.Cursor = response.AdditionalData.NextCursor
	result.Pagination.NextCursor = response.AdditionalData.NextCursor
	result.Pagination.HasMore = response.AdditionalData.NextCursor != ""

	return result, nil
}

type PeekProjectsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	} `json:"data"`
	AdditionalData struct {
		MoreItemsInCollection bool `json:"more_items_in_collection"`
		NextStart             int  `json:"next_start"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekProjects(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	offset := params.Pagination.Offset

	endpoint := fmt.Sprintf("/projects?start=%d&limit=%d", offset, limit)

	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get projects: %w", err)
	}

	var response PeekProjectsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, project := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", project.ID),
			Value:   fmt.Sprintf("%d", project.ID),
			Content: project.Title,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	if response.AdditionalData.MoreItemsInCollection {
		result.Pagination.NextOffset = response.AdditionalData.NextStart
	}
	result.Pagination.HasMore = response.AdditionalData.MoreItemsInCollection

	return result, nil
}

type PeekLeadsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	} `json:"data"`
	AdditionalData struct {
		MoreItemsInCollection bool `json:"more_items_in_collection"`
		NextStart             int  `json:"next_start"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekLeads(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	offset := params.Pagination.Offset

	endpoint := fmt.Sprintf("/leads?start=%d&limit=%d", offset, limit)

	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get leads: %w", err)
	}

	var response PeekLeadsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, lead := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", lead.ID),
			Value:   fmt.Sprintf("%d", lead.ID),
			Content: lead.Title,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	if response.AdditionalData.MoreItemsInCollection {
		result.Pagination.NextOffset = response.AdditionalData.NextStart
	}
	result.Pagination.HasMore = response.AdditionalData.MoreItemsInCollection

	return result, nil
}

type PeekLabelsResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
	AdditionalData struct {
		MoreItemsInCollection bool `json:"more_items_in_collection"`
		NextStart             int  `json:"next_start"`
	} `json:"additional_data"`
}

func (i *PipedriveIntegration) PeekLabels(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	limit := params.GetLimitWithMax(20, 500)
	offset := params.Pagination.Offset

	endpoint := fmt.Sprintf("/labels?start=%d&limit=%d", offset, limit)

	respBody, err := i.makeRequestV1(ctx, "GET", endpoint, nil)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get labels: %w", err)
	}

	var response PeekLabelsResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var results []domain.PeekResultItem
	for _, label := range response.Data {
		results = append(results, domain.PeekResultItem{
			Key:     fmt.Sprintf("%d", label.ID),
			Value:   fmt.Sprintf("%d", label.ID),
			Content: label.Name,
		})
	}

	result := domain.PeekResult{
		Result: results,
	}
	if response.AdditionalData.MoreItemsInCollection {
		result.Pagination.NextOffset = response.AdditionalData.NextStart
	}
	result.Pagination.HasMore = response.AdditionalData.MoreItemsInCollection

	return result, nil
}
