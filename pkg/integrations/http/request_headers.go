package http

import (
	"net/http"
	"strings"
)

type HTTPHeaderParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (h *httpActionManager) setRequestHeaders(req *http.Request, defaultContentType string, headers []HTTPHeaderParam) {
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
