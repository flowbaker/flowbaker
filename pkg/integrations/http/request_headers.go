package http

import (
	"net/http"
	"strings"
)

func (i *HTTPIntegration) setRequestHeaders(req *http.Request, defaultContentType string, headers []HTTPHeaderParam) {
	if strings.TrimSpace(defaultContentType) != "" {
		req.Header.Set("Content-Type", defaultContentType)
	}

	for _, header := range headers {
		if header.Key == "" {
			continue
		}

		req.Header.Set(header.Key, header.Value)
	}
}
