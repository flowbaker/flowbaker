package executor

import (
	"sync"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type NodeExecutionTask struct {
	NodeID            string
	ItemsByInputIndex map[int]domain.NodeItems
}

type WaitingExecutionTask struct {
	NodeID           string
	ReceivedPayloads map[int]map[int]domain.NodeItems
	mutex            *sync.Mutex
}

// TODO: Enes: Examine merging strategy for multi input nodes
func (t WaitingExecutionTask) MergeItemsByInputIndex() map[int]domain.NodeItems {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	result := map[int]domain.NodeItems{}

	for inputIndex, byOutputIndex := range t.ReceivedPayloads {
		var allItems []domain.Item
		var fromNodeID string

		for _, nodeItems := range byOutputIndex {
			allItems = append(allItems, nodeItems.Items...)
			fromNodeID = nodeItems.FromNodeID
		}

		result[inputIndex] = domain.NodeItems{
			FromNodeID: fromNodeID,
			Items:      allItems,
		}
	}

	return result
}

func (t WaitingExecutionTask) AddItems(fromNodeID string, inputIndex int, outputIndex int, items []domain.Item) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.ReceivedPayloads == nil {
		t.ReceivedPayloads = map[int]map[int]domain.NodeItems{}
	}

	if _, exists := t.ReceivedPayloads[inputIndex]; !exists {
		t.ReceivedPayloads[inputIndex] = map[int]domain.NodeItems{}
	}

	existing, exists := t.ReceivedPayloads[inputIndex][outputIndex]
	if exists {
		existing.Items = append(existing.Items, items...)
		t.ReceivedPayloads[inputIndex][outputIndex] = existing
	} else {
		t.ReceivedPayloads[inputIndex][outputIndex] = domain.NodeItems{
			FromNodeID: fromNodeID,
			Items:      items,
		}
	}
}
