package domain

type OAuthAccount struct {
	ID        string
	UserID    string
	OAuthName string
	OAuthType OAuthType
	Metadata  map[string]interface{}
	ClientID  string
}
