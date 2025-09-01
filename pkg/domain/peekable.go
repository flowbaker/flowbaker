package domain

type PeekParams struct {
	PeekableType IntegrationPeekableType
	PayloadJSON  []byte
	Cursor       string
	Path         string
	UserID       string
	WorkspaceID  string
}

type PeekResult struct {
	ResultJSON []byte
	Result     []PeekResultItem
	Cursor     string
	HasMore    bool
}

type PeekResultItem struct {
	Key     string `json:"key"`
	Value   string `json:"value,omitempty"`
	Content string `json:"content,omitempty"`
}
