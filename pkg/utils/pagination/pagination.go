package pagination

import (
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/domain"
)


type Handler interface {
	GetType() domain.IntegrationPeekablePaginationType

	BuildRequestParams(params domain.PaginationParams) (map[string]interface{}, error)

	ParseResponseMetadata(response interface{}) (domain.PaginationMetadata, error)

	ValidateParams(params domain.PaginationParams) error
}


type CursorHandler struct {
	DefaultLimit int
	MaxLimit     int
}

func NewCursorHandler(config domain.PeekablePaginationConfig) *CursorHandler {
	return &CursorHandler{
		DefaultLimit: config.DefaultLimit,
		MaxLimit:     config.MaxLimit,
	}
}

func (h *CursorHandler) GetType() domain.IntegrationPeekablePaginationType {
	return domain.PeekablePaginationType_Cursor
}

func (h *CursorHandler) BuildRequestParams(params domain.PaginationParams) (map[string]interface{}, error) {
	limit := params.Limit
	if limit == 0 {
		limit = h.DefaultLimit
	}
	if h.MaxLimit > 0 && limit > h.MaxLimit {
		limit = h.MaxLimit
	}

	reqParams := map[string]interface{}{
		"limit": limit,
	}

	if params.Cursor != "" {
		reqParams["cursor"] = params.Cursor
	}

	if params.OrderBy != "" {
		reqParams["order_by"] = params.OrderBy
	}

	if params.IncludeArchived {
		reqParams["include_archived"] = true
	}

	return reqParams, nil
}

type cursorResponse struct {
	Cursor      string `json:"cursor"`
	NextCursor  string `json:"next_cursor"`
	EndCursor   string `json:"end_cursor"`
	HasMore     bool   `json:"has_more"`
	HasNextPage bool   `json:"has_next_page"`
	TotalCount  int    `json:"total_count"`
}

func (h *CursorHandler) ParseResponseMetadata(response interface{}) (domain.PaginationMetadata, error) {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	var resp cursorResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	metadata := domain.PaginationMetadata{}

	if resp.Cursor != "" {
		metadata.Cursor = resp.Cursor
		metadata.NextCursor = resp.Cursor
	} else if resp.NextCursor != "" {
		metadata.Cursor = resp.NextCursor
		metadata.NextCursor = resp.NextCursor
	} else if resp.EndCursor != "" {
		metadata.Cursor = resp.EndCursor
		metadata.NextCursor = resp.EndCursor
	}

	metadata.HasMore = resp.HasMore || resp.HasNextPage

	if resp.TotalCount > 0 {
		metadata.TotalCount = &resp.TotalCount
	}

	return metadata, nil
}

func (h *CursorHandler) ValidateParams(params domain.PaginationParams) error {
	if params.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if h.MaxLimit > 0 && params.Limit > h.MaxLimit {
		return fmt.Errorf("limit %d exceeds maximum %d", params.Limit, h.MaxLimit)
	}
	return nil
}


type PageTokenHandler struct {
	DefaultPageSize int
	MaxPageSize     int
}

func NewPageTokenHandler(config domain.PeekablePaginationConfig) *PageTokenHandler {
	return &PageTokenHandler{
		DefaultPageSize: config.DefaultLimit,
		MaxPageSize:     config.MaxLimit,
	}
}

func (h *PageTokenHandler) GetType() domain.IntegrationPeekablePaginationType {
	return domain.PeekablePaginationType_PageToken
}

func (h *PageTokenHandler) BuildRequestParams(params domain.PaginationParams) (map[string]interface{}, error) {
	pageSize := params.Limit
	if pageSize == 0 {
		pageSize = h.DefaultPageSize
	}
	if h.MaxPageSize > 0 && pageSize > h.MaxPageSize {
		pageSize = h.MaxPageSize
	}

	reqParams := map[string]interface{}{
		"pageSize": pageSize,
	}

	if params.PageToken != "" {
		reqParams["pageToken"] = params.PageToken
	}

	return reqParams, nil
}

type pageTokenResponse struct {
	NextPageToken      string `json:"nextPageToken"`
	NextPageTokenSnake string `json:"next_page_token"`
	TotalCount         int    `json:"totalCount"`
}

func (h *PageTokenHandler) ParseResponseMetadata(response interface{}) (domain.PaginationMetadata, error) {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	var resp pageTokenResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	metadata := domain.PaginationMetadata{}

	if resp.NextPageToken != "" {
		metadata.NextPageToken = resp.NextPageToken
		metadata.HasMore = true
	} else if resp.NextPageTokenSnake != "" {
		metadata.NextPageToken = resp.NextPageTokenSnake
		metadata.HasMore = true
	}

	if resp.TotalCount > 0 {
		metadata.TotalCount = &resp.TotalCount
	}

	return metadata, nil
}

