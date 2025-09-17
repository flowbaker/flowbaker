package flowbaker

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ClientInterface defines the main interface for interacting with the Flowbaker API
type ClientInterface interface {
	// Workflow execution operations
	CompleteWorkflowExecution(ctx context.Context, req *CompleteExecutionRequest) error

	// Event operations
	PublishExecutionEvent(ctx context.Context, workspaceID string, req *PublishEventRequest) error

	// Task publishing operations (for executor clients)
	EnqueueTask(ctx context.Context, workspaceID string, req *EnqueueTaskRequest) (*EnqueueTaskResponse, error)
	EnqueueTaskAndWait(ctx context.Context, workspaceID string, req *EnqueueTaskAndWaitRequest) (*EnqueueTaskAndWaitResponse, error)

	// Executor registration operations
	CreateExecutorRegistration(ctx context.Context, req *CreateExecutorRegistrationRequest) (*CreateExecutorRegistrationResponse, error)
	VerifyExecutorRegistration(ctx context.Context, workspaceID, code string) (*Executor, error)
	GetWorkspaceExecutors(ctx context.Context, workspaceID string) ([]Executor, error)

	// Workspace operations (for executor clients)
	GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error)
	GetWorkspaces(ctx context.Context) ([]Workspace, error)

	// Credential operations (for executor clients)
	GetCredential(ctx context.Context, workspaceID, credentialID string) (*EncryptedCredential, error)
	GetFullCredential(ctx context.Context, workspaceID, credentialID string) (*EncryptedFullCredential, error)
	BindParametersToStruct(ctx context.Context, workspaceID string, item any, nodeSettings map[string]any) ([]byte, error)
	GetExecutionFile(ctx context.Context, req GetExecutionFileRequest) (ExecutionWorkspaceFile, error)

	// File upload operations (for executor clients)
	UploadFile(ctx context.Context, req *UploadFileRequest) (*UploadFileResponse, error)

	// File streaming operations (for executor clients)
	StreamFile(ctx context.Context, req *StreamFileRequest) (*StreamFileResponse, error)
	GetFileReader(ctx context.Context, req *GetFileReaderRequest) (*GetFileReaderResult, error)
	GetFileInfo(ctx context.Context, req *GetFileInfoRequest) (*GetFileInfoResponse, error)
	CreateFileWriter(ctx context.Context, req *CreateFileWriterRequest) (*CreateFileWriterResult, error)
	PersistFile(ctx context.Context, req *PersistFileRequest) (*PersistFileResponse, error)

	// File and folder management operations (for executor clients)
	ListWorkspaceFiles(ctx context.Context, req *ListWorkspaceFilesRequest) (*ListWorkspaceFilesResponse, error)
	DeleteFile(ctx context.Context, req *DeleteFileRequest) (*DeleteFileResponse, error)
	ListFolders(ctx context.Context, req *ListFoldersRequest) (*ListFoldersResponse, error)

	// OAuth account operations (for executor clients)
	GetOAuthAccount(ctx context.Context, workspaceID, oauthAccountID string) (*GetOAuthAccountResponse, error)
	UpdateOAuthAccountMetadata(ctx context.Context, workspaceID, oauthAccountID string, req *UpdateOAuthAccountMetadataRequest) (*UpdateOAuthAccountMetadataResponse, error)

	// Integration operations (for executor clients)
	GetIntegrations(ctx context.Context) ([]byte, error)
	GetIntegration(ctx context.Context, integrationType string) ([]byte, error)

	// Knowledge operations (for executor clients)
	GetWorkspaceKnowledges(ctx context.Context, workspaceID string) ([]byte, error)
	GetKnowledge(ctx context.Context, workspaceID, knowledgeID string) ([]byte, error)
	GetKnowledgeFiles(ctx context.Context, workspaceID, knowledgeID string) ([]byte, error)
	GetKnowledgeFile(ctx context.Context, workspaceID, knowledgeID, fileID string) ([]byte, error)
	SearchKnowledge(ctx context.Context, workspaceID, knowledgeID string, req *SearchKnowledgeRequest) ([]byte, error)

	// Agent memory operations (for executor clients)
	SaveAgentConversation(ctx context.Context, workspaceID string, conversation *AgentConversation) (*SaveAgentConversationResponse, error)
	GetAgentConversations(ctx context.Context, req *GetAgentConversationsRequest) (*GetAgentConversationsResponse, error)
	DeleteOldAgentConversations(ctx context.Context, req *DeleteOldAgentConversationsRequest) (*DeleteOldAgentConversationsResponse, error)

	// Schedule operations (for executor clients)
	GetSchedule(ctx context.Context, workspaceID, scheduleID, workflowID string) ([]byte, error)
	CreateSchedule(ctx context.Context, workspaceID string, req *CreateScheduleRequest) ([]byte, error)

	// Route operations (for executor clients)
	GetRoutes(ctx context.Context, req GetRoutesRequest) (GetRoutesResponse, error)
}

