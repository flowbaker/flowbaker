package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/flowbaker/flowbaker/pkg/domain"
	postgresqlint "github.com/flowbaker/flowbaker/pkg/integrations/postgresql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Store struct {
	conn        *pgx.Conn
	tablePrefix string
}

type Opts struct {
	CredentialID string
	TablePrefix  string
}

type StoreDeps struct {
	Context          context.Context
	CredentialGetter domain.CredentialGetter[postgresqlint.PostgreSQLCredential]
}

type conversation struct {
	ID        string
	SessionID string
	Status    string
	Metadata  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

type message struct {
	ConversationID string
	Role           string
	Order          int
	Content        string
	ToolCalls      []toolCall
	ToolResults    []toolResult
	Timestamp      time.Time
	Metadata       map[string]any
	CreatedAt      time.Time
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

	conn, err := pgx.Connect(deps.Context, credential.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	store := &Store{
		conn:        conn,
		tablePrefix: opts.TablePrefix,
	}

	if err := store.ensureTables(deps.Context); err != nil {
		conn.Close(deps.Context)
		return nil, fmt.Errorf("failed to ensure tables: %w", err)
	}

	return store, nil
}

func (s *Store) convTable() string {
	if s.tablePrefix != "" {
		return fmt.Sprintf("%s_conversations", s.tablePrefix)
	}
	return "conversations"
}

func (s *Store) msgTable() string {
	if s.tablePrefix != "" {
		return fmt.Sprintf("%s_messages", s.tablePrefix)
	}
	return "messages"
}

func (s *Store) ensureTables(ctx context.Context) error {
	convTable := s.convTable()
	msgTable := s.msgTable()

	createConvSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			status TEXT NOT NULL,
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)
	`, convTable)

	createConvIndexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_session ON %s(session_id)
	`, convTable, convTable)

	createMsgSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			role TEXT NOT NULL,
			msg_order INT NOT NULL,
			content TEXT,
			tool_calls JSONB,
			tool_results JSONB,
			timestamp TIMESTAMPTZ,
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL
		)
	`, msgTable)

	createMsgIndexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_conv ON %s(conversation_id, msg_order)
	`, msgTable, msgTable)

	_, err := s.conn.Exec(ctx, createConvSQL)
	if err != nil {
		return fmt.Errorf("failed to create conversations table: %w", err)
	}

	_, err = s.conn.Exec(ctx, createConvIndexSQL)
	if err != nil {
		return fmt.Errorf("failed to create conversations index: %w", err)
	}

	_, err = s.conn.Exec(ctx, createMsgSQL)
	if err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	_, err = s.conn.Exec(ctx, createMsgIndexSQL)
	if err != nil {
		return fmt.Errorf("failed to create messages index: %w", err)
	}

	return nil
}

