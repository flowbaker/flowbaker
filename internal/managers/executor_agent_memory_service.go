package managers

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type ExecutorAgentMemoryService struct {
	client flowbaker.ClientInterface
}

type ExecutorAgentMemoryServiceDependencies struct {
	Client flowbaker.ClientInterface
}

func NewExecutorAgentMemoryService(deps ExecutorAgentMemoryServiceDependencies) domain.AgentMemoryService {
	return &ExecutorAgentMemoryService{
		client: deps.Client,
	}
}

func (s *ExecutorAgentMemoryService) SaveConversation(ctx context.Context, conversation *domain.AgentConversation) error {
	if conversation == nil {
		return fmt.Errorf("conversation is required")
	}

	flowbakerConversation := &flowbaker.AgentConversation{
		ID:             conversation.ID,
		WorkspaceID:    conversation.WorkspaceID,
		SessionID:      conversation.SessionID,
		ConversationID: conversation.ConversationID,
		UserPrompt:     conversation.UserPrompt,
		FinalResponse:  conversation.FinalResponse,
		Messages:       make([]flowbaker.AgentMessage, len(conversation.Messages)),
		ToolsUsed:      conversation.ToolsUsed,
		Status:         conversation.Status,
		CreatedAt:      conversation.CreatedAt,
		UpdatedAt:      conversation.UpdatedAt,
		Metadata:       conversation.Metadata,
	}

	for i, msg := range conversation.Messages {
		flowbakerConversation.Messages[i] = flowbaker.AgentMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolCalls: make([]flowbaker.AgentToolCall, len(msg.ToolCalls)),
			Timestamp: msg.Timestamp,
		}

		for j, tc := range msg.ToolCalls {
			flowbakerConversation.Messages[i].ToolCalls[j] = flowbaker.AgentToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}
	}

	_, err := s.client.SaveAgentConversation(ctx, conversation.WorkspaceID, flowbakerConversation)
	if err != nil {
		return fmt.Errorf("failed to save conversation via FlowbakerClient: %w", err)
	}

	return nil
}

func (s *ExecutorAgentMemoryService) GetConversations(ctx context.Context, params domain.GetConversationsParams) ([]*domain.AgentConversation, error) {
	req := &flowbaker.GetAgentConversationsRequest{
		WorkspaceID: params.WorkspaceID,
		SessionID:   params.SessionID,
		Limit:       params.Limit,
		Offset:      params.Offset,
	}

	resp, err := s.client.GetAgentConversations(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversations via FlowbakerClient: %w", err)
	}

	conversations := make([]*domain.AgentConversation, len(resp.Conversations))
	for i, conv := range resp.Conversations {
		conversations[i] = &domain.AgentConversation{
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
			conversations[i].Messages[j] = domain.AgentMessage{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: make([]domain.AgentToolCall, len(msg.ToolCalls)),
				Timestamp: msg.Timestamp,
			}

			// Convert tool calls
			for k, tc := range msg.ToolCalls {
				conversations[i].Messages[j].ToolCalls[k] = domain.AgentToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				}
			}
		}
	}

	return conversations, nil
}

func (s *ExecutorAgentMemoryService) DeleteOldConversations(ctx context.Context, workspaceID string, sessionID string, keepCount int) error {
	req := &flowbaker.DeleteOldAgentConversationsRequest{
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		KeepCount:   keepCount,
	}

	_, err := s.client.DeleteOldAgentConversations(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete old conversations via FlowbakerClient: %w", err)
	}

	return nil
}
