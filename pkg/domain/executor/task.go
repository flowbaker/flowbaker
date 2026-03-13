package executor

import (
	"sync"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type NodeExecutionTask struct {
	NodeID              string
	PayloadByInputIndex NodePayloadByInputIndex
}

type WaitingExecutionTask struct {
	NodeID           string
	Payload          []byte
	ReceivedPayloads map[int]map[int]NodePayload // input index -> subscribed output index -> payload
	mutex            *sync.Mutex
}

func (t WaitingExecutionTask) MergePayloadsByInputIndex() map[int]NodePayload {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	payloadsByInputIndex := map[int]NodePayload{}

	for inputIndex, p := range t.ReceivedPayloads {
		for _, payload := range p {
			payloadsByInputIndex[inputIndex] = payload
		}
	}

	return payloadsByInputIndex
}

func (t WaitingExecutionTask) AddPayload(sourceNodeID string, inputIndex int, outputIndex int, payload domain.Payload) {
	nodeID := t.NodeID

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.ReceivedPayloads == nil {
		t.ReceivedPayloads = map[int]map[int]NodePayload{}
	}

	if _, exists := t.ReceivedPayloads[inputIndex]; !exists {
		t.ReceivedPayloads[inputIndex] = map[int]NodePayload{
			outputIndex: {
				SourceNodeID: sourceNodeID,
				TargetNodeID: nodeID,
				Payload:      payload,
			},
		}
	}

	t.ReceivedPayloads[inputIndex][outputIndex] = NodePayload{
		SourceNodeID: sourceNodeID,
		TargetNodeID: nodeID,
		Payload:      payload,
	}
}
