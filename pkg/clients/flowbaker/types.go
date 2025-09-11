// Package flowbaker provides a Go SDK for interacting with the Flowbaker API.
// This package is designed for community use and has no internal dependencies.
package flowbaker

import (
	"encoding/json"
	"io"
	"time"
)

// Item represents any data item
type Item any

// NodeItems represents items from a specific node
type NodeItems struct {
	FromNodeID string `json:"from_node_id"`
	Items      []Item `json:"items"`
}

// EventType represents the type of execution event
type EventType string

const (
	EventTypeNodeExecutionStarted       EventType = "node_execution_started"
	EventTypeNodeExecuted               EventType = "node_executed"
	EventTypeNodeFailed                 EventType = "node_failed"
	EventTypeWorkflowExecutionCompleted EventType = "workflow_execution_completed"
)

// NodeExecution represents the execution metrics for a single node
type NodeExecution struct {
	ID                     string           `json:"id"`
	NodeID                 string           `json:"node_id"`
	IntegrationType        string           `json:"integration_type"`
	IntegrationActionType  string           `json:"integration_action_type"`
	StartedAt              time.Time        `json:"started_at"`
	EndedAt                time.Time        `json:"ended_at"`
	ExecutionOrder         int64            `json:"execution_order"`
	InputItemsCount        map[string]int64 `json:"input_items_count"`
	InputItemsSizeInBytes  map[string]int64 `json:"input_items_size_in_bytes"`
	OutputItemsCount       map[int64]int64  `json:"output_items_count"`
	OutputItemsSizeInBytes map[int64]int64  `json:"output_items_size_in_bytes"`
}

// NodeExecutionEntry represents a history entry for node execution
type NodeExecutionEntry struct {
	NodeID          string               `json:"node_id"`
	ItemsByInputID  map[string]NodeItems `json:"items_by_input_id"`
	ItemsByOutputID map[string]NodeItems `json:"items_by_output_id"`
	EventType       EventType            `json:"event_type"`
	Error           string               `json:"error,omitempty"`
	Timestamp       int64                `json:"timestamp"`
	ExecutionOrder  int                  `json:"execution_order"`
}

// PublishEventRequest represents the request to publish an execution event
type PublishEventRequest struct {
	EventType EventType       `json:"event_type"`
	EventData json.RawMessage `json:"event_data"`
}

// CompleteExecutionRequest represents the request to complete a workflow execution
type CompleteExecutionRequest struct {
	ExecutionID       string               `json:"execution_id"`
	WorkspaceID       string               `json:"workspace_id"`
	WorkflowID        string               `json:"workflow_id"`
	TriggerNodeID     string               `json:"trigger_node_id"`
	StartedAt         time.Time            `json:"started_at"`
	EndedAt           time.Time            `json:"ended_at"`
	NodeExecutions    []NodeExecution      `json:"node_executions"`
	HistoryEntries    []NodeExecutionEntry `json:"history_entries"`
	IsTestingWorkflow bool                 `json:"is_testing_workflow"`
}

// EncryptedCredential represents an encrypted credential for executor use
type EncryptedCredential struct {
	ID                 string `json:"id"`
	WorkspaceID        string `json:"workspace_id"`
	EphemeralPublicKey []byte `json:"ephemeral_public_key"` // 32 bytes X25519 ephemeral public key
	EncryptedPayload   []byte `json:"encrypted_payload"`
	Nonce              []byte `json:"nonce"` // 12 bytes for ChaCha20-Poly1305
	ExpiresAt          int64  `json:"expires_at"`
	ExecutorID         string `json:"executor_id"`
}

// EncryptedFullCredential represents an encrypted full credential struct for executor use
// Contains the entire domain.Credential struct encrypted (not just the payload)
type EncryptedFullCredential struct {
	ID                 string `json:"id"`
	WorkspaceID        string `json:"workspace_id"`
	EphemeralPublicKey []byte `json:"ephemeral_public_key"` // 32 bytes X25519 ephemeral public key
	EncryptedPayload   []byte `json:"encrypted_payload"`    // Full domain.Credential JSON encrypted
	Nonce              []byte `json:"nonce"`                // 12 bytes for ChaCha20-Poly1305
	ExpiresAt          int64  `json:"expires_at"`
	ExecutorID         string `json:"executor_id"`
}

