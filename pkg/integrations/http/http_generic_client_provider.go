package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type HTTPClientProviderGeneric struct{}

func NewHTTPClientProviderGeneric() *HTTPClientProviderGeneric {
	return &HTTPClientProviderGeneric{}
}

type HTTPDecryptionResult struct {
	AuthType        HTTPGenericAuthType `json:"generic_auth_type"`
	Username        string              `json:"username"`
	Password        string              `json:"password"`
	BearerToken     string              `json:"bearer_token"`
	QueryAuthKey    string              `json:"query_auth_key"`
	QueryAuthValue  string              `json:"query_auth_value"`
	HeaderAuthKey   string              `json:"header_auth_key"`
	HeaderAuthValue string              `json:"header_auth_value"`
}

func (p *HTTPClientProviderGeneric) GetHTTPDefaultClientGeneric(ctx context.Context, decryptionResult HTTPDecryptionResult) (*http.Client, error) {
	transport := &http.Transport{}
	client := &http.Client{
		Transport: transport,
	}

	switch decryptionResult.AuthType {
	case HTTPGenericAuthType_Basic:
		client.Transport = &basicAuthTransport{
			username: decryptionResult.Username,
			password: decryptionResult.Password,
			base:     transport,
		}

	case HTTPGenericAuthType_Bearer:
		client.Transport = &bearerAuthTransport{
			token: decryptionResult.BearerToken,
			base:  transport,
		}

	case HTTPGenericAuthType_Query:
		client.Transport = &queryAuthTransport{
			key:   decryptionResult.QueryAuthKey,
			value: decryptionResult.QueryAuthValue,
			base:  transport,
		}

	case HTTPGenericAuthType_Header:
		client.Transport = &headerAuthTransport{
			key:   decryptionResult.HeaderAuthKey,
			value: decryptionResult.HeaderAuthValue,
			base:  transport,
		}

	case HTTPGenericAuthType_Custom:
		client.Transport = &customAuthTransport{
			headers: map[string]string{
				decryptionResult.HeaderAuthKey: decryptionResult.HeaderAuthValue,
			},
			base: transport,
		}

	default:
		return nil, errors.New("invalid auth type")
	}

	return client, nil
}

type basicAuthTransport struct {
	username string
	password string
	base     http.RoundTripper
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(req)
}

type bearerAuthTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

type queryAuthTransport struct {
	key   string
	value string
	base  http.RoundTripper
}

func (t *queryAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()
	q.Set(t.key, t.value)
	req.URL.RawQuery = q.Encode()
	return t.base.RoundTrip(req)
}

type headerAuthTransport struct {
	key   string
	value string
	base  http.RoundTripper
}

func (t *headerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	formattedKey := formatHeaderKey(t.key)
	if formattedKey == "" {
		return nil, fmt.Errorf("invalid header key: %s", t.key)
	}

	req.Header.Set(formattedKey, t.value)

	return t.base.RoundTrip(req)
}

func formatHeaderKey(key string) string {
	parts := strings.Fields(key)
	if len(parts) == 0 {
		return ""
	}

	formattedParts := make([]string, len(parts))
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		formattedParts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}

	return strings.Join(formattedParts, "-")
}

type customAuthTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *customAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	return t.base.RoundTrip(req)
}
