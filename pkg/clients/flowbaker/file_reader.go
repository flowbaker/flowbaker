package flowbaker

import (
	"context"
	"fmt"
	"io"
)

// ChunkedFileReader implements io.ReadCloser and automatically fetches file chunks on-demand
type ChunkedFileReader struct {
	client      *Client
	workspaceID string
	uploadID    string
	ctx         context.Context

	// File metadata (fetched once)
	totalSize   int64
	contentType string
	fileName    string

	// Reading state
	currentPos  int64  // Current position in file
	chunkSize   int64  // Size of each chunk request
	buffer      []byte // Current chunk data
	bufferPos   int    // Position within current buffer
	eof         bool   // True when we've reached end of file
	initialized bool   // True when metadata has been fetched

	// Error handling
	lastError error
}

// Read implements io.Reader interface for ChunkedFileReader
func (r *ChunkedFileReader) Read(p []byte) (n int, err error) {
	// Check for previous errors
	if r.lastError != nil {
		return 0, r.lastError
	}

	// Check if we've reached EOF
	if r.eof {
		return 0, io.EOF
	}

	// Check context cancellation
	select {
	case <-r.ctx.Done():
		r.lastError = r.ctx.Err()
		return 0, r.lastError
	default:
	}

	// If buffer is empty or exhausted, fetch next chunk
	if r.buffer == nil || r.bufferPos >= len(r.buffer) {
		if err := r.fetchNextChunk(); err != nil {
			if err == io.EOF {
				r.eof = true
			} else {
				r.lastError = err
			}
			return 0, err
		}
	}

	// Copy from buffer to p
	available := len(r.buffer) - r.bufferPos
	copySize := len(p)
	if copySize > available {
		copySize = available
	}

	copy(p, r.buffer[r.bufferPos:r.bufferPos+copySize])
	r.bufferPos += copySize
	r.currentPos += int64(copySize)

	// Check if we've read everything
	if r.currentPos >= r.totalSize {
		r.eof = true
		if copySize == 0 {
			return 0, io.EOF
		}
	}

	return copySize, nil
}

// Close implements io.Closer interface for ChunkedFileReader
func (r *ChunkedFileReader) Close() error {
	// Clear buffer to free memory
	r.buffer = nil
	r.eof = true
	return nil
}

// GetFileInfo returns metadata about the file being streamed (deprecated - use GetFileReaderResult fields instead)
func (r *ChunkedFileReader) GetFileInfo() (fileName string, contentType string, totalSize int64, err error) {
	return r.fileName, r.contentType, r.totalSize, nil
}

// initialize fetches file metadata on first access using the dedicated file info endpoint
func (r *ChunkedFileReader) initialize() error {
	// Use the dedicated file info endpoint instead of making a range request
	fileInfo, err := r.client.GetFileInfo(r.ctx, &GetFileInfoRequest{
		WorkspaceID: r.workspaceID,
		UploadID:    r.uploadID,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize file reader: %w", err)
	}

	// Check if file is ready for streaming
	if fileInfo.Status != "completed" {
		return fmt.Errorf("file is not ready for streaming (status: %s)", fileInfo.Status)
	}

	// Set metadata from response
	r.totalSize = fileInfo.SizeInBytes
	r.contentType = fileInfo.ContentType
	r.fileName = fileInfo.FileName
	r.initialized = true

	return nil
}

// fetchNextChunk fetches the next chunk of data from the server
func (r *ChunkedFileReader) fetchNextChunk() error {
	// Check if we've already read everything
	if r.currentPos >= r.totalSize {
		return io.EOF
	}

	// Calculate range for next chunk
	start := r.currentPos
	end := start + r.chunkSize - 1
	if end >= r.totalSize {
		end = r.totalSize - 1
	}

	// Make range request
	resp, err := r.client.StreamFile(r.ctx, &StreamFileRequest{
		WorkspaceID: r.workspaceID,
		UploadID:    r.uploadID,
		RangeHeader: fmt.Sprintf("bytes=%d-%d", start, end),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch chunk: %w", err)
	}
	defer resp.Content.Close()

	// Read chunk into buffer
	chunkData, err := io.ReadAll(resp.Content)
	if err != nil {
		return fmt.Errorf("failed to read chunk data: %w", err)
	}

	// Update buffer state
	r.buffer = chunkData
	r.bufferPos = 0

	// If we got less data than expected, we might be at EOF
	if int64(len(chunkData)) < (end - start + 1) {
		r.totalSize = r.currentPos + int64(len(chunkData))
	}

	return nil
}