// GetCredentialRequest represents the request to get an encrypted credential
type GetCredentialRequest struct {
	CredentialID string `json:"credential_id"`
	ExecutorID   string `json:"executor_id"`
}

// NodeExecutionEvent represents a node execution event
type NodeExecutionEvent struct {
	ExecutionID string                 `json:"execution_id"`
	NodeID      string                 `json:"node_id"`
	NodeType    string                 `json:"node_type"`
	StartedAt   time.Time              `json:"started_at"`
	EndedAt     *time.Time             `json:"ended_at,omitempty"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// WorkflowExecutionEvent represents a workflow execution completion event
type WorkflowExecutionEvent struct {
	ExecutionID    string          `json:"execution_id"`
	WorkspaceID    string          `json:"workspace_id"`
	WorkflowID     string          `json:"workflow_id"`
	StartedAt      time.Time       `json:"started_at"`
	EndedAt        time.Time       `json:"ended_at"`
	IsSuccessful   bool            `json:"is_successful"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	NodeExecutions []NodeExecution `json:"node_executions"`
}

// Task publishing types
type EnqueueTaskRequest struct {
	TaskType string      `json:"task_type"`
	TaskData interface{} `json:"task_data"`
}

type EnqueueTaskResponse struct {
	Success bool   `json:"success"`
	TaskID  string `json:"task_id,omitempty"`
}

type EnqueueTaskAndWaitRequest struct {
	TaskType string      `json:"task_type"`
	TaskData interface{} `json:"task_data"`
}

type EnqueueTaskAndWaitResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	TaskID  string      `json:"task_id,omitempty"`
}

type GetExecutionFileRequest struct {
	WorkspaceID string `json:"workspace_id"`
	UploadID    string `json:"upload_id"`
}

type ExecutionWorkspaceFile struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	SizeInBytes int64  `json:"size_in_bytes"`
	ContentType string `json:"content_type"`
	UploadedBy  string `json:"uploaded_by"`

	Reader io.ReadCloser `json:"-"`
}

// Upload related types

// UploadFileRequest represents the request to upload a file through the executor
type UploadFileRequest struct {
	WorkspaceID string           `json:"workspace_id"`
	FileName    string           `json:"file_name"`
	ContentType string           `json:"content_type"`
	TotalSize   int64            `json:"total_size"`
	ChunkSize   int64            `json:"chunk_size,omitempty"`
	Reader      io.Reader        `json:"-"`
	ProgressFn  ProgressCallback `json:"-"`
	FolderID    *string          `json:"folder_id,omitempty"`
}

// UploadFileResponse represents the response from uploading a file
type UploadFileResponse struct {
	UploadID string `json:"upload_id"`
	FileID   string `json:"file_id"`
	FileSize int64  `json:"file_size"`
	FileName string `json:"file_name"`
	Status   string `json:"status"`
}

// ProgressCallback is called during upload progress
type ProgressCallback func(uploaded, total int64, percent float64)

// InitiateUploadRequest represents the request to initiate an upload
type InitiateUploadRequest struct {
	FileName    string  `json:"file_name"`
	ContentType string  `json:"content_type"`
	TotalSize   int64   `json:"total_size"`
	ChunkSize   int64   `json:"chunk_size"`
	Checksum    string  `json:"checksum"`
	FolderID    *string `json:"folder_id,omitempty"`
}

// InitiateUploadResponse represents the response from initiating an upload
type InitiateUploadResponse struct {
	UploadID    string `json:"upload_id"`
	TotalChunks int64  `json:"total_chunks"`
	ChunkSize   int64  `json:"chunk_size"`
}

// CompleteUploadResponse represents the response from completing an upload
type CompleteUploadResponse struct {
	UploadID string `json:"upload_id"`
	FileID   string `json:"file_id,omitempty"`
}

