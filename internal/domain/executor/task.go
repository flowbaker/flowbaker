package executor

import (
	"flowbaker/internal/domain"
	"sync"
)

type NodeExecutionTask struct {
	NodeID           string
	PayloadByInputID SourceNodePayloadByInputID
}

type WaitingExecutionTask struct {
	NodeID           string
	Payload          []byte
	ReceivedPayloads map[string]map[string]SourceNodePayload // inputID -> subscribed outputID -> payload
	mutex            *sync.Mutex
}

func (t WaitingExecutionTask) MergePayloadsByInputID() map[string]SourceNodePayload {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	payloadsByInputID := map[string]SourceNodePayload{}

	for inputID, p := range t.ReceivedPayloads {
		for _, payload := range p {
			payloadsByInputID[inputID] = payload
		}
	}

	return payloadsByInputID
}

func (t WaitingExecutionTask) AddPayload(sourceNodeID string, inputID string, outputID string, payload domain.Payload) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.ReceivedPayloads == nil {
		t.ReceivedPayloads = map[string]map[string]SourceNodePayload{}
	}

	if _, exists := t.ReceivedPayloads[inputID]; !exists {
		t.ReceivedPayloads[inputID] = map[string]SourceNodePayload{
			outputID: {
				SourceNodeID: sourceNodeID,
				Payload:      payload,
			},
		}
	}

	t.ReceivedPayloads[inputID][outputID] = SourceNodePayload{
		SourceNodeID: sourceNodeID,
		Payload:      payload,
	}
}