// Client provides a high-level interface for interacting with the Flowbaker API
type Client struct {
	config     *ClientConfig
	httpClient *http.Client
	privateKey ed25519.PrivateKey // Parsed Ed25519 private key for signing
}

// NewClient creates a new Flowbaker client with the given options
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

	client := &Client{
		config:     config,
		httpClient: httpClient,
	}

	// Parse Ed25519 private key if provided
	if config.Ed25519PrivateKey != "" {
		privateKeyBytes, err := base64.StdEncoding.DecodeString(config.Ed25519PrivateKey)
		if err == nil && len(privateKeyBytes) == ed25519.PrivateKeySize {
			client.privateKey = ed25519.PrivateKey(privateKeyBytes)
		}
	}

	return client
}

// NewClientWithDefaults creates a new client with default configuration
func NewClientWithDefaults(baseURL, executorID, privateKey string) *Client {
	return NewClient(
		WithBaseURL(baseURL),
		WithExecutorID(executorID),
		WithEd25519PrivateKey(privateKey),
	)
}

// CompleteWorkflowExecution completes a workflow execution
func (c *Client) CompleteWorkflowExecution(ctx context.Context, req *CompleteExecutionRequest) error {
	path := fmt.Sprintf("/v1/workspaces/%s/executions/%s/complete", req.WorkspaceID, req.ExecutionID)

	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return fmt.Errorf("failed to complete workflow execution: %w", err)
	}

	var result struct {
		Success     bool   `json:"success"`
		ExecutionID string `json:"execution_id"`
	}
	if err := c.handleResponse(resp, &result); err != nil {
		return fmt.Errorf("failed to process complete execution response: %w", err)
	}

	return nil
}

// PublishExecutionEvent publishes an execution event
func (c *Client) PublishExecutionEvent(ctx context.Context, workspaceID string, req *PublishEventRequest) error {
	path := fmt.Sprintf("/v1/workspaces/%s/events", workspaceID)
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return fmt.Errorf("failed to publish execution event: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := c.handleResponse(resp, &result); err != nil {
		return fmt.Errorf("failed to process publish event response: %w", err)
	}

	return nil
}

// EnqueueTask publishes a task to the task queue (fire-and-forget)
func (c *Client) EnqueueTask(ctx context.Context, workspaceID string, req *EnqueueTaskRequest) (*EnqueueTaskResponse, error) {
	path := fmt.Sprintf("/v1/workspaces/%s/tasks/enqueue", workspaceID)
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	var result EnqueueTaskResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process enqueue task response: %w", err)
	}

	return &result, nil
}

// EnqueueTaskAndWait publishes a task to the task queue and waits for the result
func (c *Client) EnqueueTaskAndWait(ctx context.Context, workspaceID string, req *EnqueueTaskAndWaitRequest) (*EnqueueTaskAndWaitResponse, error) {
	path := fmt.Sprintf("/v1/workspaces/%s/tasks/enqueue-and-wait", workspaceID)
	resp, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue task and wait: %w", err)
	}

	var result EnqueueTaskAndWaitResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process enqueue task and wait response: %w", err)
	}

	return &result, nil
}

// CreateExecutorRegistration creates a new executor registration
func (c *Client) CreateExecutorRegistration(ctx context.Context, req *CreateExecutorRegistrationRequest) (*CreateExecutorRegistrationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	resp, err := c.doRequest(ctx, "POST", "/executors", req)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor registration: %w", err)
	}

	var result CreateExecutorRegistrationResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process create executor registration response: %w", err)
	}

	return &result, nil
}

