package jira

// JiraAccessibleResource represents a Jira cloud instance accessible to the user
type JiraAccessibleResource struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	AvatarURL string   `json:"avatarUrl"`
	Scopes    []string `json:"scopes"`
}

// JiraOAuthMetadata represents the metadata stored in OAuth account
type JiraOAuthMetadata struct {
	UserInfo            map[string]interface{}   `json:"user_info"`
	AccessibleResources []JiraAccessibleResource `json:"accessible_resources"`
	AccountID           string                   `json:"account_id"`
	Email               string                   `json:"email"`
	DisplayName         string                   `json:"display_name"`
	SelectedCloudID     string                   `json:"selected_cloud_id,omitempty"`
}