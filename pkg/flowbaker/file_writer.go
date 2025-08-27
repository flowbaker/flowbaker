package flowbaker

import (
	"context"
	"fmt"
	"io"
)

// CreateFileWriterRequest represents the request to create a streaming file writer
type CreateFileWriterRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	FileName    string  `json:"file_name"`
	ContentType string  `json:"content_type"`
	ChunkSize   int64   `json:"chunk_size,omitempty"` // Optional chunk size, defaults to 2MB
	FolderID    *string `json:"folder_id,omitempty"`
	TotalSize   int64   `json:"total_size,omitempty"`
}

// CreateFileWriterResult represents the result of creating a file writer
type CreateFileWriterResult struct {
	Writer   io.WriteCloser `json:"-"`         // The streaming writer
	UploadID string         `json:"upload_id"` // Upload ID for tracking
	FileName string         `json:"file_name"` // Original filename
	Status   string         `json:"status"`    // Upload status
}

// ChunkedFileWriter implements io.WriteCloser for streaming file uploads
// It automatically uploads data in chunks as it's written
type ChunkedFileWriter struct {
	client      *Client
	workspaceID string
	uploadID    string
	fileName    string
	contentType string
	chunkSize   int64
	totalSize   int64
	ctx         context.Context

	// Internal state
	buffer       []byte
	currentChunk int64
	totalWritten int64
	closed       bool
}

// CreateFileWriter creates a streaming file writer that implements io.WriteCloser
// Data written to the writer is automatically uploaded in chunks as it's written
func (c *Client) CreateFileWriter(ctx context.Context, req *CreateFileWriterRequest) (*CreateFileWriterResult, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for file writing")
	}

	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if req.FileName == "" {
		return nil, fmt.Errorf("file name is required")
	}

	// Set default chunk size
	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 2 * 1024 * 1024 // 2MB default
	}

	// Cap chunk size for memory safety
	maxChunkSize := int64(10 * 1024 * 1024) // 10MB max
	if chunkSize > maxChunkSize {
		chunkSize = maxChunkSize
	}

	// Set default content type if not provided
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Initialize upload immediately
	initiateReq := InitiateUploadRequest{
		FileName:    req.FileName,
		ContentType: contentType,
		TotalSize:   req.TotalSize,
		ChunkSize:   chunkSize,
		Checksum:    "", // Will be calculated on server side
		FolderID:    req.FolderID,
	}

	resp, err := c.doRequestWithExecutorID(ctx, "POST", fmt.Sprintf("/v1/workspaces/%s/files", req.WorkspaceID), initiateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate upload: %w", err)
	}

	var initiateResp InitiateUploadResponse
	if err := c.handleResponse(resp, &initiateResp); err != nil {
		return nil, fmt.Errorf("failed to process initiate upload response: %w", err)
	}

	// Create the chunked writer with initialized upload ID
	writer := &ChunkedFileWriter{
		client:      c,
		workspaceID: req.WorkspaceID,
		uploadID:    initiateResp.UploadID,
		fileName:    req.FileName,
		contentType: contentType,
		chunkSize:   chunkSize,
		ctx:         ctx,
		buffer:      make([]byte, 0, chunkSize),
		totalSize:   req.TotalSize,
	}

	return &CreateFileWriterResult{
		Writer:   writer,
		UploadID: initiateResp.UploadID,
		FileName: req.FileName,
		Status:   "initialized",
	}, nil
}

// Write implements io.Writer for ChunkedFileWriter
// It buffers data and uploads complete chunks automatically
func (w *ChunkedFileWriter) Write(p []byte) (n int, err error) {
	// Check if writer is closed
	if w.closed {
		return 0, fmt.Errorf("writer is closed")
	}

	// Process all input data
	remaining := p
	totalWritten := 0

	for len(remaining) > 0 {
		// Check context cancellation
		select {
		case <-w.ctx.Done():
			return totalWritten, w.ctx.Err()
		default:
		}

		// Calculate how much we can buffer
		bufferSpace := w.chunkSize - int64(len(w.buffer))
		writeSize := int64(len(remaining))
		if writeSize > bufferSpace {
			writeSize = bufferSpace
		}

		// Add data to buffer
		w.buffer = append(w.buffer, remaining[:writeSize]...)
		remaining = remaining[writeSize:]
		totalWritten += int(writeSize)

		// Upload chunk if buffer is full
		if int64(len(w.buffer)) >= w.chunkSize {
			if err := w.uploadCurrentChunk(); err != nil {
				return totalWritten, fmt.Errorf("failed to upload chunk %d: %w", w.currentChunk, err)
			}
		}
	}

	w.totalWritten += int64(totalWritten)
	return totalWritten, nil
}

// Close implements io.Closer for ChunkedFileWriter
// It uploads any remaining buffered data and completes the upload
func (w *ChunkedFileWriter) Close() error {
	if w.closed {
		return nil // Already closed
	}
	w.closed = true

	// Upload any remaining buffered data
	if len(w.buffer) > 0 {
		if err := w.uploadCurrentChunk(); err != nil {
			// Try to abort upload on error
			w.client.abortUpload(context.Background(), w.workspaceID, w.uploadID)
			return fmt.Errorf("failed to upload final chunk: %w", err)
		}
	}

	// Complete the upload
	resp, err := w.client.doRequestWithExecutorID(w.ctx, "POST", fmt.Sprintf("/v1/workspaces/%s/files/%s/complete", w.workspaceID, w.uploadID), nil)
	if err != nil {
		return fmt.Errorf("failed to complete upload: %w", err)
	}

	var completeResult CompleteUploadResponse
	if err := w.client.handleResponse(resp, &completeResult); err != nil {
		return fmt.Errorf("failed to process complete upload response: %w", err)
	}

	return nil
}

// uploadCurrentChunk uploads the current buffer as a chunk
func (w *ChunkedFileWriter) uploadCurrentChunk() error {
	if len(w.buffer) == 0 {
		return nil
	}

	// Upload the chunk using existing retry logic
	if err := w.client.uploadChunkWithRetry(w.ctx, w.workspaceID, w.uploadID, w.currentChunk, w.buffer); err != nil {
		return err
	}

	// Reset buffer and increment chunk counter
	w.buffer = w.buffer[:0]
	w.currentChunk++

	return nil
}
