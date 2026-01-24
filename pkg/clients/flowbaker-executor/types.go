package executor

import (
	"time"

	api "github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

// HealthCheckResponse represents the response from a health check
type HealthCheckResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// WorkflowType represents the type of workflow execution
type WorkflowType string

const (
	WorkflowTypeDefault WorkflowType = "default"
	WorkflowTypeTesting WorkflowType = "testing"
)

// WorkflowActivationStatus represents the activation status of a workflow
type WorkflowActivationStatus string

const (
	WorkflowActivationStatusActive   WorkflowActivationStatus = "active"
	WorkflowActivationStatusInactive WorkflowActivationStatus = "inactive"
)

// IntegrationType represents the type of integration
type IntegrationType string

// IntegrationActionType represents the type of integration action
type IntegrationActionType string

// IntegrationTriggerEventType represents the type of integration trigger event
type IntegrationTriggerEventType string

type StartExecutionRequest struct {
	ExecutionID     string           `json:"execution_id"`
	UserID          *string          `json:"user_id,omitempty"`
	EventName       string           `json:"event_name"`
	PayloadJSON     []byte           `json:"payload_json"`
	EnableEvents    bool             `json:"enable_events"`
	WorkflowType    WorkflowType     `json:"workflow_type"`
	Workspace       Workspace        `json:"workspace"`
	Workflow        *Workflow        `json:"workflow,omitempty"`
	TestingWorkflow *TestingWorkflow `json:"testing_workflow,omitempty"`
}

type StopExecutionRequest struct {
	ExecutionID string `json:"execution_id"`
}

type StopExecutionResponse struct {
	Success bool `json:"success"`
}

// TestingWorkflow represents a testing workflow that references a parent workflow
type TestingWorkflow struct {
	ParentWorkflowID string    `json:"parent_workflow_id"`
	UserID           string    `json:"user_id"`
	Workflow         Workflow  `json:"workflow"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// StartExecutionResponse represents the response from starting a workflow execution
type StartExecutionResponse struct {
	ExecutionResult ExecutionResult `json:"execution_result"`
}

type ExecutionResult struct {
	Payload    []byte              `json:"payload,omitempty"`
	Headers    map[string][]string `json:"headers"`
	StatusCode int                 `json:"status_code"`
}

// Workspace represents a workspace in the executor context
type Workspace struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	OwnerID     string         `json:"owner_id"`
	Slug        string         `json:"slug"`
	AvatarURL   string         `json:"avatar_url"`
	Usage       WorkspaceUsage `json:"usage"`
	Plan        WorkspacePlan  `json:"plan"`
}

// WorkspaceUsage represents the usage metrics of a workspace
type WorkspaceUsage struct {
	StorageUsageInBytes int64 `json:"storage_usage_in_bytes"`
	TaskUsageCount      int64 `json:"task_usage_count"`
	FolderCount         int64 `json:"folder_count"`
}

// WorkspacePlan represents the plan limits of a workspace
type WorkspacePlan struct {
	StorageLimitInBytes int64 `json:"storage_limit_in_bytes"`
	TaskUsageLimit      int64 `json:"task_usage_limit"`
	FolderLimit         int64 `json:"folder_limit"`
}

// Workflow represents a workflow in the executor context
type Workflow struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Description      string                   `json:"description"`
	WorkspaceID      string                   `json:"workspace_id"`
	AuthorUserID     string                   `json:"author_user_id"`
	Slug             string                   `json:"slug"`
	Nodes            []WorkflowNode           `json:"nodes"`
	Settings         WorkflowSettings         `json:"settings"`
	LastUpdatedAt    int64                    `json:"last_updated_at"`
	ActivationStatus WorkflowActivationStatus `json:"activation_status"`
}

type WorkflowSettings struct {
	NodeExecutionLimit int `json:"node_execution_limit"`
}

type NodeType string

const (
	NodeTypeAction  NodeType = "action"
	NodeTypeTrigger NodeType = "trigger"
)

// WorkflowNode represents a node in a workflow
type WorkflowNode struct {
	ID                           string          `json:"id"`
	WorkflowID                   string          `json:"workflow_id"`
	Name                         string          `json:"name"`
	Type                         NodeType        `json:"type"`
	IntegrationType              IntegrationType `json:"integration_type"`
	IntegrationSettings          map[string]any  `json:"integration_settings"`
	Settings                     NodeSettings    `json:"common_settings"`
	ExpressionSelectedProperties []string        `json:"expression_selected_properties"`
	ProvidedByAgent              []string        `json:"provided_by_agent"`
	XPosition                    float64         `json:"x_position"`
	YPosition                    float64         `json:"y_position"`
	Inputs                       []NodeInput     `json:"inputs"`
	UsageContext                 string          `json:"usage_context,omitempty"`
	ParentID                     string          `json:"parent_id,omitempty"`
	ActionNodeOpts               ActionNodeOpts  `json:"action_node_opts,omitempty"`
	TriggerNodeOpts              TriggerNodeOpts `json:"trigger_node_opts,omitempty"`
}

type ActionNodeOpts struct {
	ActionType IntegrationActionType `json:"action_type"`
}

type TriggerNodeOpts struct {
	EventType IntegrationTriggerEventType `json:"event_type"`
}

// NodeInput represents an input for a workflow node
type NodeInput struct {
	InputID          string   `json:"input_id"`
	SubscribedEvents []string `json:"subscribed_events"`
}

type NodeSettings struct {
	ReturnErrorAsItem       bool `json:"return_error_as_item"`
	OverwriteExecutionLimit bool `json:"overwrite_execution_limit"`
	ExecutionLimit          int  `json:"execution_limit"`
}

type WorkflowLoop struct {
	ID        int      `json:"id"`
	Threshold int      `json:"threshold"`
	EdgeIDs   []string `json:"edge_ids"`
	NodeIDs   []string `json:"node_ids"`
}

// PollingEventRequest represents a request to handle a polling event
type PollingEventRequest struct {
	IntegrationType IntegrationType `json:"integration_type"`
	Trigger         WorkflowNode    `json:"trigger"`
	Workflow        Workflow        `json:"workflow"`
	UserID          string          `json:"user_id"`
	WorkflowType    WorkflowType    `json:"workflow_type"`
}

// PollingEventResponse represents the response from handling a polling event
type PollingEventResponse struct {
	LastModifiedData string `json:"last_modified_data"`
}

// ConnectionTestRequest represents a request to test a connection
type ConnectionTestRequest struct {
	IntegrationType IntegrationType `json:"integration_type"`
	CredentialID    string          `json:"credential_id"`
	Payload         map[string]any  `json:"payload"`
}

// ConnectionTestResponse represents the response from testing a connection
type ConnectionTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// PeekDataRequest represents a request to peek data from an integration
type PeekDataRequest struct {
	IntegrationType IntegrationType          `json:"integration_type"`
	CredentialID    string                   `json:"credential_id"`
	UserID          string                   `json:"user_id"`
	PeekableType    string                   `json:"peekable_type"`
	Cursor          string                   `json:"cursor,omitempty"`
	Pagination      *domain.PaginationParams `json:"pagination,omitempty"`
	PayloadJSON     []byte                   `json:"payload_json,omitempty"`
}

// PeekDataResponse represents the response from peeking data
type PeekDataResponse struct {
	Success    bool                      `json:"success"`
	Error      string                    `json:"error,omitempty"`
	Result     []PeekResultItem          `json:"result,omitempty"`
	Pagination domain.PaginationMetadata `json:"pagination,omitempty"`
}

// PeekResultItem represents an item in the peek result
type PeekResultItem struct {
	Key     string `json:"key"`
	Value   string `json:"value,omitempty"`
	Content string `json:"content,omitempty"`
}

// RegisterWorkspaceRequest represents a request to register a workspace
type RegisterWorkspaceRequest struct {
	ExecutorID string              `json:"executor_id"`
	Passcode   string              `json:"passcode"`
	Assignment WorkspaceAssignment `json:"assignment"`
}

// RegisterWorkspaceResponse represents the response from registering a workspace
type RegisterWorkspaceResponse struct {
	Success bool `json:"success"`
}

// WorkspaceAssignment represents an assignment of a workspace to an executor
type WorkspaceAssignment struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
	APIPublicKey  string `json:"api_public_key"`
}

type UnregisterWorkspaceResponse struct {
	Success bool `json:"success"`
}

type RerunNodeRequest struct {
	ExecutionID        string                 `json:"execution_id"`
	NodeID             string                 `json:"node_id"`
	Workflow           Workflow               `json:"workflow"`
	NodeExecutionEntry api.NodeExecutionEntry `json:"node_execution_entry"`
}

type RerunNodeResponse struct {
	Payload []byte `json:"payload"`
}

type RunNodeRequest struct {
	ExecutionID    string            `json:"execution_id"`
	NodeID         string            `json:"node_id"`
	Workflow       Workflow          `json:"workflow"`
	ItemsByInputID map[string][]byte `json:"items_by_input"`
}

type RunNodeResponse struct {
	Results []domain.NodeExecutionEntry `json:"results"`
}
