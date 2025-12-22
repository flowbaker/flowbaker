package flowbaker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// StreamEventMessage represents a single event in the stream (NDJSON format)
type StreamEventMessage struct {
	EventType string `json:"event_type"`
	Data      any    `json:"data"`
}

// StreamEventsResponse represents the response after streaming events
type StreamEventsResponse struct {
	Success         bool   `json:"success"`
	EventsPublished int    `json:"events_published"`
	Error           string `json:"error,omitempty"`
}

// CreateEventStreamRequest represents the request to create an event stream
type CreateEventStreamRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

// CreateEventStreamResult represents the result of creating an event stream
type CreateEventStreamResult struct {
	Writer *EventStreamWriter `json:"-"`
}

// EventStreamWriter implements a streaming event writer
// It buffers events and sends them as NDJSON over HTTP chunked transfer encoding
type EventStreamWriter struct {
	client      *Client
	workspaceID string
	ctx         context.Context
	cancelCtx   context.CancelFunc

	// Pipe for streaming data
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter

	// Internal state
	closed   bool
	mu       sync.Mutex
	wg       sync.WaitGroup
	response *StreamEventsResponse
	err      error
}

type NewEventStreamWriterParams struct {
	Client      *Client
	WorkspaceID string
	Ctx         context.Context
	CancelCtx   context.CancelFunc
	PipeReader  *io.PipeReader
	PipeWriter  *io.PipeWriter
}

func NewEventStreamWriter(params NewEventStreamWriterParams) (*EventStreamWriter, error) {

	if params.Client == nil {
		return nil, fmt.Errorf("client is required")
	}

	if params.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	writer := &EventStreamWriter{
		client:      params.Client,
		workspaceID: params.WorkspaceID,
		ctx:         params.Ctx,
		cancelCtx:   params.CancelCtx,
		pipeReader:  params.PipeReader,
		pipeWriter:  params.PipeWriter,
		mu:          sync.Mutex{},
	}

	return writer, nil
}

func (w *EventStreamWriter) Initialize() error {
	w.wg.Add(1)
	go w.initialize()

	return nil
}

// runStreamRequest runs the HTTP POST request with chunked transfer encoding
func (w *EventStreamWriter) initialize() {
	defer w.wg.Done()

	// Use the existing doRequestWithExecutorID method which handles Ed25519 signing
	// But we need to use a streaming body, so we'll construct the request manually
	url := fmt.Sprintf("%s/v1/workspaces/%s/events/stream", w.client.config.BaseURL, w.workspaceID)

	resp, err := w.client.doStreamingRequest(w.ctx, "POST", url, w.pipeReader)
	if err != nil {
		w.setError(fmt.Errorf("stream request failed: %w", err))
		return
	}
	defer resp.Body.Close()

	// Parse response
	var streamResp StreamEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
		w.setError(fmt.Errorf("failed to decode stream response: %w", err))
		return
	}

	w.mu.Lock()
	w.response = &streamResp
	w.mu.Unlock()

	if !streamResp.Success {
		w.setError(fmt.Errorf("stream request failed: %s", streamResp.Error))
	}
}

// WriteEvent writes an event to the stream
func (w *EventStreamWriter) WriteEvent(eventType string, data any) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return fmt.Errorf("stream is closed")
	}
	w.mu.Unlock()

	// Check context cancellation
	select {
	case <-w.ctx.Done():
		return w.ctx.Err()
	default:
	}

	// Create event message
	msg := StreamEventMessage{
		EventType: eventType,
		Data:      data,
	}

	// Marshal to JSON and add newline (NDJSON format)
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	msgBytes = append(msgBytes, '\n')

	// Write to pipe
	_, err = w.pipeWriter.Write(msgBytes)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// Close closes the event stream and waits for the response
func (w *EventStreamWriter) Close() (*StreamEventsResponse, error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return w.response, w.err
	}
	w.closed = true
	w.mu.Unlock()

	// Close the pipe writer to signal end of stream
	w.pipeWriter.Close()

	// Wait for the HTTP request to complete
	w.wg.Wait()

	return w.response, w.err
}

// Cancel cancels the stream without waiting for completion
func (w *EventStreamWriter) Cancel() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.mu.Unlock()

	// Cancel context and close pipe
	w.cancelCtx()
	w.pipeWriter.CloseWithError(context.Canceled)
}

func (w *EventStreamWriter) setError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err == nil {
		w.err = err
	}
}
