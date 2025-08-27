package flowbaker

import "fmt"

// Error represents an error from the Flowbaker API
type Error struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Body       string `json:"body,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("flowbaker: %s (status: %d, request_id: %s)", e.Message, e.StatusCode, e.RequestID)
	}
	return fmt.Sprintf("flowbaker: %s (status: %d)", e.Message, e.StatusCode)
}

// IsRetryable returns true if the error might be resolved by retrying
func (e *Error) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429 // Server errors or rate limiting
}

// IsClientError returns true if the error is due to client input
func (e *Error) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsServerError returns true if the error is due to server issues
func (e *Error) IsServerError() bool {
	return e.StatusCode >= 500
}

// IsAuthError returns true if the error is related to authentication
func (e *Error) IsAuthError() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}

// IsRateLimited returns true if the error is due to rate limiting
func (e *Error) IsRateLimited() bool {
	return e.StatusCode == 429
}

// IsNotFound returns true if the resource was not found
func (e *Error) IsNotFound() bool {
	return e.StatusCode == 404
}

// Common error types
var (
	ErrInvalidCredentials = &Error{StatusCode: 401, Message: "invalid credentials"}
	ErrAccessDenied      = &Error{StatusCode: 403, Message: "access denied"}
	ErrNotFound          = &Error{StatusCode: 404, Message: "resource not found"}
	ErrRateLimited       = &Error{StatusCode: 429, Message: "rate limited"}
	ErrServerError       = &Error{StatusCode: 500, Message: "internal server error"}
	ErrBadGateway        = &Error{StatusCode: 502, Message: "bad gateway"}
	ErrServiceUnavailable = &Error{StatusCode: 503, Message: "service unavailable"}
	ErrGatewayTimeout    = &Error{StatusCode: 504, Message: "gateway timeout"}
)

// IsFlowbakerError checks if an error is a Flowbaker API error
func IsFlowbakerError(err error) (*Error, bool) {
	if e, ok := err.(*Error); ok {
		return e, true
	}
	return nil, false
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if e, ok := IsFlowbakerError(err); ok {
		return e.IsRetryable()
	}
	return false
}

// IsClientError checks if an error is a client error
func IsClientError(err error) bool {
	if e, ok := IsFlowbakerError(err); ok {
		return e.IsClientError()
	}
	return false
}

// IsAuthError checks if an error is an authentication error
func IsAuthError(err error) bool {
	if e, ok := IsFlowbakerError(err); ok {
		return e.IsAuthError()
	}
	return false
}