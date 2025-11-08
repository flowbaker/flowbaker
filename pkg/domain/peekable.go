package domain

type PaginationParams struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type PaginationMetadata struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
	NextOffset int    `json:"next_offset,omitempty"`
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
