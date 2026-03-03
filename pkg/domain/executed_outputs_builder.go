package domain

import (
	"context"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type ExecutionHistoryProvider interface {
	GetExecutionHistory(ctx context.Context, executionID string) (*ExecutionHistoryForOutputs, error)
}

type ExecutionHistoryForOutputs struct {
	NodeExecutions []NodeExecutionEntry
}

type NewExecutedOutputsContextParams struct {
	ExecutionHistoryProvider ExecutionHistoryProvider
	ExecutionID              string
	CurrentNodeID            string
}

func NewExecutedOutputsContext(ctx context.Context, params NewExecutedOutputsContextParams) context.Context {
	outputs, err := BuildExecutedOutputs(ctx, BuildExecutedOutputsParams{
		ExecutionHistoryProvider: params.ExecutionHistoryProvider,
		ExecutionID:              params.ExecutionID,
		CurrentNodeID:            params.CurrentNodeID,
	})
	if err != nil || outputs == nil {
		log.Error().Err(err).Msg("Failed to build executed outputs")
		return ctx
	}

	execCtx := &WorkflowExecutionContext{
		ExecutedOutputsProvider: func() map[string][][]Item { return outputs },
	}

	return context.WithValue(ctx, WorkflowExecutionContextKey{}, execCtx)
}

type BuildExecutedOutputsParams struct {
	ExecutionHistoryProvider ExecutionHistoryProvider
	ExecutionID              string
	CurrentNodeID            string
}

func BuildExecutedOutputs(ctx context.Context, params BuildExecutedOutputsParams) (map[string][][]Item, error) {
	history, err := params.ExecutionHistoryProvider.GetExecutionHistory(ctx, params.ExecutionID)
	if err != nil {
		return nil, err
	}
	if history == nil {
		return nil, nil
	}
	out := make(map[string][][]Item)
	for _, entry := range history.NodeExecutions {
		if entry.EventType != NodeExecuted {
			continue
		}
		if params.CurrentNodeID != "" && entry.NodeID == params.CurrentNodeID {
			continue
		}
		for outputID, nodeItems := range entry.ItemsByOutputID {
			nodeID := nodeItems.FromNodeID
			outputIndex := parseOutputIndexFromOutputID(outputID)
			if outputIndex < 0 {
				continue
			}
			if _, exists := out[nodeID]; !exists {
				out[nodeID] = [][]Item{}
			}
			for len(out[nodeID]) <= outputIndex {
				out[nodeID] = append(out[nodeID], []Item{})
			}
			out[nodeID][outputIndex] = append(out[nodeID][outputIndex], nodeItems.Items...)
		}
	}
	return out, nil
}

func parseOutputIndexFromOutputID(outputID string) int {
	lastDash := strings.LastIndex(outputID, "-")
	if lastDash < 0 {
		return -1
	}
	idx, err := strconv.Atoi(outputID[lastDash+1:])
	if err != nil {
		return -1
	}
	return idx
}
