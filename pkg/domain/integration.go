package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/rs/zerolog/log"
)

var (
	ErrIntegrationNotFound = errors.New("integration not found")
)

type IntegrationType string
type IntegrationActionType string
type IntegrationTriggerEventType string
type IntegrationPeekableType string
type IntegrationPeekablePaginationType string

const (
	PeekablePaginationType_None   IntegrationPeekablePaginationType = "none"
	PeekablePaginationType_Cursor IntegrationPeekablePaginationType = "cursor"
	PeekablePaginationType_Offset IntegrationPeekablePaginationType = "offset"
)

const (
	IntegrationType_Empty                IntegrationType = "empty"
	IntegrationType_Discord              IntegrationType = "discord"
	IntegrationType_Switch               IntegrationType = "switch"
	IntegrationType_Slack                IntegrationType = "slack"
	IntegrationType_Dropbox              IntegrationType = "dropbox"
	IntegrationType_Email                IntegrationType = "email"
	IntegrationType_OpenAI               IntegrationType = "openai"
	IntegrationType_GoogleSheets         IntegrationType = "google_sheets"
	IntegrationType_HTTP                 IntegrationType = "http"
	IntegrationType_PostgreSQL           IntegrationType = "postgresql"
	IntegrationType_Webhook              IntegrationType = "webhook"
	IntegrationType_MongoDB              IntegrationType = "mongodb"
	IntegrationType_Cron                 IntegrationType = "cron"
	IntegrationType_Youtube              IntegrationType = "youtube"
	IntegrationType_AwsS3                IntegrationType = "aws_s3"
	IntegrationType_Transform            IntegrationType = "transform"
	IntegrationType_Condition            IntegrationType = "condition"
	IntegrationType_FileUpload           IntegrationType = "file_upload"
	IntegrationType_Click                IntegrationType = "click"
	IntegrationType_Gmail                IntegrationType = "gmail"
	IntegrationType_Drive                IntegrationType = "google_drive"
	IntegrationType_Github               IntegrationType = "github"
	IntegrationType_Redis                IntegrationType = "redis"
	IntegrationType_Linear               IntegrationType = "linear"
	IntegrationType_Anthropic            IntegrationType = "anthropic"
	IntegrationType_Gemini               IntegrationType = "gemini"
	IntegrationType_Resend               IntegrationType = "resend"
	IntegrationType_SendResponse         IntegrationType = "send_response"
	IntegrationType_JWT                  IntegrationType = "jwt"
	IntegrationType_Jira                 IntegrationType = "jira"
	IntegrationType_FlowbakerStorage     IntegrationType = "flowbaker_storage"
	IntegrationType_SplitField           IntegrationType = "split_field"
	IntegrationType_Stripe               IntegrationType = "stripe"
	IntegrationType_AIAgent              IntegrationType = "ai_agent"
	IntegrationType_SimpleMemory         IntegrationType = "simple_memory"
	IntegrationType_FlowbakerAgentMemory IntegrationType = "flowbaker_agent_memory"
	IntegrationType_Knowledge            IntegrationType = "flowbaker_knowledge"
	IntegrationType_Base64               IntegrationType = "base64"
	IntegrationType_ContentClassifier    IntegrationType = "content_classifier"
	IntegrationType_Teams                IntegrationType = "teams"
	IntegrationType_Pipedrive            IntegrationType = "pipedrive"
	IntegrationType_StartupsWatch        IntegrationType = "startups_watch"
	IntegrationType_Manipulation         IntegrationType = "manipulation"
	IntegrationType_SplitArray           IntegrationType = "split_array"
	IntegrationType_RawFileToItem        IntegrationType = "rawfiletoitem"
	IntegrationType_BrightData           IntegrationType = "brightdata"
	IntegrationType_ItemsToItem          IntegrationType = "items_to_item"
	IntegrationType_Telegram             IntegrationType = "telegram"
	IntegrationType_Notion               IntegrationType = "notion"
	IntegrationType_OnError              IntegrationType = "on_error"
	IntegrationType_Toolset              IntegrationType = "toolset"
	IntegrationType_ChatTrigger          IntegrationType = "chat_trigger"
	IntegrationType_Groq                 IntegrationType = "groq"
)

