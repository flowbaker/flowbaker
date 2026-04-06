package executor

import (
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type NodeExecutionTask struct {
	NodeID            string
	ItemsByInputIndex domain.NodeItemsMap
}

type WaitingExecutionTask struct {
	ID               string
	NodeID           string
	ReceivedPayloads domain.NodeItemsMap
	mutex            *sync.Mutex
}

func NewWaitingExecutionTask(nodeID string, payloads domain.NodeItemsMap) WaitingExecutionTask {
	b := make([]byte, 16)
	rand.Read(b)

	return WaitingExecutionTask{
		ID:               fmt.Sprintf("%x", b),
		NodeID:           nodeID,
		ReceivedPayloads: payloads,
		mutex:            &sync.Mutex{},
	}
}

func (t WaitingExecutionTask) AddItems(fromNodeID string, inputIndex int, items []domain.Item) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.ReceivedPayloads == nil {
		t.ReceivedPayloads = domain.NodeItemsMap{}
	}

	t.ReceivedPayloads.Set(inputIndex, fromNodeID, items)
}

func (t *WaitingExecutionTask) ToExecutionTask() NodeExecutionTask {
	return NodeExecutionTask{
		NodeID:            t.NodeID,
		ItemsByInputIndex: t.ReceivedPayloads,
	}
}