func (h *PageTokenHandler) ValidateParams(params domain.PaginationParams) error {
	if params.Limit < 0 {
		return fmt.Errorf("page size cannot be negative")
	}
	if h.MaxPageSize > 0 && params.Limit > h.MaxPageSize {
		return fmt.Errorf("page size %d exceeds maximum %d", params.Limit, h.MaxPageSize)
	}
	return nil
}


type IDBasedHandler struct {
	DefaultLimit int
	Direction    domain.IntegrationPeekablePaginationType
}

func NewIDBasedHandler(config domain.PeekablePaginationConfig, direction domain.IntegrationPeekablePaginationType) *IDBasedHandler {
	return &IDBasedHandler{
		DefaultLimit: config.DefaultLimit,
		Direction:    direction,
	}
}

func (h *IDBasedHandler) GetType() domain.IntegrationPeekablePaginationType {
	return h.Direction
}

func (h *IDBasedHandler) BuildRequestParams(params domain.PaginationParams) (map[string]interface{}, error) {
	limit := params.Limit
	if limit == 0 {
		limit = h.DefaultLimit
	}

	reqParams := map[string]interface{}{
		"limit": limit,
	}

	if h.Direction == domain.PeekablePaginationType_IDBasedBefore && params.BeforeID != "" {
		reqParams["before_id"] = params.BeforeID
	} else if h.Direction == domain.PeekablePaginationType_IDBasedAfter && params.AfterID != "" {
		reqParams["after_id"] = params.AfterID
	}

	return reqParams, nil
}

type idBasedResponse struct {
	FirstID string `json:"first_id"`
	LastID  string `json:"last_id"`
	HasMore bool   `json:"has_more"`
}

func (h *IDBasedHandler) ParseResponseMetadata(response interface{}) (domain.PaginationMetadata, error) {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	var resp idBasedResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	metadata := domain.PaginationMetadata{
		FirstID: resp.FirstID,
		LastID:  resp.LastID,
		HasMore: resp.HasMore,
	}

	return metadata, nil
}

func (h *IDBasedHandler) ValidateParams(params domain.PaginationParams) error {
	if params.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}

	if params.BeforeID != "" && params.AfterID != "" {
		return fmt.Errorf("cannot specify both before_id and after_id")
	}

	return nil
}


type OffsetHandler struct {
	DefaultLimit int
	MaxLimit     int
}

func NewOffsetHandler(config domain.PeekablePaginationConfig) *OffsetHandler {
	return &OffsetHandler{
		DefaultLimit: config.DefaultLimit,
		MaxLimit:     config.MaxLimit,
	}
}

func (h *OffsetHandler) GetType() domain.IntegrationPeekablePaginationType {
	return domain.PeekablePaginationType_Offset
}

func (h *OffsetHandler) BuildRequestParams(params domain.PaginationParams) (map[string]interface{}, error) {
	limit := params.Limit
	if limit == 0 {
		limit = h.DefaultLimit
	}
	if h.MaxLimit > 0 && limit > h.MaxLimit {
		limit = h.MaxLimit
	}

	reqParams := map[string]interface{}{
		"limit":  limit,
		"offset": params.Offset,
	}

	return reqParams, nil
}

type offsetResponse struct {
	Offset     int `json:"offset"`
	Limit      int `json:"limit"`
	TotalCount int `json:"total_count"`
}

func (h *OffsetHandler) ParseResponseMetadata(response interface{}) (domain.PaginationMetadata, error) {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	var resp offsetResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return domain.PaginationMetadata{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	metadata := domain.PaginationMetadata{
		Offset:     resp.Offset,
		NextOffset: resp.Offset + resp.Limit,
	}

	if resp.TotalCount > 0 {
		metadata.TotalCount = &resp.TotalCount
		metadata.HasMore = metadata.NextOffset < resp.TotalCount
	}

	return metadata, nil
}

func (h *OffsetHandler) ValidateParams(params domain.PaginationParams) error {
	if params.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if params.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}
	if h.MaxLimit > 0 && params.Limit > h.MaxLimit {
		return fmt.Errorf("limit %d exceeds maximum %d", params.Limit, h.MaxLimit)
	}
	return nil
}


func NewHandler(paginationType domain.IntegrationPeekablePaginationType, config domain.PeekablePaginationConfig) (Handler, error) {
	switch paginationType {
	case domain.PeekablePaginationType_Cursor:
		return NewCursorHandler(config), nil

	case domain.PeekablePaginationType_PageToken:
		return NewPageTokenHandler(config), nil

	case domain.PeekablePaginationType_IDBasedBefore:
		return NewIDBasedHandler(config, domain.PeekablePaginationType_IDBasedBefore), nil

	case domain.PeekablePaginationType_IDBasedAfter:
		return NewIDBasedHandler(config, domain.PeekablePaginationType_IDBasedAfter), nil

	case domain.PeekablePaginationType_Offset:
		return NewOffsetHandler(config), nil

	case domain.PeekablePaginationType_None:
		return nil, fmt.Errorf("pagination type 'none' does not require a handler")

	default:
		return nil, fmt.Errorf("unknown pagination type: %s", paginationType)
	}
}
