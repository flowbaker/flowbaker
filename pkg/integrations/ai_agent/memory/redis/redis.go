package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/flowbaker/flowbaker/pkg/domain"
	redisint "github.com/flowbaker/flowbaker/pkg/integrations/redis"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

type Opts struct {
	CredentialID string
	KeyPrefix    string
	TTL          time.Duration
}

type StoreDeps struct {
	Context          context.Context
	CredentialGetter domain.CredentialGetter[redisint.RedisCredential]
}

type conversation struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	UserID    string         `json:"user_id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type message struct {
	ConversationID string         `json:"conversation_id"`
	Role           string         `json:"role"`
	Order          int            `json:"order"`
	Content        string         `json:"content"`
	ToolCalls      []toolCall     `json:"tool_calls,omitempty"`
	ToolResults    []toolResult   `json:"tool_results,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type toolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type toolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

func conversationFromTypes(c types.Conversation) conversation {
	return conversation{
		ID:        c.ID,
		SessionID: c.SessionID,
		UserID:    c.UserID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Status:    string(c.Status),
		Metadata:  c.Metadata,
	}
}

func (c conversation) toTypes(messages []types.Message) types.Conversation {
	return types.Conversation{
		ID:        c.ID,
		SessionID: c.SessionID,
		UserID:    c.UserID,
		Messages:  messages,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Status:    types.ConversationStatus(c.Status),
		Metadata:  c.Metadata,
	}
}

func messageFromTypes(m types.Message, order int) message {
	toolCalls := make([]toolCall, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		toolCalls[i] = toolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		}
	}

	toolResults := make([]toolResult, len(m.ToolResults))
	for i, tr := range m.ToolResults {
		toolResults[i] = toolResult{
			ToolCallID: tr.ToolCallID,
			Content:    tr.Content,
			IsError:    tr.IsError,
		}
	}

	return message{
		ConversationID: m.ConversationID,
		Role:           string(m.Role),
		Order:          order,
		Content:        m.Content,
		ToolCalls:      toolCalls,
		ToolResults:    toolResults,
		Timestamp:      m.Timestamp,
		Metadata:       m.Metadata,
		CreatedAt:      m.CreatedAt,
	}
}

func (m message) toTypes() types.Message {
	toolCalls := make([]types.ToolCall, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		toolCalls[i] = types.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		}
	}

	toolResults := make([]types.ToolResult, len(m.ToolResults))
	for i, tr := range m.ToolResults {
		toolResults[i] = types.ToolResult{
			ToolCallID: tr.ToolCallID,
			Content:    tr.Content,
			IsError:    tr.IsError,
		}
	}

	return types.Message{
		ConversationID: m.ConversationID,
		Role:           types.MessageRole(m.Role),
		Content:        m.Content,
		ToolCalls:      toolCalls,
		ToolResults:    toolResults,
		Timestamp:      m.Timestamp,
		Metadata:       m.Metadata,
		CreatedAt:      m.CreatedAt,
	}
}