// VerifyExecutorRegistration verifies an executor registration for a workspace
func (c *Client) VerifyExecutorRegistration(ctx context.Context, workspaceID, code string) (*Executor, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if code == "" {
		return nil, fmt.Errorf("verification code is required")
	}

	path := fmt.Sprintf("/workspaces/%s/executors/verify?code=%s", workspaceID, code)
	resp, err := c.doRequest(ctx, "POST", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify executor registration: %w", err)
	}

	var result struct {
		Executor *Executor `json:"executor"`
	}
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process verify executor registration response: %w", err)
	}

	return result.Executor, nil
}

// GetWorkspaceExecutors retrieves all executors for a workspace
func (c *Client) GetWorkspaceExecutors(ctx context.Context, workspaceID string) ([]Executor, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	path := fmt.Sprintf("/workspaces/%s/executors", workspaceID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace executors: %w", err)
	}

	var result []Executor
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process get workspace executors response: %w", err)
	}

	return result, nil
}

// GetWorkspace retrieves workspace information by ID
func (c *Client) GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s", workspaceID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}

	var result Workspace
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process get workspace response: %w", err)
	}

	return &result, nil
}

// GetWorkspaces retrieves all workspaces for authenticated executor
func (c *Client) GetWorkspaces(ctx context.Context) ([]Workspace, error) {
	path := "/v1/workspaces"
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get executor workspaces: %w", err)
	}

	var result []Workspace
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process get executor workspaces response: %w", err)
	}

	return result, nil
}

// GetCredential retrieves an encrypted credential for the executor
func (c *Client) GetCredential(ctx context.Context, workspaceID, credentialID string) (*EncryptedCredential, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for credential requests")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/credentials/%s", workspaceID, credentialID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	var credential EncryptedCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, fmt.Errorf("failed to process credential response: %w", err)
	}

	return &credential, nil
}

// GetFullCredential retrieves an encrypted full credential for the executor
func (c *Client) GetFullCredential(ctx context.Context, workspaceID, credentialID string) (*EncryptedFullCredential, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for full credential requests")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/credentials/%s?content=full", workspaceID, credentialID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get full credential: %w", err)
	}

	var credential EncryptedFullCredential
	if err := c.handleResponse(resp, &credential); err != nil {
		return nil, fmt.Errorf("failed to process full credential response: %w", err)
	}

	return &credential, nil
}

