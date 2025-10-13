package executor

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

// HistoryRecorder records node execution history
type HistoryRecorder struct {
	historyEntries []domain.NodeExecutionEntry
	mutex          sync.Mutex
}

// NewHistoryRecorder creates a new history recorder
func NewHistoryRecorder() *HistoryRecorder {
	return &HistoryRecorder{
		historyEntries: []domain.NodeExecutionEntry{},
	}
}

// HandleEvent processes execution events and records history
func (h *HistoryRecorder) HandleEvent(ctx context.Context, event domain.ExecutionEvent) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	switch e := event.(type) {
	case NodeExecutionCompletedEvent:
		h.historyEntries = append(h.historyEntries, domain.NodeExecutionEntry{
			NodeID:          e.NodeID,
			ItemsByInputID:  e.ItemsByInputID,
			ItemsByOutputID: e.ItemsByOutputID,
			EventType:       domain.NodeExecuted,
			Timestamp:       e.EndedAt.UnixNano(),
			ExecutionOrder:  int(e.ExecutionOrder),
		})

	case NodeExecutionFailedEvent:
		h.historyEntries = append(h.historyEntries, domain.NodeExecutionEntry{
			NodeID:          e.NodeID,
			ItemsByInputID:  e.ItemsByInputID,
			ItemsByOutputID: map[string]domain.NodeItems{},
			EventType:       domain.NodeFailed,
			Error:           e.Error.Error(),
			Timestamp:       e.Timestamp.UnixNano(),
		})
	}

	return nil
}

// GetHistoryEntries returns all recorded history entries
func (h *HistoryRecorder) GetHistoryEntries() []domain.NodeExecutionEntry {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.historyEntries
}

// EventBroadcaster publishes events to external event publisher
type EventBroadcaster struct {
	orderedEventPublisher domain.EventPublisher
	enableEvents          bool
	workflowID            string
	executionID           string
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster(
	orderedEventPublisher domain.EventPublisher,
	enableEvents bool,
	workflowID string,
	executionID string,
) *EventBroadcaster {
	return &EventBroadcaster{
		orderedEventPublisher: orderedEventPublisher,
		enableEvents:          enableEvents,
		workflowID:            workflowID,
		executionID:           executionID,
	}
}

// HandleEvent processes execution events and publishes them externally
func (b *EventBroadcaster) HandleEvent(ctx context.Context, event domain.ExecutionEvent) error {
	if !b.enableEvents {
		return nil
	}

	switch e := event.(type) {
	case NodeExecutionStartedEvent:
		return b.orderedEventPublisher.PublishEvent(ctx, &domain.NodeExecutionStartedEvent{
			WorkflowID:          b.workflowID,
			WorkflowExecutionID: b.executionID,
			NodeID:              e.NodeID,
			Timestamp:           e.Timestamp.UnixNano(),
			IsReExecution:       e.IsReExecution,
		})

	case NodeExecutionCompletedEvent:
		return b.orderedEventPublisher.PublishEvent(ctx, &domain.NodeExecutedEvent{
			WorkflowID:          b.workflowID,
			WorkflowExecutionID: b.executionID,
			NodeID:              e.NodeID,
			Timestamp:           e.EndedAt.UnixNano(),
			ItemsByInputID:      e.ItemsByInputID,
			ItemsByOutputID:     e.ItemsByOutputID,
			ExecutionOrder:      int(e.ExecutionOrder),
			IsReExecution:       e.IsReExecution,
		})

	case NodeExecutionFailedEvent:
		return b.orderedEventPublisher.PublishEvent(ctx, &domain.NodeFailedEvent{
			WorkflowID:          b.workflowID,
			WorkflowExecutionID: b.executionID,
			NodeID:              e.NodeID,
			Timestamp:           e.Timestamp.UnixNano(),
			Error:               e.Error.Error(),
			ItemsByInputID:      e.ItemsByInputID,
			ItemsByOutputID:     map[string]domain.NodeItems{},
			IsReExecution:       e.IsReExecution,
		})

	case WorkflowExecutionCompletedEvent:
		return b.orderedEventPublisher.PublishEvent(ctx, &domain.WorkflowExecutionCompletedEvent{
			WorkflowID:          b.workflowID,
			WorkflowExecutionID: b.executionID,
			Timestamp:           e.Timestamp.UnixNano(),
		})
	}

	return nil
}

// UsageCollector collects node execution usage data
type UsageCollector struct {
	nodeExecutions []domain.NodeExecution
	mutex          sync.Mutex
}

// NewUsageCollector creates a new usage collector
func NewUsageCollector() *UsageCollector {
	return &UsageCollector{
		nodeExecutions: []domain.NodeExecution{},
	}
}

// HandleEvent processes execution events and collects usage data
func (u *UsageCollector) HandleEvent(ctx context.Context, event domain.ExecutionEvent) error {
	switch e := event.(type) {
	case NodeExecutionCompletedEvent:
		u.mutex.Lock()
		defer u.mutex.Unlock()

		inputItemsCount := domain.InputItemsCount{}
		inputItemsSizeInBytes := domain.InputItemsSizeInBytes{}

		for inputID, payload := range e.SourceNodePayloadByInputID {
			items, err := payload.Payload.ToItems()
			if err != nil {
				log.Error().Err(err).Msgf("Failed to parse JSON for input %s", inputID)
				continue
			}

			inputItemsCount[inputID] = int64(len(items))
			inputItemsSizeInBytes[inputID] = int64(len(payload.Payload))
		}

		outputItemsCount := domain.OutputItemsCount{}
		outputItemsSizeInBytes := domain.OutputItemsSizeInBytes{}

		for outputID, payload := range e.IntegrationOutput.ResultJSONByOutputID {
			var items []domain.Item
			err := json.Unmarshal(payload, &items)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to parse JSON for output %d", outputID)
				continue
			}

			outputItemsCount[int64(outputID)] = int64(len(items))
			outputItemsSizeInBytes[int64(outputID)] = int64(len(payload))
		}

		u.nodeExecutions = append(u.nodeExecutions, domain.NodeExecution{
			ID:                     xid.New().String(),
			NodeID:                 e.NodeID,
			IntegrationType:        e.IntegrationType,
			IntegrationActionType:  e.IntegrationActionType,
			StartedAt:              e.StartedAt,
			EndedAt:                e.EndedAt,
			ExecutionOrder:         e.ExecutionOrder,
			InputItemsCount:        inputItemsCount,
			InputItemsSizeInBytes:  inputItemsSizeInBytes,
			OutputItemsCount:       outputItemsCount,
			OutputItemsSizeInBytes: outputItemsSizeInBytes,
		})
	}

	return nil
}

// GetNodeExecutions returns all collected node executions
func (u *UsageCollector) GetNodeExecutions() []domain.NodeExecution {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.nodeExecutions
}
