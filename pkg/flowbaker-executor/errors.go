package executor

import "fmt"

// Error represents an error from the executor service
type Error struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id"`
	Body       string `json:"body"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("executor error (status %d, request %s): %s", e.StatusCode, e.RequestID, e.Message)
	}
	return fmt.Sprintf("executor error (status %d): %s", e.StatusCode, e.Message)
}

// IsRetryable returns true if the error is retryable
func (e *Error) IsRetryable() bool {
	// Server errors (5xx) are generally retryable
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// IsClientError returns true if the error is a client error
func (e *Error) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsServerError returns true if the error is a server error
func (e *Error) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}