func (s *Store) SaveConversation(ctx context.Context, conv types.Conversation) error {
	if conv.ID == "" {
		return types.ErrInvalidMessage
	}

	now := time.Now()
	conv.UpdatedAt = now

	doc := conversationFromTypes(conv)

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	updateSQL := fmt.Sprintf(`
		UPDATE %s SET updated_at = $1, status = $2, metadata = $3
		WHERE session_id = $4
	`, s.convTable())

	_, err = s.conn.Exec(ctx, updateSQL, doc.UpdatedAt, doc.Status, metadataJSON, conv.SessionID)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	for i := range conv.Messages {
		msg := &conv.Messages[i]
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = now
			msg.ConversationID = conv.ID

			msgDoc := messageFromTypes(*msg, i)

			toolCallsJSON, err := json.Marshal(msgDoc.ToolCalls)
			if err != nil {
				return fmt.Errorf("failed to marshal tool calls: %w", err)
			}

			toolResultsJSON, err := json.Marshal(msgDoc.ToolResults)
			if err != nil {
				return fmt.Errorf("failed to marshal tool results: %w", err)
			}

			msgMetadataJSON, err := json.Marshal(msgDoc.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal message metadata: %w", err)
			}

			insertSQL := fmt.Sprintf(`
				INSERT INTO %s (conversation_id, role, msg_order, content, tool_calls, tool_results, timestamp, metadata, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, s.msgTable())

			_, err = s.conn.Exec(ctx, insertSQL,
				msgDoc.ConversationID,
				msgDoc.Role,
				msgDoc.Order,
				msgDoc.Content,
				toolCallsJSON,
				toolResultsJSON,
				msgDoc.Timestamp,
				msgMetadataJSON,
				msgDoc.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("failed to save message: %w", err)
			}
		}
	}

	return nil
}

func (s *Store) GetConversation(ctx context.Context, filter memory.Filter) (types.Conversation, error) {
	if filter.SessionID == "" {
		return types.Conversation{}, fmt.Errorf("failed to get conversation: session ID is required")
	}

	selectSQL := fmt.Sprintf(`
		SELECT id, session_id, status, metadata, created_at, updated_at
		FROM %s WHERE session_id = $1
	`, s.convTable())

	row := s.conn.QueryRow(ctx, selectSQL, filter.SessionID)

	var conv conversation
	var metadataJSON []byte

	err := row.Scan(&conv.ID, &conv.SessionID, &conv.Status, &metadataJSON, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			now := time.Now()
			newConv := conversation{
				ID:        uuid.New().String(),
				SessionID: filter.SessionID,
				Status:    string(types.StatusActive),
				CreatedAt: now,
				UpdatedAt: now,
				Metadata:  map[string]any{},
			}

			metadataJSON, _ := json.Marshal(newConv.Metadata)

			insertSQL := fmt.Sprintf(`
				INSERT INTO %s (id, session_id, status, metadata, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, s.convTable())

			_, err = s.conn.Exec(ctx, insertSQL,
				newConv.ID,
				newConv.SessionID,
				newConv.Status,
				metadataJSON,
				newConv.CreatedAt,
				newConv.UpdatedAt,
			)
			if err != nil {
				return types.Conversation{}, fmt.Errorf("failed to create conversation: %w", err)
			}

			return newConv.toTypes([]types.Message{}), nil
		}
		return types.Conversation{}, fmt.Errorf("failed to get conversation: %w", err)
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &conv.Metadata); err != nil {
			return types.Conversation{}, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	selectMsgSQL := fmt.Sprintf(`
		SELECT conversation_id, role, msg_order, content, tool_calls, tool_results, timestamp, metadata, created_at
		FROM %s WHERE conversation_id = $1 ORDER BY msg_order
	`, s.msgTable())

	rows, err := s.conn.Query(ctx, selectMsgSQL, conv.ID)
	if err != nil {
		return types.Conversation{}, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []types.Message
	for rows.Next() {
		var msg message
		var toolCallsJSON, toolResultsJSON, msgMetadataJSON []byte
		var timestamp *time.Time

		err := rows.Scan(
			&msg.ConversationID,
			&msg.Role,
			&msg.Order,
			&msg.Content,
			&toolCallsJSON,
			&toolResultsJSON,
			&timestamp,
			&msgMetadataJSON,
			&msg.CreatedAt,
		)
		if err != nil {
			return types.Conversation{}, fmt.Errorf("failed to scan message: %w", err)
		}

		if timestamp != nil {
			msg.Timestamp = *timestamp
		}

		if toolCallsJSON != nil {
			if err := json.Unmarshal(toolCallsJSON, &msg.ToolCalls); err != nil {
				return types.Conversation{}, fmt.Errorf("failed to unmarshal tool calls: %w", err)
			}
		}

		if toolResultsJSON != nil {
			if err := json.Unmarshal(toolResultsJSON, &msg.ToolResults); err != nil {
				return types.Conversation{}, fmt.Errorf("failed to unmarshal tool results: %w", err)
			}
		}

		if msgMetadataJSON != nil {
			if err := json.Unmarshal(msgMetadataJSON, &msg.Metadata); err != nil {
				return types.Conversation{}, fmt.Errorf("failed to unmarshal message metadata: %w", err)
			}
		}

		messages = append(messages, msg.toTypes())
	}

	if err := rows.Err(); err != nil {
		return types.Conversation{}, fmt.Errorf("failed to iterate messages: %w", err)
	}

	return conv.toTypes(messages), nil
}