func New(deps StoreDeps, opts Opts) (*Store, error) {
	credential, err := deps.CredentialGetter.GetDecryptedCredential(deps.Context, opts.CredentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	port, err := strconv.Atoi(credential.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	db := 0
	if credential.Database != "" {
		db, err = strconv.Atoi(credential.Database)
		if err != nil {
			return nil, fmt.Errorf("invalid database: %w", err)
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", credential.Host, port),
		Password: credential.Password,
		DB:       db,
		Username: credential.Username,
		TLSConfig: func() *tls.Config {
			if credential.TLS {
				serverName := credential.Host
				if credential.TLSServerName != "" {
					serverName = credential.TLSServerName
				}
				return &tls.Config{
					ServerName:         serverName,
					InsecureSkipVerify: credential.TLSSkipVerify,
				}
			}
			return nil
		}(),
	})

	if err := client.Ping(deps.Context).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Store{
		client:    client,
		keyPrefix: opts.KeyPrefix,
		ttl:       opts.TTL,
	}, nil
}

func (s *Store) convKey(sessionID string) string {
	if s.keyPrefix != "" {
		return fmt.Sprintf("%s:conversations:%s", s.keyPrefix, sessionID)
	}
	return fmt.Sprintf("conversations:%s", sessionID)
}

func (s *Store) msgKey(conversationID string) string {
	if s.keyPrefix != "" {
		return fmt.Sprintf("%s:messages:%s", s.keyPrefix, conversationID)
	}
	return fmt.Sprintf("messages:%s", conversationID)
}

func (s *Store) SaveConversation(ctx context.Context, conv types.Conversation) error {
	if conv.ID == "" {
		return types.ErrInvalidMessage
	}

	now := time.Now()
	conv.UpdatedAt = now

	doc := conversationFromTypes(conv)

	key := s.convKey(conv.SessionID)

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	err = s.client.HSet(ctx, key,
		"updated_at", doc.UpdatedAt.Format(time.RFC3339Nano),
		"status", doc.Status,
		"metadata", string(metadataJSON),
	).Err()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	msgKey := s.msgKey(conv.ID)

	for i := range conv.Messages {
		msg := &conv.Messages[i]
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = now
			msg.ConversationID = conv.ID

			msgDoc := messageFromTypes(*msg, i)
			msgJSON, err := json.Marshal(msgDoc)
			if err != nil {
				return fmt.Errorf("failed to marshal message: %w", err)
			}

			err = s.client.ZAdd(ctx, msgKey, redis.Z{
				Score:  float64(i),
				Member: string(msgJSON),
			}).Err()
			if err != nil {
				return fmt.Errorf("failed to save message: %w", err)
			}
		}
	}

	if s.ttl > 0 {
		s.client.Expire(ctx, key, s.ttl)
		s.client.Expire(ctx, msgKey, s.ttl)
	}

	return nil
}

func (s *Store) GetConversation(ctx context.Context, filter memory.Filter) (types.Conversation, error) {
	if filter.SessionID == "" {
		return types.Conversation{}, fmt.Errorf("failed to get conversation: session ID is required")
	}

	key := s.convKey(filter.SessionID)

	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return types.Conversation{}, fmt.Errorf("failed to get conversation: %w", err)
	}

	if len(result) == 0 {
		now := time.Now()
		newConv := conversation{
			ID:        uuid.New().String(),
			SessionID: filter.SessionID,
			UserID:    filter.UserID,
			Status:    string(types.StatusActive),
			CreatedAt: now,
			UpdatedAt: now,
			Metadata:  map[string]any{},
		}

		metadataJSON, _ := json.Marshal(newConv.Metadata)

		err = s.client.HSet(ctx, key,
			"id", newConv.ID,
			"session_id", newConv.SessionID,
			"user_id", newConv.UserID,
			"status", newConv.Status,
			"created_at", newConv.CreatedAt.Format(time.RFC3339Nano),
			"updated_at", newConv.UpdatedAt.Format(time.RFC3339Nano),
			"metadata", string(metadataJSON),
		).Err()
		if err != nil {
			return types.Conversation{}, fmt.Errorf("failed to create conversation: %w", err)
		}

		if s.ttl > 0 {
			s.client.Expire(ctx, key, s.ttl)
		}

		return newConv.toTypes([]types.Message{}), nil
	}

	conv, err := s.parseConversation(result)
	if err != nil {
		return types.Conversation{}, fmt.Errorf("failed to parse conversation: %w", err)
	}

	msgKey := s.msgKey(conv.ID)

	var msgResults []string

	if filter.Limit > 0 {
		// With pagination: use ZRevRangeByScore to get newest messages first
		maxScore := "+inf"
		if filter.Before != nil {
			// Exclusive upper bound: messages with order < before
			maxScore = fmt.Sprintf("(%d", *filter.Before)
		}

		msgResults, err = s.client.ZRevRangeByScore(ctx, msgKey, &redis.ZRangeBy{
			Min:   "-inf",
			Max:   maxScore,
			Count: int64(filter.Limit),
		}).Result()
		if err != nil {
			return types.Conversation{}, fmt.Errorf("failed to get messages: %w", err)
		}
	} else {
		// No pagination: get all messages in chronological order
		msgResults, err = s.client.ZRange(ctx, msgKey, 0, -1).Result()
		if err != nil {
			return types.Conversation{}, fmt.Errorf("failed to get messages: %w", err)
		}
	}

	messages := make([]types.Message, len(msgResults))
	for i, msgJSON := range msgResults {
		var msg message
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			return types.Conversation{}, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		messages[i] = msg.toTypes()
	}

	// If we used ZRevRangeByScore for pagination, reverse to chronological order
	if filter.Limit > 0 {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}

	return conv.toTypes(messages), nil
}

func (s *Store) parseConversation(data map[string]string) (conversation, error) {
	var conv conversation

	conv.ID = data["id"]
	conv.SessionID = data["session_id"]
	conv.UserID = data["user_id"]
	conv.Status = data["status"]

	if createdAt, ok := data["created_at"]; ok {
		t, err := time.Parse(time.RFC3339Nano, createdAt)
		if err == nil {
			conv.CreatedAt = t
		}
	}

	if updatedAt, ok := data["updated_at"]; ok {
		t, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err == nil {
			conv.UpdatedAt = t
		}
	}

	if metadataStr, ok := data["metadata"]; ok && metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &conv.Metadata); err != nil {
			return conversation{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return conv, nil
}
