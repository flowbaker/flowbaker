package dropbox

import (
	"encoding/json"
	"net/http"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type DropboxRequestProvider struct {
}

func NewDropboxRequestProvider(deps domain.IntegrationDeps) domain.HTTPRequestProvider {
	return &DropboxRequestProvider{}
}

type CredentialPayload struct {
	AccessToken string `json:"access_token"`
}

// TODO: check if it is working correctly
func (p *DropboxRequestProvider) AttachCredential(req *http.Request, credential *domain.Credential) (*http.Request, error) {
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