// StreamFileRequest represents the request to stream a file with range support
type StreamFileRequest struct {
	WorkspaceID string `json:"workspace_id"`
	UploadID    string `json:"upload_id"`
	RangeHeader string `json:"range_header,omitempty"` // Optional range header for partial requests
}

// StreamFileResponse represents a streaming file response
type StreamFileResponse struct {
	Content       io.ReadCloser `json:"-"`
	ContentLength int64         `json:"content_length"`
	ContentRange  string        `json:"content_range,omitempty"`
	StatusCode    int           `json:"status_code"` // 200 for full content, 206 for partial content
	ContentType   string        `json:"content_type"`
	FileName      string        `json:"file_name"`
}

// GetFileReaderRequest represents the request to get a file as a streaming reader
type GetFileReaderRequest struct {
	WorkspaceID string `json:"workspace_id"`
	UploadID    string `json:"upload_id"`
	ChunkSize   int64  `json:"chunk_size,omitempty"` // Optional chunk size, defaults to 2MB
}

// GetFileReaderResult represents the result of getting a file reader with metadata
type GetFileReaderResult struct {
	Reader      io.ReadCloser `json:"-"`            // The streaming reader
	FileName    string        `json:"file_name"`    // Original filename
	ContentType string        `json:"content_type"` // MIME type
	FileSize    int64         `json:"file_size"`    // Total file size in bytes
	Status      string        `json:"status"`       // Upload status
}

// GetFileInfoRequest represents the request to get file metadata
type GetFileInfoRequest struct {
	WorkspaceID string `json:"workspace_id"`
	UploadID    string `json:"upload_id"`
}

// GetFileInfoResponse represents the response from getting file metadata
type GetFileInfoResponse struct {
	UploadID      string `json:"upload_id"`
	FileName      string `json:"file_name"`
	ContentType   string `json:"content_type"`
	SizeInBytes   int64  `json:"size_in_bytes"`
	SupportsRange bool   `json:"supports_range"`
	Status        string `json:"status"`
}

// PersistFileRequest represents the request to persist a file (remove expiration)
type PersistFileRequest struct {
	WorkspaceID string `json:"workspace_id"`
	UploadID    string `json:"upload_id"`
}

// PersistFileResponse represents the response from persisting a file
type PersistFileResponse struct {
	Success  bool   `json:"success"`
	UploadID string `json:"upload_id"`
	Message  string `json:"message"`
}

// Executor represents an executor in the system
type Executor struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Address          string    `json:"address"`
	WorkspaceIDs     []string  `json:"workspace_ids"`
	X25519PublicKey  string    `json:"x25519_public_key"`
	Ed25519PublicKey string    `json:"ed25519_public_key"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateExecutorRegistrationRequest represents the request to create an executor registration
type CreateExecutorRegistrationRequest struct {
	ExecutorName     string `json:"executor_name"`
	Address          string `json:"address"`
	X25519PublicKey  string `json:"x25519_public_key"`
	Ed25519PublicKey string `json:"ed25519_public_key"`
}

// CreateExecutorRegistrationResponse represents the response from creating an executor registration
type CreateExecutorRegistrationResponse struct {
	VerificationCode string `json:"verification_code"`
}

// VerifyExecutorRegistrationRequest represents the request to verify an executor registration
type VerifyExecutorRegistrationRequest struct {
	Code string `json:"code"`
}

// Workspace represents a workspace
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// File and folder management types

// WorkspaceFile represents a file in the workspace
type WorkspaceFile struct {
	UploadID    string     `json:"upload_id"`
	UploadedBy  string     `json:"uploaded_by"`
	WorkspaceID string     `json:"workspace_id"`
	FileName    string     `json:"file_name"`
	ContentType string     `json:"content_type"`
	Size        int64      `json:"size"`
	UploadedAt  time.Time  `json:"uploaded_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	TagIDs      []string   `json:"tag_ids"`
	FolderID    *string    `json:"folder_id"`
}

