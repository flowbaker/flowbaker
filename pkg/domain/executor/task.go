package executor

import (
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type NodeExecutionTask struct {
	NodeID            string
	ItemsByInputIndex map[int]domain.NodeItems
}

type WaitingExecutionTask struct {
	ID               string
	NodeID           string
	ReceivedPayloads map[int]domain.NodeItems
	mutex            *sync.Mutex
}

func NewWaitingExecutionTask(nodeID string, payloads map[int]domain.NodeItems) WaitingExecutionTask {
	b := make([]byte, 16)
	rand.Read(b)

	return WaitingExecutionTask{
		ID:               fmt.Sprintf("%x", b),
		NodeID:           nodeID,
		ReceivedPayloads: payloads,
		mutex:            &sync.Mutex{},
	}
}

func (t WaitingExecutionTask) MergeItemsByInputIndex() map[int]domain.NodeItems {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	result := map[int]domain.NodeItems{}
	for inputIndex, nodeItems := range t.ReceivedPayloads {
		result[inputIndex] = nodeItems
	}

	return result
}

func (t WaitingExecutionTask) AddItems(fromNodeID string, inputIndex int, items []domain.Item) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.ReceivedPayloads == nil {
		t.ReceivedPayloads = map[int]domain.NodeItems{}
	}

	t.ReceivedPayloads[inputIndex] = domain.NodeItems{
		FromNodeID: fromNodeID,
		Items:      items,
	}
}
