package gmail

import (
	"encoding/json"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type GmailRequestProvider struct {
}

func NewGmailRequestProvider(deps domain.IntegrationDeps) domain.HTTPRequestProvider {
	return &GmailRequestProvider{}
}

type CredentialPayload struct {
	AccessToken string `json:"access_token"`
}

func (p *GmailRequestProvider) Attach(req *http.Request, credential *domain.Credential) (*http.Request, error) {
	payload := CredentialPayload{}
	jsonPayload, err := json.Marshal(credential.DecryptedPayload)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonPayload, &payload)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+payload.AccessToken)
	return req, nil
}