func (c *Client) BindParametersToStruct(ctx context.Context, workspaceID string, item any, nodeSettings map[string]any) ([]byte, error) {
	path := fmt.Sprintf("/v1/workspaces/%s/expressions/resolve", workspaceID)

	resp, err := c.doRequestWithExecutorID(ctx, "POST", path, map[string]any{
		"item":          item,
		"node_settings": nodeSettings,
		"workspace_id":  workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("parameter binding failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

func (c *Client) GetExecutionFile(ctx context.Context, req GetExecutionFileRequest) (ExecutionWorkspaceFile, error) {
	path := fmt.Sprintf("/v1/workspaces/%s/files/%s", req.WorkspaceID, req.UploadID)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return ExecutionWorkspaceFile{}, fmt.Errorf("failed to get execution file: %w", err)
	}

	var file ExecutionWorkspaceFile

	if err := c.handleResponse(resp, &file); err != nil {
		return ExecutionWorkspaceFile{}, fmt.Errorf("failed to process execution file response: %w", err)
	}

	return file, nil
}

// UploadFile uploads a file using chunked upload through the executor hooks API
func (c *Client) UploadFile(ctx context.Context, req *UploadFileRequest) (*UploadFileResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for file uploads")
	}

	// Set default chunk size if not provided
	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 2 * 1024 * 1024 // 2MB default
	}

	// Create a TeeReader to calculate checksum while reading
	hasher := sha256.New()
	var buffer bytes.Buffer

	// First pass: read all data to calculate checksum and store in buffer
	teeReader := io.TeeReader(req.Reader, io.MultiWriter(hasher, &buffer))
	written, err := io.Copy(io.Discard, teeReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read input data: %w", err)
	}

	// Verify size matches if provided
	if req.TotalSize > 0 && written != req.TotalSize {
		return nil, fmt.Errorf("actual size (%d) doesn't match expected size (%d)", written, req.TotalSize)
	}

	// Update total size if not provided
	totalSize := req.TotalSize
	if totalSize <= 0 {
		totalSize = written
	}

	// Calculate checksum
	checksum := hex.EncodeToString(hasher.Sum(nil))

	// Calculate total chunks
	totalChunks := (totalSize + chunkSize - 1) / chunkSize

	// Step 1: Initiate upload
	initiateReq := InitiateUploadRequest{
		FileName:    req.FileName,
		ContentType: req.ContentType,
		TotalSize:   totalSize,
		ChunkSize:   chunkSize,
		Checksum:    checksum,
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

	// Step 2: Upload chunks
	uploadedBytes := int64(0)
	reader := bytes.NewReader(buffer.Bytes())

	for chunkIndex := int64(0); chunkIndex < totalChunks; chunkIndex++ {
		select {
		case <-ctx.Done():
			// Try to abort upload on cancellation
			c.abortUpload(context.Background(), req.WorkspaceID, initiateResp.UploadID)
			return nil, ctx.Err()
		default:
		}

		// Calculate chunk boundaries
		start := chunkIndex * chunkSize
		end := start + chunkSize
		if end > totalSize {
			end = totalSize
		}
		currentChunkSize := end - start

		// Read chunk data
		chunkData := make([]byte, currentChunkSize)
		n, err := reader.ReadAt(chunkData, start)
		if err != nil && err != io.EOF {
			c.abortUpload(context.Background(), req.WorkspaceID, initiateResp.UploadID)
			return nil, fmt.Errorf("failed to read chunk %d: %w", chunkIndex, err)
		}
		if int64(n) != currentChunkSize {
			c.abortUpload(context.Background(), req.WorkspaceID, initiateResp.UploadID)
			return nil, fmt.Errorf("read %d bytes, expected %d for chunk %d", n, currentChunkSize, chunkIndex)
		}

		// Upload chunk with retry
		if err := c.uploadChunkWithRetry(ctx, req.WorkspaceID, initiateResp.UploadID, chunkIndex, chunkData); err != nil {
			c.abortUpload(context.Background(), req.WorkspaceID, initiateResp.UploadID)
			return nil, fmt.Errorf("failed to upload chunk %d: %w", chunkIndex, err)
		}

		uploadedBytes += currentChunkSize

		// Call progress callback if provided
		if req.ProgressFn != nil {
			percent := float64(uploadedBytes) / float64(totalSize) * 100
			req.ProgressFn(uploadedBytes, totalSize, percent)
		}
	}

	// Step 3: Complete upload
	completeResp, err := c.doRequestWithExecutorID(ctx, "POST", fmt.Sprintf("/v1/workspaces/%s/files/%s/complete", req.WorkspaceID, initiateResp.UploadID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to complete upload: %w", err)
	}

	var completeResult CompleteUploadResponse
	if err := c.handleResponse(completeResp, &completeResult); err != nil {
		return nil, fmt.Errorf("failed to process complete upload response: %w", err)
	}

	// Return success response
	return &UploadFileResponse{
		UploadID: initiateResp.UploadID,
		FileID:   completeResult.FileID,
		FileSize: totalSize,
		FileName: req.FileName,
		Status:   "completed",
	}, nil
}

// uploadChunkWithRetry uploads a single chunk with retry logic
func (c *Client) uploadChunkWithRetry(ctx context.Context, workspaceID, uploadID string, chunkIndex int64, chunkData []byte) error {
	maxRetries := c.config.RetryAttempts
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}

		path := fmt.Sprintf("/v1/workspaces/%s/files/%s/chunks/%d", workspaceID, uploadID, chunkIndex)

		// Create request with chunk data
		req, err := http.NewRequestWithContext(ctx, "PUT", c.config.BaseURL+path, bytes.NewReader(chunkData))
		if err != nil {
			lastErr = fmt.Errorf("failed to create chunk upload request: %w", err)
			continue
		}

		// Set headers
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Length", strconv.Itoa(len(chunkData)))

		// Apply user agent
		if c.config.UserAgent != "" {
			req.Header.Set("User-Agent", c.config.UserAgent)
		}

		// Sign the request
		c.signRequest(req, chunkData)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check for successful upload (2xx status codes)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}

		// Handle error response
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		lastErr = &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("chunk upload failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}

		// Don't retry on client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			break
		}
	}

	return fmt.Errorf("chunk upload failed after %d retries: %w", maxRetries, lastErr)
}

// abortUpload attempts to abort an upload (best effort, doesn't return error)
func (c *Client) abortUpload(ctx context.Context, workspaceID, uploadID string) {
	path := fmt.Sprintf("/v1/workspaces/%s/files/%s/abort", workspaceID, uploadID)
	resp, err := c.doRequestWithExecutorID(ctx, "DELETE", path, nil)
	if err == nil && resp != nil {
		resp.Body.Close()
	}
}

