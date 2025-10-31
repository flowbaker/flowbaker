package domain

type PaginationParams struct {
	Limit           int    `json:"limit,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	PageToken       string `json:"page_token,omitempty"`
	Offset          int    `json:"offset,omitempty"`
	OrderBy         string `json:"order_by,omitempty"`
	IncludeArchived bool   `json:"include_archived,omitempty"`
}

type PaginationMetadata struct {
	HasMore       bool   `json:"has_more"`
	TotalCount    *int   `json:"total_count,omitempty"`
	Cursor        string `json:"cursor,omitempty"`
	NextCursor    string `json:"next_cursor,omitempty"`
	NextPageToken string `json:"next_page_token,omitempty"`
	Offset        int    `json:"offset,omitempty"`
	NextOffset    int    `json:"next_offset,omitempty"`
}

type PeekParams struct {
	PeekableType IntegrationPeekableType
	PayloadJSON  []byte
	Pagination   PaginationParams
	Path         string
	UserID       string
	WorkspaceID  string
}

type PeekResult struct {
	ResultJSON []byte             `json:"result_json,omitempty"`
	Result     []PeekResultItem   `json:"result,omitempty"`
	Pagination PaginationMetadata `json:"pagination,omitempty"`
}

type PeekResultItem struct {
	Key     string `json:"key"`
	Value   string `json:"value,omitempty"`
	Content string `json:"content,omitempty"`
}

func (p *PeekParams) GetLimitWithMax(defaultLimit, maxLimit int) int {
	limit := p.Pagination.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	return limit
}
