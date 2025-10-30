package domain

type PaginationParams struct {
	Limit           int    `json:"limit,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	PageToken       string `json:"page_token,omitempty"`
	BeforeID        string `json:"before_id,omitempty"`
	AfterID         string `json:"after_id,omitempty"`
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
	FirstID       string `json:"first_id,omitempty"`
	LastID        string `json:"last_id,omitempty"`
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

func (p *PeekParams) GetCursor() string {
	return p.Pagination.Cursor
}

func (p *PeekParams) SetCursor(cursor string) {
	p.Pagination.Cursor = cursor
}

func (p *PeekParams) GetLimit(defaultLimit int) int {
	if p.Pagination.Limit > 0 {
		return p.Pagination.Limit
	}
	return defaultLimit
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

func (r *PeekResult) GetCursor() string {
	if r.Pagination.Cursor != "" {
		return r.Pagination.Cursor
	}
	return r.Pagination.NextCursor
}

func (r *PeekResult) SetCursor(cursor string) {
	r.Pagination.Cursor = cursor
	r.Pagination.NextCursor = cursor
}

func (r *PeekResult) GetHasMore() bool {
	return r.Pagination.HasMore
}

func (r *PeekResult) SetHasMore(hasMore bool) {
	r.Pagination.HasMore = hasMore
}

func (r *PeekResult) SetPaginationMetadata(metadata PaginationMetadata) {
	r.Pagination = metadata
}

func (p *PeekParams) GetPageToken() string {
	return p.Pagination.PageToken
}

func (p *PeekParams) SetPageToken(pageToken string) {
	p.Pagination.PageToken = pageToken
}

func (r *PeekResult) GetPageToken() string {
	return r.Pagination.NextPageToken
}

func (r *PeekResult) SetPageToken(pageToken string) {
	r.Pagination.NextPageToken = pageToken
}

func (p *PeekParams) GetBeforeID() string {
	return p.Pagination.BeforeID
}

func (p *PeekParams) SetBeforeID(beforeID string) {
	p.Pagination.BeforeID = beforeID
}

func (r *PeekResult) GetFirstID() string {
	return r.Pagination.FirstID
}

func (r *PeekResult) SetFirstID(firstID string) {
	r.Pagination.FirstID = firstID
}

func (p *PeekParams) GetAfterID() string {
	return p.Pagination.AfterID
}

func (p *PeekParams) SetAfterID(afterID string) {
	p.Pagination.AfterID = afterID
}

func (r *PeekResult) GetLastID() string {
	return r.Pagination.LastID
}

func (r *PeekResult) SetLastID(lastID string) {
	r.Pagination.LastID = lastID
}

func (p *PeekParams) GetOffset() int {
	return p.Pagination.Offset
}

func (p *PeekParams) SetOffset(offset int) {
	p.Pagination.Offset = offset
}

func (r *PeekResult) GetOffset() int {
	return r.Pagination.Offset
}

func (r *PeekResult) SetOffset(offset int) {
	r.Pagination.Offset = offset
}

func (r *PeekResult) GetNextOffset() int {
	return r.Pagination.NextOffset
}

func (r *PeekResult) SetNextOffset(nextOffset int) {
	r.Pagination.NextOffset = nextOffset
}
