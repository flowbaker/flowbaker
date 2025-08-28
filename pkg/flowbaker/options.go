package flowbaker

import (
	"net/http"
	"time"
)

// ClientOption represents an option for configuring the Flowbaker client
type ClientOption func(*ClientConfig)

// ClientConfig holds the configuration for the Flowbaker client
type ClientConfig struct {
	BaseURL           string
	ExecutorID        string
	Ed25519PrivateKey string // Base64-encoded Ed25519 private key for signing requests
	Timeout           time.Duration
	RetryAttempts     int
	RetryDelay        time.Duration
	DefaultHeaders    map[string]string
	HTTPClient        *http.Client
	UserAgent         string
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL:       "https://api.flowbaker.io",
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
		DefaultHeaders: map[string]string{
			"Content-Type": "application/json",
		},
		UserAgent: "flowbaker-go-sdk/1.0.0",
	}
}

// WithBaseURL sets the base URL for the Flowbaker API
func WithBaseURL(baseURL string) ClientOption {
	return func(c *ClientConfig) {
		c.BaseURL = baseURL
	}
}

// WithEd25519PrivateKey sets the Ed25519 private key for request signing
func WithEd25519PrivateKey(privateKey string) ClientOption {
	return func(c *ClientConfig) {
		c.Ed25519PrivateKey = privateKey
	}
}

// WithExecutorID sets the executor ID for credential requests
func WithExecutorID(executorID string) ClientOption {
	return func(c *ClientConfig) {
		c.ExecutorID = executorID
	}
}

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.Timeout = timeout
	}
}

// WithRetry sets the retry configuration
func WithRetry(attempts int, delay time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.RetryAttempts = attempts
		c.RetryDelay = delay
	}
}

// WithHeader adds a default header to all requests
func WithHeader(key, value string) ClientOption {
	return func(c *ClientConfig) {
		if c.DefaultHeaders == nil {
			c.DefaultHeaders = make(map[string]string)
		}
		c.DefaultHeaders[key] = value
	}
}

// WithHeaders sets multiple default headers
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *ClientConfig) {
		if c.DefaultHeaders == nil {
			c.DefaultHeaders = make(map[string]string)
		}
		for key, value := range headers {
			c.DefaultHeaders[key] = value
		}
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *ClientConfig) {
		c.HTTPClient = httpClient
	}
}

// WithUserAgent sets a custom user agent
func WithUserAgent(userAgent string) ClientOption {
	return func(c *ClientConfig) {
		c.UserAgent = userAgent
	}
}

// ClientBuilder provides a fluent interface for building Flowbaker clients
type ClientBuilder struct {
	config *ClientConfig
}

// NewClientBuilder creates a new client builder with default configuration
func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		config: DefaultConfig(),
	}
}

// WithBaseURL sets the base URL for the Flowbaker API
func (b *ClientBuilder) WithBaseURL(baseURL string) *ClientBuilder {
	b.config.BaseURL = baseURL
	return b
}

// WithEd25519PrivateKey sets the Ed25519 private key for request signing
func (b *ClientBuilder) WithEd25519PrivateKey(privateKey string) *ClientBuilder {
	b.config.Ed25519PrivateKey = privateKey
	return b
}

// WithExecutorID sets the executor ID for credential requests
func (b *ClientBuilder) WithExecutorID(executorID string) *ClientBuilder {
	b.config.ExecutorID = executorID
	return b
}

// WithTimeout sets the request timeout
func (b *ClientBuilder) WithTimeout(timeout time.Duration) *ClientBuilder {
	b.config.Timeout = timeout
	return b
}

// WithRetry sets the retry configuration
func (b *ClientBuilder) WithRetry(attempts int, delay time.Duration) *ClientBuilder {
	b.config.RetryAttempts = attempts
	b.config.RetryDelay = delay
	return b
}

// WithHeader adds a default header to all requests
func (b *ClientBuilder) WithHeader(key, value string) *ClientBuilder {
	if b.config.DefaultHeaders == nil {
		b.config.DefaultHeaders = make(map[string]string)
	}
	b.config.DefaultHeaders[key] = value
	return b
}

// WithHeaders sets multiple default headers
func (b *ClientBuilder) WithHeaders(headers map[string]string) *ClientBuilder {
	if b.config.DefaultHeaders == nil {
		b.config.DefaultHeaders = make(map[string]string)
	}
	for key, value := range headers {
		b.config.DefaultHeaders[key] = value
	}
	return b
}

// WithHTTPClient sets a custom HTTP client
func (b *ClientBuilder) WithHTTPClient(httpClient *http.Client) *ClientBuilder {
	b.config.HTTPClient = httpClient
	return b
}

// WithUserAgent sets a custom user agent
func (b *ClientBuilder) WithUserAgent(userAgent string) *ClientBuilder {
	b.config.UserAgent = userAgent
	return b
}

// Build creates the Flowbaker client with the configured settings
func (b *ClientBuilder) Build() *Client {
	return NewClient(
		WithBaseURL(b.config.BaseURL),
		WithEd25519PrivateKey(b.config.Ed25519PrivateKey),
		WithExecutorID(b.config.ExecutorID),
		WithTimeout(b.config.Timeout),
		WithRetry(b.config.RetryAttempts, b.config.RetryDelay),
		WithHeaders(b.config.DefaultHeaders),
		WithHTTPClient(b.config.HTTPClient),
		WithUserAgent(b.config.UserAgent),
	)
}