type Integration struct {
	ID          IntegrationType `json:"id" bson:"id"`
	Name        string          `json:"name" bson:"name"`
	Description string          `json:"description" bson:"description"`

	CredentialProperties []NodeProperty              `json:"credential_props" bson:"credential_properties"`
	Actions              []IntegrationAction         `json:"actions" bson:"actions"`
	Triggers             []IntegrationTrigger        `json:"triggers" bson:"triggers"`
	EmbeddingModels      []IntegrationEmbeddingModel `json:"embedding_models,omitempty" bson:"embedding_models,omitempty"`

	IsGroup bool `json:"is_group" bson:"is_group"`

	CanTestConnection    bool `json:"can_test_connection" bson:"can_test_connection"`
	IsCredentialOptional bool `json:"is_credential_optional" bson:"is_credential_optional"`
}

func (i Integration) GetActionByType(actionType IntegrationActionType) (IntegrationAction, bool) {
	for _, action := range i.Actions {
		if action.ActionType == actionType {
			return action, true
		}
	}

	return IntegrationAction{}, false
}

func (i Integration) GetTriggerByType(triggerType IntegrationTriggerEventType) (IntegrationTrigger, bool) {
	for _, trigger := range i.Triggers {
		if trigger.EventType == triggerType {
			return trigger, true
		}
	}

	return IntegrationTrigger{}, false
}

type IntegrationTrigger struct {
	ID                            string                                `json:"id" bson:"id"`
	EventType                     IntegrationTriggerEventType           `json:"event_type" bson:"event_type"`
	Name                          string                                `json:"name" bson:"name"`
	Description                   string                                `json:"description" bson:"description"`
	Properties                    []NodeProperty                        `json:"properties" bson:"properties"`
	HandlesByContext              map[ActionUsageContext]ContextHandles `json:"handles_by_context" bson:"handles_by_context"`
	IsNonAvailableForDefaultOAuth bool                                  `json:"is_non_available_for_default_oauth" bson:"is_non_available_for_default_oauth"`
	Decoration                    NodeDecoration                        `json:"decoration" bson:"decoration"`
}

type NodeDecoration struct {
	HasButton     bool `json:"has_button" bson:"has_button"`
	DisableEditor bool `json:"disable_editor" bson:"disable_editor"`
}

// ActionUsageContext represents the context in which an integration is being used
type ActionUsageContext string

const (
	UsageContextWorkflow       ActionUsageContext = "workflow"        // Regular workflow automation
	UsageContextTool           ActionUsageContext = "tool"            // AI Agent tool
	UsageContextLLMProvider    ActionUsageContext = "llm_provider"    // LLM provider for AI agents
	UsageContextMemoryProvider ActionUsageContext = "memory_provider" // Memory provider for AI agents
)

type NodeHandleType string
type NodeHandlePosition string

var (
	NodeHandleTypeDefault     NodeHandleType = "default"
	NodeHandleTypeSuccess     NodeHandleType = "success"
	NodeHandleTypeDestructive NodeHandleType = "destructive"
)

var (
	NodeHandlePositionTop    NodeHandlePosition = "top"
	NodeHandlePositionBottom NodeHandlePosition = "bottom"
	NodeHandlePositionLeft   NodeHandlePosition = "left"
	NodeHandlePositionRight  NodeHandlePosition = "right"
)

type NodeHandle struct {
	Index               int                `json:"index" bson:"index"`
	Type                NodeHandleType     `json:"type" bson:"type"`
	Position            NodeHandlePosition `json:"position,omitempty" bson:"position,omitempty"`
	Text                string             `json:"text,omitempty" bson:"text,omitempty"`
	UsageContext        ActionUsageContext `json:"usage_context,omitempty" bson:"usage_context,omitempty"`
	AllowedIntegrations []IntegrationType  `json:"allowed_integrations,omitempty" bson:"allowed_integrations,omitempty"`
}