// StreamFile streams a file with optional range support for chunk-by-chunk downloading
func (c *Client) StreamFile(ctx context.Context, req *StreamFileRequest) (*StreamFileResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for file streaming")
	}

	if req.UploadID == "" {
		return nil, fmt.Errorf("upload ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/files/%s/content", req.WorkspaceID, req.UploadID)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.config.BaseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream request: %w", err)
	}

	// Apply user agent
	if c.config.UserAgent != "" {
		httpReq.Header.Set("User-Agent", c.config.UserAgent)
	}

	// Set range header if provided
	if req.RangeHeader != "" {
		httpReq.Header.Set("Range", req.RangeHeader)
	}

	// Sign the request (no body for GET requests)
	c.signRequest(httpReq, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute stream request: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("stream request failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	// Parse content length
	contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	// Extract filename from Content-Disposition header if present
	fileName := ""
	if disposition := resp.Header.Get("Content-Disposition"); disposition != "" {
		// Simple extraction of filename from Content-Disposition
		// Format: attachment; filename="filename.ext"
		if start := strings.Index(disposition, `filename="`); start != -1 {
			start += len(`filename="`)
			if end := strings.Index(disposition[start:], `"`); end != -1 {
				fileName = disposition[start : start+end]
			}
		}
	}

	return &StreamFileResponse{
		Content:       resp.Body,
		ContentLength: contentLength,
		ContentRange:  resp.Header.Get("Content-Range"),
		StatusCode:    resp.StatusCode,
		ContentType:   resp.Header.Get("Content-Type"),
		FileName:      fileName,
	}, nil
}

// GetFileReader returns a GetFileReaderResult containing an io.ReadCloser that automatically
// fetches file chunks on-demand as the file is read, along with file metadata
func (c *Client) GetFileReader(ctx context.Context, req *GetFileReaderRequest) (*GetFileReaderResult, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for file streaming")
	}

	if req.UploadID == "" {
		return nil, fmt.Errorf("upload ID is required")
	}

	// Get file info first
	fileInfo, err := c.GetFileInfo(ctx, &GetFileInfoRequest{
		WorkspaceID: req.WorkspaceID,
		UploadID:    req.UploadID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Check if file is ready for streaming
	if fileInfo.Status != "completed" {
		return nil, fmt.Errorf("file is not ready for streaming (status: %s)", fileInfo.Status)
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

	// Create the chunked reader with pre-fetched metadata
	reader := &ChunkedFileReader{
		client:      c,
		workspaceID: req.WorkspaceID,
		uploadID:    req.UploadID,
		ctx:         ctx,
		chunkSize:   chunkSize,
		totalSize:   fileInfo.SizeInBytes,
		contentType: fileInfo.ContentType,
		fileName:    fileInfo.FileName,
		initialized: true, // Already initialized with metadata
	}

	return &GetFileReaderResult{
		Reader:      reader,
		FileName:    fileInfo.FileName,
		ContentType: fileInfo.ContentType,
		FileSize:    fileInfo.SizeInBytes,
		Status:      fileInfo.Status,
	}, nil
}

// GetFileInfo gets metadata about a file without downloading it
func (c *Client) GetFileInfo(ctx context.Context, req *GetFileInfoRequest) (*GetFileInfoResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for file info")
	}

	if req.UploadID == "" {
		return nil, fmt.Errorf("upload ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/files/%s", req.WorkspaceID, req.UploadID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	var fileInfo GetFileInfoResponse
	if err := c.handleResponse(resp, &fileInfo); err != nil {
		return nil, fmt.Errorf("failed to process file info response: %w", err)
	}

	return &fileInfo, nil
}

// PersistFile marks a file as persistent by removing its expiration
func (c *Client) PersistFile(ctx context.Context, req *PersistFileRequest) (*PersistFileResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for file persistence")
	}

	if req.UploadID == "" {
		return nil, fmt.Errorf("upload ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/files/%s", req.WorkspaceID, req.UploadID)

	resp, err := c.doRequestWithExecutorID(ctx, "PUT", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to persist file: %w", err)
	}

	var result PersistFileResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process persist file response: %w", err)
	}

	return &result, nil
}

// doRequest performs an HTTP request with retry logic and proper error handling
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return c.doRequestWithHeaders(ctx, method, path, body, nil)
}

// doRequestWithExecutorID performs an HTTP request with executor ID header
func (c *Client) doRequestWithExecutorID(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	headers := map[string]string{
		"X-Executor-ID": c.config.ExecutorID,
	}
	return c.doRequestWithHeaders(ctx, method, path, body, headers)
}

// signRequest adds signature headers to the request
func (c *Client) signRequest(req *http.Request, bodyBytes []byte) {
	if c.privateKey == nil || c.config.ExecutorID == "" {
		return
	}

	// Calculate timestamp
	timestamp := time.Now().Unix()

	// Calculate body hash
	bodyHash := ""
	if len(bodyBytes) > 0 {
		h := sha256.Sum256(bodyBytes)
		bodyHash = hex.EncodeToString(h[:])
	} else {
		// Hash of empty string
		h := sha256.Sum256([]byte{})
		bodyHash = hex.EncodeToString(h[:])
	}

	// Build signature payload: method|path|timestamp|executor-id|body-sha256
	signaturePayload := fmt.Sprintf("%s|%s|%d|%s|%s",
		req.Method,
		req.URL.Path,
		timestamp,
		c.config.ExecutorID,
		bodyHash,
	)

	// Sign the payload
	signature := ed25519.Sign(c.privateKey, []byte(signaturePayload))

	// Add signature headers
	req.Header.Set("X-Executor-ID", c.config.ExecutorID)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(signature))
}

// doRequestWithHeaders performs an HTTP request with custom headers
func (c *Client) doRequestWithHeaders(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyBytes []byte
	var requestBody io.Reader

	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		requestBody = bytes.NewBuffer(bodyBytes)
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
			// Reset body reader for retry
			if bodyBytes != nil {
				requestBody = bytes.NewBuffer(bodyBytes)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Apply default headers
		for key, value := range c.config.DefaultHeaders {
			req.Header.Set(key, value)
		}

		// Apply custom headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Apply user agent
		if c.config.UserAgent != "" {
			req.Header.Set("User-Agent", c.config.UserAgent)
		}

		// Sign the request
		c.signRequest(req, bodyBytes)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check for server errors that might be retryable
		if resp.StatusCode >= 500 {
			log.Error().
				Int("status_code", resp.StatusCode).
				Str("body", string(bodyBytes)).
				Str("request_id", resp.Header.Get("X-Request-ID")).
				Msg("server error")

			resp.Body.Close()
			lastErr = &Error{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("server error: %d", resp.StatusCode),
			}
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.config.RetryAttempts, lastErr)
}

// handleResponse processes the HTTP response and unmarshals JSON if successful
func (c *Client) handleResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errorResponse struct {
			Error   string `json:"error"`
			Message string `json:"message"`
			Code    string `json:"code"`
		}

		// Try to parse error response
		if json.Unmarshal(body, &errorResponse) == nil && errorResponse.Error != "" {
			return &Error{
				StatusCode: resp.StatusCode,
				Message:    errorResponse.Error,
				Body:       string(body),
				RequestID:  resp.Header.Get("X-Request-ID"),
			}
		}

		if json.Unmarshal(body, &errorResponse) == nil && errorResponse.Message != "" {
			return &Error{
				StatusCode: resp.StatusCode,
				Message:    errorResponse.Message,
				Body:       string(body),
				RequestID:  resp.Header.Get("X-Request-ID"),
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

// ListWorkspaceFiles lists files in a workspace, optionally filtered by folder
func (c *Client) ListWorkspaceFiles(ctx context.Context, req *ListWorkspaceFilesRequest) (*ListWorkspaceFilesResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for listing workspace files")
	}

	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/files", req.WorkspaceID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace files: %w", err)
	}

	var result ListWorkspaceFilesResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process list workspace files response: %w", err)
	}

	return &result, nil
}