// Folder represents a folder in the workspace
type Folder struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	Name           string    `json:"name"`
	ParentFolderID *string   `json:"parent_folder_id"`
	Path           string    `json:"path"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedBy      string    `json:"created_by"`
	UpdatedAt      time.Time `json:"updated_at"`
	IsDeleted      bool      `json:"is_deleted"`
	Order          int       `json:"order"`
	FileCount      int64     `json:"file_count"`
}

// ListWorkspaceFilesRequest represents the request to list workspace files
type ListWorkspaceFilesRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	FolderID    *string `json:"folder_id,omitempty"` // nil for root level, specific ID for folder contents
	Cursor      string  `json:"cursor,omitempty"`
	Limit       int     `json:"limit,omitempty"`
}

// ListWorkspaceFilesResponse represents the response from listing workspace files
type ListWorkspaceFilesResponse struct {
	Files      []WorkspaceFile `json:"files"`
	NextCursor string          `json:"next_cursor"`
}

// DeleteFileRequest represents the request to delete a file
type DeleteFileRequest struct {
	WorkspaceID string `json:"workspace_id"`
	UploadID    string `json:"upload_id"`
}

// DeleteFileResponse represents the response from deleting a file
type DeleteFileResponse struct {
	Success     bool   `json:"success"`
	UploadID    string `json:"upload_id"`
	DeletedSize int64  `json:"deleted_size"`
	Message     string `json:"message"`
}

// ListFoldersRequest represents the request to list folders
type ListFoldersRequest struct {
	WorkspaceID    string  `json:"workspace_id"`
	ParentFolderID *string `json:"parent_folder_id,omitempty"` // nil for root level folders
	IncludeDeleted bool    `json:"include_deleted,omitempty"`
	AllFolders     bool    `json:"all_folders,omitempty"` // When true, ignore ParentFolderID and return all folders
}

// ListFoldersResponse represents the response from listing folders
type ListFoldersResponse struct {
	Folders []Folder `json:"folders"`
}

// OAuth account management types

// OAuthType represents the type of OAuth provider
type OAuthType string

const (
	OAuthTypeGoogle  OAuthType = "google"
	OAuthTypeSlack   OAuthType = "slack"
	OAuthTypeDropbox OAuthType = "dropbox"
	OAuthTypeGitHub  OAuthType = "github"
	OAuthTypeLinear  OAuthType = "linear"
	OAuthTypeJira    OAuthType = "jira"
)

// OAuthAccount represents an OAuth account without sensitive data (for executor use)
type OAuthAccount struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	OAuthName string                 `json:"oauth_name"`
	OAuthType OAuthType              `json:"oauth_type"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// GetOAuthAccountRequest represents the request to get an OAuth account
type GetOAuthAccountRequest struct {
	OAuthAccountID string `json:"oauth_account_id"`
}

// GetOAuthAccountResponse represents the response from getting an OAuth account
type GetOAuthAccountResponse struct {
	OAuthAccount OAuthAccount `json:"oauth_account"`
}

// UpdateOAuthAccountMetadataRequest represents the request to update OAuth account metadata
type UpdateOAuthAccountMetadataRequest struct {
	Metadata map[string]interface{} `json:"metadata"`
}

// UpdateOAuthAccountMetadataResponse represents the response from updating OAuth account metadata
type UpdateOAuthAccountMetadataResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Schedule management types

// Schedule represents a polling schedule for workflow triggers
type Schedule struct {
	ID                        string    `json:"id"`
	WorkflowID                string    `json:"workflow_id"`
	ScheduleCreatedAt         time.Time `json:"schedule_created_at"`
	TriggerID                 string    `json:"trigger_id"`
	UserID                    string    `json:"user_id"`
	WorkflowType              string    `json:"workflow_type"`
	IntegrationType           string    `json:"integration_type"`
	LastCheckedAt             time.Time `json:"last_checked_at"`
	NextScheduledCheckAt      time.Time `json:"next_scheduled_check_at"`
	IsActive                  bool      `json:"is_active"`
	LastModifiedData          string    `json:"last_modified_data"`
	PollingScheduleGapSeconds int       `json:"polling_schedule_gap_seconds"`
}

