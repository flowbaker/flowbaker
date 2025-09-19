package domain

import "time"

type OAuthAccount struct {
	ID          string
	WorkspaceID string
	OAuthName   string
	OAuthType   OAuthType
	Metadata    map[string]interface{}
	ClientID    string
}

type OAuthAccountSensitiveData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

type OAuthAccountWithSensitiveData struct {
	OAuthAccount
	SensitiveData OAuthAccountSensitiveData
}