// DeleteFile deletes a file from the workspace
func (c *Client) DeleteFile(ctx context.Context, req *DeleteFileRequest) (*DeleteFileResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for deleting files")
	}

	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if req.UploadID == "" {
		return nil, fmt.Errorf("upload ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/files/%s", req.WorkspaceID, req.UploadID)

	resp, err := c.doRequestWithExecutorID(ctx, "DELETE", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	var result DeleteFileResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process delete file response: %w", err)
	}

	return &result, nil
}

// ListFolders lists folders in a workspace, optionally filtered by parent folder
func (c *Client) ListFolders(ctx context.Context, req *ListFoldersRequest) (*ListFoldersResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for listing folders")
	}

	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/folders", req.WorkspaceID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	var result ListFoldersResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process list folders response: %w", err)
	}

	return &result, nil
}

// GetOAuthAccount retrieves an OAuth account for the executor
func (c *Client) GetOAuthAccount(ctx context.Context, workspaceID, oauthAccountID string) (*GetOAuthAccountResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required")
	}

	if oauthAccountID == "" {
		return nil, fmt.Errorf("OAuth account ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/oauths/%s", workspaceID, oauthAccountID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth account: %w", err)
	}

	var result GetOAuthAccountResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process OAuth account response: %w", err)
	}

	return &result, nil
}

