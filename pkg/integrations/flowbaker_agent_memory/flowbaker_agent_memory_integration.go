package flowbaker_agent_memory

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/internal/domain"
)

type FlowbakerAgentMemoryIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
	client flowbaker.ClientInterface
}

func NewFlowbakerAgentMemoryIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &FlowbakerAgentMemoryIntegrationCreator{
		binder: deps.ParameterBinder,
		client: deps.FlowbakerClient,
	}
}

func (c *FlowbakerAgentMemoryIntegrationCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	deps := FlowbakerAgentMemoryIntegrationDependencies{
		Binder:      c.binder,
		Client:      c.client,
		WorkspaceID: params.WorkspaceID,
	}

	integration, err := NewFlowbakerAgentMemoryIntegration(ctx, deps)
	if err != nil {
		return nil, err
	}

	return integration, nil
}

type FlowbakerAgentMemoryIntegration struct {
	binder      domain.IntegrationParameterBinder
	client      flowbaker.ClientInterface
	workspaceID string

	actionManager *domain.IntegrationActionManager
}

type FlowbakerAgentMemoryIntegrationDependencies struct {
	Binder      domain.IntegrationParameterBinder
	Client      flowbaker.ClientInterface
	WorkspaceID string
}

func NewFlowbakerAgentMemoryIntegration(ctx context.Context, deps FlowbakerAgentMemoryIntegrationDependencies) (*FlowbakerAgentMemoryIntegration, error) {
	integration := &FlowbakerAgentMemoryIntegration{
		binder:      deps.Binder,
		client:      deps.Client,
		workspaceID: deps.WorkspaceID,
	}

	actionManager := domain.NewIntegrationActionManager()
	integration.actionManager = actionManager

	return integration, nil
}

func (i *FlowbakerAgentMemoryIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

// IntegrationMemory interface implementation

func (i *FlowbakerAgentMemoryIntegration) StoreConversation(ctx context.Context, request domain.StoreConversationRequest) error {
	if request.Conversation == nil {
		return fmt.Errorf("conversation is required")
	}

	// Convert domain AgentConversation to flowbaker AgentConversation
	flowbakerConversation := &flowbaker.AgentConversation{
		ID:             request.Conversation.ID,
		WorkspaceID:    request.Conversation.WorkspaceID,
		SessionID:      request.Conversation.SessionID,
		ConversationID: request.Conversation.ConversationID,
		UserPrompt:     request.Conversation.UserPrompt,
		FinalResponse:  request.Conversation.FinalResponse,
		Messages:       make([]flowbaker.AgentMessage, len(request.Conversation.Messages)),
		ToolsUsed:      request.Conversation.ToolsUsed,
		Status:         request.Conversation.Status,
		CreatedAt:      request.Conversation.CreatedAt,
		UpdatedAt:      request.Conversation.UpdatedAt,
		Metadata:       request.Conversation.Metadata,
	}

	// Convert messages
	for i, msg := range request.Conversation.Messages {
		flowbakerConversation.Messages[i] = flowbaker.AgentMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolCalls: make([]flowbaker.AgentToolCall, len(msg.ToolCalls)),
			Timestamp: msg.Timestamp,
		}

		// Convert tool calls
		for j, tc := range msg.ToolCalls {
			flowbakerConversation.Messages[i].ToolCalls[j] = flowbaker.AgentToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}
	}

	_, err := i.client.SaveAgentConversation(ctx, i.workspaceID, flowbakerConversation)
	if err != nil {
		return fmt.Errorf("failed to save conversation via FlowbakerClient: %w", err)
	}

	return nil
}

func (i *FlowbakerAgentMemoryIntegration) RetrieveConversations(ctx context.Context, filter domain.ConversationFilter) ([]domain.AgentConversation, error) {
	req := &flowbaker.GetAgentConversationsRequest{
		WorkspaceID: filter.WorkspaceID,
		SessionID:   filter.SessionID,
		Limit:       filter.Limit,
		Offset:      0,
	}

	if req.Limit <= 0 {
		req.Limit = 10 // default limit
	}

	resp, err := i.client.GetAgentConversations(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversations via FlowbakerClient: %w", err)
	}

	// Convert flowbaker AgentConversations to domain AgentConversations
	conversations := make([]domain.AgentConversation, 0, len(resp.Conversations))
	for _, conv := range resp.Conversations {
		// Apply additional filters
		if filter.Status != "" && conv.Status != filter.Status {
			continue
		}
		if !filter.Since.IsZero() && conv.CreatedAt.Before(filter.Since) {
			continue
		}

		domainConv := domain.AgentConversation{
			ID:             conv.ID,
			WorkspaceID:    conv.WorkspaceID,
			SessionID:      conv.SessionID,
			ConversationID: conv.ConversationID,
			UserPrompt:     conv.UserPrompt,
			FinalResponse:  conv.FinalResponse,
			Messages:       make([]domain.AgentMessage, len(conv.Messages)),
			ToolsUsed:      conv.ToolsUsed,
			Status:         conv.Status,
			CreatedAt:      conv.CreatedAt,
			UpdatedAt:      conv.UpdatedAt,
			Metadata:       conv.Metadata,
		}

		// Convert messages
		for j, msg := range conv.Messages {
			domainConv.Messages[j] = domain.AgentMessage{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: make([]domain.AgentToolCall, len(msg.ToolCalls)),
				Timestamp: msg.Timestamp,
			}

			// Convert tool calls
			for k, tc := range msg.ToolCalls {
				domainConv.Messages[j].ToolCalls[k] = domain.AgentToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
		}

		conversations = append(conversations, domainConv)
	}

	return conversations, nil
}
