package domain

import (
	"strconv"
	"strings"
)

type ExecutedOutputs map[string][][]Item

func BuildExecutedOutputs(entries []NodeExecutionEntry, currentNodeID string) ExecutedOutputs {
	out := make(ExecutedOutputs)
	for _, entry := range entries {
		if entry.EventType != NodeExecuted {
			continue
		}
		if currentNodeID != "" && entry.NodeID == currentNodeID {
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
	return out
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