// UpdateOAuthAccountMetadata updates the metadata of an OAuth account for the executor
func (c *Client) UpdateOAuthAccountMetadata(ctx context.Context, workspaceID, oauthAccountID string, req *UpdateOAuthAccountMetadataRequest) (*UpdateOAuthAccountMetadataResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required")
	}

	if oauthAccountID == "" {
		return nil, fmt.Errorf("OAuth account ID is required")
	}

	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/oauths/%s/metadata", workspaceID, oauthAccountID)

	resp, err := c.doRequestWithExecutorID(ctx, "PUT", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update OAuth account metadata: %w", err)
	}

	var result UpdateOAuthAccountMetadataResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process update OAuth account metadata response: %w", err)
	}

	return &result, nil
}

// GetIntegrations retrieves all integrations for the executor
func (c *Client) GetIntegrations(ctx context.Context) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for integration requests")
	}

	path := "/v1/integrations"

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get integrations: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("get integrations failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// GetIntegration retrieves a specific integration by type for the executor
func (c *Client) GetIntegration(ctx context.Context, integrationType string) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for integration requests")
	}

	if integrationType == "" {
		return nil, fmt.Errorf("integration type is required")
	}

	path := fmt.Sprintf("/v1/integrations/%s", integrationType)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("get integration failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// GetWorkspaceKnowledges retrieves all knowledge bases for the workspace
func (c *Client) GetWorkspaceKnowledges(ctx context.Context, workspaceID string) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for knowledge requests")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/knowledge", workspaceID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace knowledges: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("get workspace knowledges failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// GetKnowledge retrieves a specific knowledge base by ID
func (c *Client) GetKnowledge(ctx context.Context, workspaceID, knowledgeID string) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for knowledge requests")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if knowledgeID == "" {
		return nil, fmt.Errorf("knowledge ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/knowledge/%s", workspaceID, knowledgeID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("get knowledge failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// GetKnowledgeFiles retrieves all files in a knowledge base
func (c *Client) GetKnowledgeFiles(ctx context.Context, workspaceID, knowledgeID string) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for knowledge requests")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if knowledgeID == "" {
		return nil, fmt.Errorf("knowledge ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/knowledge/%s/files", workspaceID, knowledgeID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge files: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("get knowledge files failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// GetKnowledgeFile retrieves a specific knowledge file by ID
func (c *Client) GetKnowledgeFile(ctx context.Context, workspaceID, knowledgeID, fileID string) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for knowledge requests")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if knowledgeID == "" {
		return nil, fmt.Errorf("knowledge ID is required")
	}

	if fileID == "" {
		return nil, fmt.Errorf("file ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/knowledge/%s/files/%s", workspaceID, knowledgeID, fileID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get knowledge file: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("get knowledge file failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// SearchKnowledge performs semantic search in a knowledge base
func (c *Client) SearchKnowledge(ctx context.Context, workspaceID, knowledgeID string, req *SearchKnowledgeRequest) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for knowledge requests")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if knowledgeID == "" {
		return nil, fmt.Errorf("knowledge ID is required")
	}

	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/knowledge/%s/search", workspaceID, knowledgeID)

	resp, err := c.doRequestWithExecutorID(ctx, "POST", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("search knowledge failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// GetSchedule retrieves a specific schedule for the executor
func (c *Client) GetSchedule(ctx context.Context, workspaceID, scheduleID, workflowID string) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for schedule requests")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if scheduleID == "" {
		return nil, fmt.Errorf("schedule ID is required")
	}

	if workflowID == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/schedules/%s", workspaceID, scheduleID)

	// Add workflow ID as query parameter
	path = fmt.Sprintf("%s?workflow_id=%s", path, workflowID)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("get schedule failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// CreateSchedule creates a new schedule for the executor
func (c *Client) CreateSchedule(ctx context.Context, workspaceID string, req *CreateScheduleRequest) ([]byte, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for schedule requests")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if req == nil {
		return nil, fmt.Errorf("create schedule request is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/schedules", workspaceID)

	resp, err := c.doRequestWithExecutorID(ctx, "POST", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}
	defer resp.Body.Close()

	// Read the raw JSON response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, &Error{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("create schedule failed with status %d", resp.StatusCode),
			Body:       string(body),
			RequestID:  resp.Header.Get("X-Request-ID"),
		}
	}

	return body, nil
}

// SaveAgentConversation saves an agent conversation for the executor
func (c *Client) SaveAgentConversation(ctx context.Context, workspaceID string, conversation *AgentConversation) (*SaveAgentConversationResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for agent memory operations")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if conversation == nil {
		return nil, fmt.Errorf("conversation is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/agent-memory/conversations", workspaceID)

	resp, err := c.doRequestWithExecutorID(ctx, "POST", path, conversation)
	if err != nil {
		return nil, fmt.Errorf("failed to save agent conversation: %w", err)
	}

	var result SaveAgentConversationResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process save agent conversation response: %w", err)
	}

	return &result, nil
}

// GetAgentConversations retrieves agent conversations for the executor
func (c *Client) GetAgentConversations(ctx context.Context, req *GetAgentConversationsRequest) (*GetAgentConversationsResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for agent memory operations")
	}

	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/agent-memory/conversations?session_id=%s&limit=%d&offset=%d",
		req.WorkspaceID, req.SessionID, req.Limit, req.Offset)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent conversations: %w", err)
	}

	var result GetAgentConversationsResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process get agent conversations response: %w", err)
	}

	return &result, nil
}

// DeleteOldAgentConversations deletes old agent conversations for the executor
func (c *Client) DeleteOldAgentConversations(ctx context.Context, req *DeleteOldAgentConversationsRequest) (*DeleteOldAgentConversationsResponse, error) {
	if c.config.ExecutorID == "" {
		return nil, fmt.Errorf("executor ID is required for agent memory operations")
	}

	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	path := fmt.Sprintf("/v1/workspaces/%s/agent-memory/conversations/cleanup", req.WorkspaceID)

	resp, err := c.doRequestWithExecutorID(ctx, "DELETE", path, req)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old agent conversations: %w", err)
	}

	var result DeleteOldAgentConversationsResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to process delete old agent conversations response: %w", err)
	}

	return &result, nil
}

