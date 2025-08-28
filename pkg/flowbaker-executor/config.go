package executor

import (
	"net/http"
	"time"
)

// ClientConfig holds the configuration for the executor client
type ClientConfig struct {
	BaseURL        string
	HTTPClient     *http.Client
	Timeout        time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
	DefaultHeaders map[string]string
	UserAgent      string
	SigningKey     string // Base64 encoded Ed25519 private key for API request signing
}

// DefaultConfig returns the default configuration
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL:        "http://localhost:8081",
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     time.Second,
		DefaultHeaders: map[string]string{"Content-Type": "application/json"},
		UserAgent:      "flowbaker-executor-client/1.0",
	}
}

// ClientOption is a function that modifies ClientConfig
type ClientOption func(*ClientConfig)

// WithBaseURL sets the base URL for the executor service
func WithBaseURL(baseURL string) ClientOption {
	return func(c *ClientConfig) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *ClientConfig) {
		c.HTTPClient = client
	}
}

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.Timeout = timeout
	}
}

// WithRetryAttempts sets the number of retry attempts
func WithRetryAttempts(attempts int) ClientOption {
	return func(c *ClientConfig) {
		c.RetryAttempts = attempts
	}
}

// WithRetryDelay sets the delay between retry attempts
func WithRetryDelay(delay time.Duration) ClientOption {
	return func(c *ClientConfig) {
		c.RetryDelay = delay
	}
}

// WithHeaders sets custom default headers
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *ClientConfig) {
		c.DefaultHeaders = headers
	}
}

// WithUserAgent sets the user agent string
func WithUserAgent(userAgent string) ClientOption {
	return func(c *ClientConfig) {
		c.UserAgent = userAgent
	}
}

// WithSigningKey sets the API signing private key
func WithSigningKey(signingKey string) ClientOption {
	return func(c *ClientConfig) {
		c.SigningKey = signingKey
	}
}

// ClientBuilder provides a fluent interface for building a client
type ClientBuilder struct {
	config *ClientConfig
}

// NewClientBuilder creates a new client builder
func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		config: DefaultConfig(),
	}
}

// WithBaseURL sets the base URL
func (b *ClientBuilder) WithBaseURL(baseURL string) *ClientBuilder {
	b.config.BaseURL = baseURL
	return b
}

// WithHTTPClient sets the HTTP client
func (b *ClientBuilder) WithHTTPClient(client *http.Client) *ClientBuilder {
	b.config.HTTPClient = client
	return b
}

// WithTimeout sets the timeout
func (b *ClientBuilder) WithTimeout(timeout time.Duration) *ClientBuilder {
	b.config.Timeout = timeout
	return b
}

// WithRetryAttempts sets the retry attempts
func (b *ClientBuilder) WithRetryAttempts(attempts int) *ClientBuilder {
	b.config.RetryAttempts = attempts
	return b
}

// WithRetryDelay sets the retry delay
func (b *ClientBuilder) WithRetryDelay(delay time.Duration) *ClientBuilder {
	b.config.RetryDelay = delay
	return b
}

// WithHeaders sets custom headers
func (b *ClientBuilder) WithHeaders(headers map[string]string) *ClientBuilder {
	b.config.DefaultHeaders = headers
	return b
}

// WithUserAgent sets the user agent
func (b *ClientBuilder) WithUserAgent(userAgent string) *ClientBuilder {
	b.config.UserAgent = userAgent
	return b
}

// WithSigningKey sets the API signing private key
func (b *ClientBuilder) WithSigningKey(signingKey string) *ClientBuilder {
	b.config.SigningKey = signingKey
	return b
}

// Build creates the client with the configured options
func (b *ClientBuilder) Build() *Client {
	// Create HTTP client if not provided
	if b.config.HTTPClient == nil {
		b.config.HTTPClient = &http.Client{
			Timeout: b.config.Timeout,
		}
	}
	
	return &Client{
		config:     b.config,
		httpClient: b.config.HTTPClient,
	}
}