// GetScheduleRequest represents the request to get a schedule
type GetScheduleRequest struct {
	WorkspaceID string `json:"workspace_id"`
	ScheduleID  string `json:"schedule_id"`
	WorkflowID  string `json:"workflow_id"`
}

// GetScheduleResponse represents the response from getting a schedule
type GetScheduleResponse struct {
	Schedule Schedule `json:"schedule"`
}

// CreateScheduleRequest represents the request to create a schedule
type CreateScheduleRequest struct {
	WorkflowID                string `json:"workflow_id"`
	TriggerID                 string `json:"trigger_id"`
	IntegrationType           string `json:"integration_type"`
	WorkflowType              string `json:"workflow_type"`
	UserID                    string `json:"user_id"`
	LastModifiedData          string `json:"last_modified_data"`
	PollingScheduleGapSeconds int    `json:"polling_schedule_gap_seconds"`
}

// CreateScheduleResponse represents the response from creating a schedule
type CreateScheduleResponse struct {
	Schedule Schedule `json:"schedule"`
}

// Agent memory types

// AgentConversation represents a conversation with an AI agent
type AgentConversation struct {
	ID             string                 `json:"id"`
	WorkspaceID    string                 `json:"workspace_id"`
	SessionID      string                 `json:"session_id"`
	ConversationID string                 `json:"conversation_id"`
	UserPrompt     string                 `json:"user_prompt"`
	FinalResponse  string                 `json:"final_response"`
	Messages       []AgentMessage         `json:"messages"`
	ToolsUsed      []string               `json:"tools_used"`
	Status         string                 `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// AgentMessage represents a message in an agent conversation
type AgentMessage struct {
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolCalls []AgentToolCall `json:"tool_calls,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// AgentToolCall represents a tool call made by an agent
type AgentToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// SaveAgentConversationResponse represents the response from saving an agent conversation
type SaveAgentConversationResponse struct {
	Success        bool   `json:"success"`
	ConversationID string `json:"conversation_id"`
}

// GetAgentConversationsRequest represents the request to get agent conversations
type GetAgentConversationsRequest struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
}

// GetAgentConversationsResponse represents the response from getting agent conversations
type GetAgentConversationsResponse struct {
	Conversations []*AgentConversation `json:"conversations"`
	Count         int                  `json:"count"`
}

// DeleteOldAgentConversationsRequest represents the request to delete old agent conversations
type DeleteOldAgentConversationsRequest struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	KeepCount   int    `json:"keep_count"`
}

// DeleteOldAgentConversationsResponse represents the response from deleting old agent conversations
type DeleteOldAgentConversationsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SearchKnowledgeRequest represents the request to search knowledge
type SearchKnowledgeRequest struct {
	Query               string  `json:"query"`
	Limit               int     `json:"limit"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
}

type WorkflowRoute struct {
	ID                     string         `json:"id"`
	UserID                 string         `json:"user_id"`
	WorkflowID             string         `json:"workflow_id"`
	RouteType              string         `json:"route_type"`
	RouteID                string         `json:"route_id"`
	TriggerID              string         `json:"trigger_id"`
	TriggerEventType       string         `json:"trigger_event_type"`
	TriggerIntegrationType string         `json:"trigger_integration_type"`
	Metadata               map[string]any `json:"metadata"`
	WorkflowType           string         `json:"workflow_type"`
	IsWebhook              bool           `json:"is_webhook"`
	WorkspaceID            string         `json:"workspace_id"`
}

type GetRoutesRequest struct {
	WorkspaceID      string `json:"workspace_id"`
	RouteID          string `json:"route_id"`
	TriggerEventType string `json:"trigger_event_type"`
	WorkflowType     string `json:"workflow_type"`
	IsWebhook        bool   `json:"is_webhook,omitempty"`
}

type GetRoutesResponse struct {
	Routes []WorkflowRoute `json:"routes"`
}
