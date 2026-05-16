package http

import "net/http"

type HTTPQueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (h *httpActionManager) setRequestQueryParams(req *http.Request, queryParams []HTTPQueryParam) {
	if len(queryParams) == 0 {
		return
	}

	query := req.URL.Query()

	for _, queryParam := range queryParams {
		if queryParam.Key == "" {
			continue
		}

		query.Add(queryParam.Key, queryParam.Value)
	}

	req.URL.RawQuery = query.Encode()
}
