package flowbaker_agent_memory

import (
	"context"
	"fmt"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/rs/zerolog/log"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type Memory struct {
	binder      domain.IntegrationParameterBinder
	client      flowbaker.ClientInterface
	workspaceID string
}

type MemoryDependencies struct {
	Binder      domain.IntegrationParameterBinder
	Client      flowbaker.ClientInterface
	WorkspaceID string
}

func New(ctx context.Context, deps MemoryDependencies) (*Memory, error) {
	memory := &Memory{
		binder:      deps.Binder,
		client:      deps.Client,
		workspaceID: deps.WorkspaceID,
	}

	if memory.workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	return memory, nil
}

func (m *Memory) SaveConversation(ctx context.Context, conversation types.Conversation) error {
	log.Debug().Interface("conversation", conversation).Msg("Saving conversation")

	if conversation.ID == "" {
		return fmt.Errorf("failed to save conversation: conversation ID is required")
	}

	if conversation.SessionID == "" {
		return fmt.Errorf("failed to save conversation: session ID is required")
	}

	convertedMessages := make([]flowbaker.Message, len(conversation.Messages))

	for i, msg := range conversation.Messages {
		convertedToolCalls := make([]flowbaker.ToolCall, len(msg.ToolCalls))

		for j, tc := range msg.ToolCalls {
			convertedToolCalls[j] = flowbaker.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}

		convertedToolResults := make([]flowbaker.ToolResult, len(msg.ToolResults))
		for k, tr := range msg.ToolResults {
			convertedToolResults[k] = flowbaker.ToolResult{
				ToolCallID: tr.ToolCallID,
				Content:    tr.Content,
				IsError:    tr.IsError,
			}
		}

		convertedMessages[i] = flowbaker.Message{
			Role:        flowbaker.MessageRole(msg.Role),
			Content:     msg.Content,
			ToolCalls:   convertedToolCalls,
			ToolResults: convertedToolResults,
			Timestamp:   msg.Timestamp,
			Metadata:    msg.Metadata,
		}
	}

	convertedConversation := &flowbaker.AgentConversation{
		ID:        conversation.ID,
		SessionID: conversation.SessionID,
		UserID:    conversation.UserID,
		Messages:  convertedMessages,
		CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt,
		Status:    flowbaker.ConversationStatus(conversation.Status),
	}

	_, err := m.client.SaveAgentConversation(ctx, m.workspaceID, convertedConversation)
	if err != nil {
		return fmt.Errorf("failed to save conversation via FlowbakerClient: %w", err)
	}

	return nil
}

func (m *Memory) GetConversation(ctx context.Context, filter memory.Filter) (types.Conversation, error) {
	log.Debug().Interface("filter", filter).Msg("Getting conversation")

	if filter.SessionID == "" {
		return types.Conversation{}, fmt.Errorf("failed to get conversation: session ID is required")
	}

	req := &flowbaker.GetAgentConversationRequest{
		WorkspaceID: m.workspaceID,
		SessionID:   filter.SessionID,
	}

	resp, err := m.client.GetAgentConversation(ctx, req)
	if err != nil {
		return types.Conversation{}, fmt.Errorf("failed to get conversations via FlowbakerClient: %w", err)
	}

	conv := resp.Conversation

	convertedMessages := make([]types.Message, len(conv.Messages))

	for j, msg := range conv.Messages {
		convertedToolCalls := make([]types.ToolCall, len(msg.ToolCalls))
		for k, tc := range msg.ToolCalls {
			convertedToolCalls[k] = types.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}

		convertedToolResults := make([]types.ToolResult, len(msg.ToolResults))
		for l, tr := range msg.ToolResults {
			convertedToolResults[l] = types.ToolResult{
				ToolCallID: tr.ToolCallID,
				Content:    tr.Content,
				IsError:    tr.IsError,
			}
		}
		convertedMessages[j] = types.Message{
			Role:        types.MessageRole(msg.Role),
			Content:     msg.Content,
			ToolCalls:   convertedToolCalls,
			ToolResults: convertedToolResults,
			Timestamp:   msg.Timestamp,
			Metadata:    msg.Metadata,
		}
	}

	return types.Conversation{
		ID:        conv.ID,
		SessionID: conv.SessionID,
		UserID:    conv.UserID,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
		Status:    types.ConversationStatus(conv.Status),
		Metadata:  conv.Metadata,
		Messages:  convertedMessages,
	}, nil
}