func (c *Client) GetRoutes(ctx context.Context, req GetRoutesRequest) (GetRoutesResponse, error) {
	if c.config.ExecutorID == "" {
		return GetRoutesResponse{}, fmt.Errorf("executor ID is required for route requests")
	}

	if req.WorkspaceID == "" {
		return GetRoutesResponse{}, fmt.Errorf("workspace ID is required")
	}

	if req.RouteID == "" {
		return GetRoutesResponse{}, fmt.Errorf("route ID is required")
	}

	if req.TriggerEventType == "" {
		return GetRoutesResponse{}, fmt.Errorf("trigger event type is required")
	}

	if req.WorkflowType == "" {
		req.WorkflowType = "default"
	}

	queryParams := url.Values{}

	queryParams.Add("trigger_event_type", req.TriggerEventType)
	queryParams.Add("workflow_type", req.WorkflowType)
	queryParams.Add("is_webhook", strconv.FormatBool(req.IsWebhook))

	queryString := queryParams.Encode()

	path := fmt.Sprintf("/v1/workspaces/%s/routes/%s?%s", req.WorkspaceID, req.RouteID, queryString)

	resp, err := c.doRequestWithExecutorID(ctx, "GET", path, nil)
	if err != nil {
		return GetRoutesResponse{}, fmt.Errorf("failed to get routes: %w", err)
	}

	var result GetRoutesResponse

	if err := c.handleResponse(resp, &result); err != nil {
		return GetRoutesResponse{}, fmt.Errorf("failed to process get routes response: %w", err)
	}

	return result, nil
}
