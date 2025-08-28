package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"flowbaker/internal/auth"
)

// ClientInterface defines the interface for the executor client
type ClientInterface interface {
	Execute(ctx context.Context, req *StartExecutionRequest) (*StartExecutionResponse, error)
	HandlePollingEvent(ctx context.Context, req *PollingEventRequest) (*PollingEventResponse, error)
	TestConnection(ctx context.Context, req *ConnectionTestRequest) (*ConnectionTestResponse, error)
	PeekData(ctx context.Context, req *PeekDataRequest) (*PeekDataResponse, error)
}

// Client provides methods to interact with the executor service
type Client struct {
	config     *ClientConfig
	httpClient *http.Client
	signer     *auth.APIRequestSigner
}

// NewClient creates a new executor client with the given options
func NewClient(options ...ClientOption) *Client {
	config := DefaultConfig()

	// Apply options
	for _, option := range options {
		option(config)
	}

	// Use provided HTTP client or create default one
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: config.Timeout,
		}
	}

	// Initialize signer if signing key is provided
	var signer *auth.APIRequestSigner
	if config.SigningKey != "" {
		var err error
		signer, err = auth.NewAPIRequestSigner(config.SigningKey)
		if err != nil {
			log.Fatalf("Failed to initialize request signer %s", err)
		}
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
		signer:     signer,
	}
}

// Execute sends a workflow execution request to the executor service
func (c *Client) Execute(ctx context.Context, req *StartExecutionRequest) (*StartExecutionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("execution request cannot be nil")
	}

	// Send request to executor
	resp, err := c.doRequest(ctx, "POST", "/executions", req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute workflow: %w", err)
	}

	var executionResult ExecutionResult

	if err := c.handleResponse(resp, &executionResult); err != nil {
		return nil, fmt.Errorf("failed to process execution response: %w", err)
	}

	return &StartExecutionResponse{
		ExecutionResult: executionResult,
	}, nil
}

// HandlePollingEvent sends a polling event request to the executor service
func (c *Client) HandlePollingEvent(ctx context.Context, req *PollingEventRequest) (*PollingEventResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("polling event request cannot be nil")
	}

	// Send request to executor
	resp, err := c.doRequest(ctx, "POST", "/polling-events", req)
	if err != nil {
		return nil, fmt.Errorf("failed to handle polling event: %w", err)
	}

	var pollingResponse PollingEventResponse

	if err := c.handleResponse(resp, &pollingResponse); err != nil {
		return nil, fmt.Errorf("failed to process polling response: %w", err)
	}

	return &pollingResponse, nil
}

// TestConnection sends a connection test request to the executor service
func (c *Client) TestConnection(ctx context.Context, req *ConnectionTestRequest) (*ConnectionTestResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("connection test request cannot be nil")
	}

	// Send request to executor
	resp, err := c.doRequest(ctx, "POST", "/connection-test", req)
	if err != nil {
		return nil, fmt.Errorf("failed to test connection: %w", err)
	}

	var connectionTestResponse ConnectionTestResponse

	if err := c.handleResponse(resp, &connectionTestResponse); err != nil {
		return nil, fmt.Errorf("failed to process connection test response: %w", err)
	}

	return &connectionTestResponse, nil
}

func (c *Client) PeekData(ctx context.Context, req *PeekDataRequest) (*PeekDataResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("peek data request cannot be nil")
	}

	resp, err := c.doRequest(ctx, "POST", "/peek-data", req)
	if err != nil {
		return nil, fmt.Errorf("failed to peek data: %w", err)
	}

	var peekDataResponse PeekDataResponse

	if err := c.handleResponse(resp, &peekDataResponse); err != nil {
		return nil, fmt.Errorf("failed to process peek data response: %w", err)
	}

	return &peekDataResponse, nil
}

// doRequest performs an HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	// Marshal body to bytes
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	url := c.config.BaseURL + path

	var lastErr error
	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}

		// Create request body reader
		var requestBody io.Reader
		if bodyBytes != nil {
			requestBody = bytes.NewBuffer(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Apply default headers
		for key, value := range c.config.DefaultHeaders {
			req.Header.Set(key, value)
		}

		// Apply user agent
		if c.config.UserAgent != "" {
			req.Header.Set("User-Agent", c.config.UserAgent)
		}

		// Add API signature if signer is available
		if c.signer != nil {
			signatureHeaders, err := c.signer.SignRequest(method, path, bodyBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to sign request: %w", err)
			}
			for key, value := range signatureHeaders {
				req.Header.Set(key, value)
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check for server errors that might be retryable
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = &Error{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("server error: %d", resp.StatusCode),
				Body:       string(body),
				RequestID:  resp.Header.Get("X-Request-ID"),
			}
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.config.RetryAttempts, lastErr)
}

// handleResponse processes the HTTP response and unmarshals JSON if successful
func (c *Client) handleResponse(resp *http.Response, result any) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errorResponse struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}

		// Try to parse error response
		if json.Unmarshal(body, &errorResponse) == nil {
			errorMsg := errorResponse.Error
			if errorMsg == "" {
				errorMsg = errorResponse.Message
			}
			if errorMsg != "" {
				return &Error{
					StatusCode: resp.StatusCode,
					Message:    errorMsg,
					Body:       string(body),
					RequestID:  resp.Header.Get("X-Request-ID"),
				}
			}
		}

		return &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
