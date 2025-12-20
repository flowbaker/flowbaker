package types

import "errors"

var (
	// ErrProviderNotSet is returned when a provider is not configured
	ErrProviderNotSet = errors.New("provider not set")

	// ErrInvalidMessage is returned when a message is invalid
	ErrInvalidMessage = errors.New("invalid message")

	// ErrToolNotFound is returned when a tool is not found
	ErrToolNotFound = errors.New("tool not found")

	// ErrToolExecutionFailed is returned when tool execution fails
	ErrToolExecutionFailed = errors.New("tool execution failed")

	// ErrMaxIterationsReached is returned when max iterations are reached
	ErrMaxIterationsReached = errors.New("max iterations reached")

	// ErrContextCanceled is returned when context is canceled
	ErrContextCanceled = errors.New("context canceled")

	// ErrEmptyResponse is returned when the provider returns an empty response
	ErrEmptyResponse = errors.New("empty response from provider")
)