type ContextHandles struct {
	Input  []NodeHandle `json:"input" bson:"input"`
	Output []NodeHandle `json:"output" bson:"output"`
}

type IntegrationAction struct {
	ID                string                                `json:"id" bson:"id"`
	ActionType        IntegrationActionType                 `json:"action_type" bson:"action_type"`
	Name              string                                `json:"name" bson:"name"`
	Description       string                                `json:"description" bson:"description"`
	Properties        []NodeProperty                        `json:"properties" bson:"properties"`
	HandlesByContext  map[ActionUsageContext]ContextHandles `json:"handles_by_context" bson:"handles_by_context"`
	SupportedContexts []ActionUsageContext                  `json:"supported_contexts" bson:"supported_contexts"`
	CombinedContexts  []ActionUsageContext                  `json:"combined_contexts" bson:"combined_contexts"`

	IsNonAvailableForDefaultOAuth bool           `json:"is_non_available_for_default_oauth" bson:"is_non_available_for_default_oauth"`
	Decoration                    NodeDecoration `json:"decoration" bson:"decoration"`
}

type IntegrationEmbeddingModel struct {
	ID          string `json:"id" bson:"id"`
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
	IsInternal  bool   `json:"is_internal" bson:"is_internal"`
}

type IntegrationInput struct {
	NodeID            string
	PayloadByInputID  map[string]Payload
	IntegrationParams IntegrationParams
	ActionType        IntegrationActionType
	Workflow          *Workflow
}

func (i IntegrationInput) GetItemsByInputID() (map[string][]Item, error) {
	itemsByInputID := map[string][]Item{}

	for inputID, payload := range i.PayloadByInputID {
		items, err := payload.ToItems()
		if err != nil {
			return nil, err
		}

		itemsByInputID[inputID] = items
	}

	return itemsByInputID, nil
}

func (i IntegrationInput) GetAllItems() ([]Item, error) {
	itemsByInputID, err := i.GetItemsByInputID()
	if err != nil {
		return nil, err
	}

	items := []Item{}

	for _, inputItems := range itemsByInputID {
		items = append(items, inputItems...)
	}

	return items, nil
}

type IntegrationParams struct {
	Settings map[string]any
}

type IntegrationOutput struct {
	ResultJSONByOutputID []Payload
}

func (o IntegrationOutput) ToItemsByOutputID(nodeID string) map[string]NodeItems {
	itemsByOutputID := map[string]NodeItems{}

	for outputIndex, payload := range o.ResultJSONByOutputID {
		items, err := payload.ToItems()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to convert payload to items for output %d", outputIndex)
			continue
		}

		outputID := fmt.Sprintf("output-%s-%d", nodeID, outputIndex)

		itemsByOutputID[outputID] = NodeItems{
			FromNodeID: nodeID,
			Items:      items,
		}
	}

	return itemsByOutputID
}

type IntegrationDeps struct {
	FlowbakerClient            flowbaker.ClientInterface
	ExecutorTaskPublisher      ExecutorTaskPublisher
	TaskSchedulerService       TaskSchedulerService
	ParameterBinder            IntegrationParameterBinder
	IntegrationSelector        IntegrationSelector
	ExecutorStorageManager     ExecutorStorageManager
	ExecutorCredentialManager  ExecutorCredentialManager
	ExecutorIntegrationManager ExecutorIntegrationManager
	ExecutorScheduleManager    ExecutorScheduleManager
	ExecutorKnowledgeManager   ExecutorKnowledgeManager
	ExecutorModelManager       ExecutorModelManager
}

type IntegrationParameterBinder interface {
	BindToStruct(ctx context.Context, item any, params any, expressions map[string]any) error
}
