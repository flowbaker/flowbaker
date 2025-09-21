package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/pkg/domain/mappers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

type SourceNodePayload struct {
	SourceNodeID string
	Payload      domain.Payload
}

type SourceNodePayloadByInputID map[string]SourceNodePayload

func (p SourceNodePayloadByInputID) ToPayloadByInputID() map[string]domain.Payload {
	payloadByInputID := map[string]domain.Payload{}

	for inputID, payload := range p {
		payloadByInputID[inputID] = payload.Payload
	}

	return payloadByInputID
}

type WorkflowExecutorI interface {
	Execute(ctx context.Context, nodeID string, payload domain.Payload) (ExecutionResult, error)
}

type WorkflowExecutor struct {
	executionID string
	workflow    domain.Workflow

	waitingExecutionTasks []WaitingExecutionTask
	executionQueue        []NodeExecutionTask
	executedNodes         map[string]struct{}

	historyEntries []domain.NodeExecutionEntry

	nodesByEventName       map[string][]domain.WorkflowNode
	executionCountByNodeID map[string]int

	mutex sync.Mutex

	integrationSelector domain.IntegrationSelector
	enableEvents        bool

	IsTestingWorkflow bool

	nodeExecutions             []domain.NodeExecution
	WorkflowExecutionStartedAt time.Time

	client                flowbaker.ClientInterface
	orderedEventPublisher domain.EventPublisher
}

type WorkflowExecutorDeps struct {
	Workflow              domain.Workflow
	Selector              domain.IntegrationSelector
	EventPublisher        domain.EventPublisher
	EnableEvents          bool
	IsTestingWorkflow     bool
	ExecutorClient        flowbaker.ClientInterface
	OrderedEventPublisher domain.EventPublisher
}

func NewWorkflowExecutor(deps WorkflowExecutorDeps) WorkflowExecutor {
	nodesByEventName := map[string][]domain.WorkflowNode{}

	for _, node := range deps.Workflow.Actions {
		for _, input := range node.Inputs {
			for _, eventName := range input.SubscribedEvents {
				nodes, ok := nodesByEventName[eventName]
				if !ok {
					nodes = []domain.WorkflowNode{}
				}

				nodes = append(nodes, node)

				nodesByEventName[eventName] = nodes
			}
		}
	}

	executionID := xid.New().String()

	return WorkflowExecutor{
		executionID:                executionID,
		workflow:                   deps.Workflow,
		waitingExecutionTasks:      []WaitingExecutionTask{},
		executionQueue:             []NodeExecutionTask{},
		executedNodes:              map[string]struct{}{},
		nodesByEventName:           nodesByEventName,
		integrationSelector:        deps.Selector,
		executionCountByNodeID:     map[string]int{},
		enableEvents:               deps.EnableEvents,
		historyEntries:             []domain.NodeExecutionEntry{},
		IsTestingWorkflow:          deps.IsTestingWorkflow,
		WorkflowExecutionStartedAt: time.Now(),
		client:                     deps.ExecutorClient,
		orderedEventPublisher:      deps.OrderedEventPublisher,
	}
}

const (
	InputHandleFormat  = "input-%s-%d"
	OutputHandleFormat = "output-%s-%d"

	MaxExecutionCount = 1000
)

func (w *WorkflowExecutor) Execute(ctx context.Context, nodeID string, payload domain.Payload) (ExecutionResult, error) {
	workspaceID := w.workflow.WorkspaceID

	log.Info().Msgf("Executing workflow %s in workspace %s", w.workflow.ID, workspaceID)

	ctx = domain.NewContextWithEventOrder(ctx)
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, workspaceID, w.workflow.ID, w.executionID, w.enableEvents)

	log.Info().Msgf("Executing node %s", nodeID)

	items, err := payload.ToItems()
	if err != nil {
		return ExecutionResult{}, err
	}

	inputID := fmt.Sprintf(InputHandleFormat, nodeID, 0)
	outputID := fmt.Sprintf(OutputHandleFormat, nodeID, 0)

	// this is trigger node spesific
	if w.enableEvents {
		err = w.PublishNodeExecutionStartedEvent(ctx, nodeID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish node execution started event")
		}

		itemsByInputID := map[string]domain.NodeItems{
			inputID: {
				FromNodeID: nodeID,
				Items:      items,
			},
		}

		itemsByOutputID := map[string]domain.NodeItems{
			outputID: {
				FromNodeID: nodeID,
				Items:      items,
			},
		}

		err = w.PublishNodeExecutionCompletedEvent(ctx, nodeID, itemsByInputID, itemsByOutputID, 0)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish node execution completed event")
		}
	}

	trigger, exists := w.workflow.GetTriggerByID(nodeID)
	if !exists {
		return ExecutionResult{}, fmt.Errorf("node %s not found in workflow", nodeID)
	}

	integrationType := domain.IntegrationType(trigger.Type)
	integrationActionType := domain.IntegrationActionType(trigger.EventType)

	// that will not see more than one output items
	w.AddUsageEntry(AddUsageEntryParams{
		SourceNodePayloadByInputID: SourceNodePayloadByInputID{
			inputID: {
				SourceNodeID: nodeID,
				Payload:      payload,
			},
		},
		IntegrationOutput: domain.IntegrationOutput{
			ResultJSONByOutputID: []domain.Payload{payload},
		},
		NodeID:                 nodeID,
		NodeExecutionStartedAt: time.Now(),
		NodeExecutionEndedAt:   time.Now(),
		IntegrationType:        integrationType,
		IntegrationActionType:  integrationActionType,
	})

	if !w.IsTestingWorkflow {
		w.AddNodeExecutionEntry(domain.NodeExecutionEntry{
			NodeID:    nodeID,
			EventType: domain.NodeExecuted,
			Timestamp: time.Now().UnixNano(),
			ItemsByInputID: map[string]domain.NodeItems{
				inputID: {
					FromNodeID: nodeID,
					Items:      items,
				},
			},
			ItemsByOutputID: map[string]domain.NodeItems{
				outputID: {
					FromNodeID: nodeID,
					Items:      items,
				},
			},
		})
	}

	eventName := outputID

	nodes := w.nodesByEventName[eventName]

	for _, node := range nodes {
		w.AddTaskForDownstreamNode(ctx, AddTaskForDownstreamNodeParams{
			FromNodeID: nodeID,
			Node:       node,
			OutputID:   eventName,
			Payload:    payload,
		})
	}

	executionCount := 0

	for len(w.executionQueue) > 0 {
		if ctx.Err() != nil {
			return ExecutionResult{}, ctx.Err()
		}

		log.Info().Msgf("Execution stack: %v", len(w.executionQueue))

		execution := w.executionQueue[0]
		w.executionQueue = w.executionQueue[1:]

		executionCount++

		err := w.ExecuteNode(ctx, execution, int64(executionCount))
		if err != nil {
			log.Error().Err(err).Msg("Error executing node")

			errPublish := w.PublishNodeFailedEvent(ctx, execution, err)
			if errPublish != nil {
				log.Error().Err(errPublish).Str("workflow_id", w.workflow.ID).Msg("executor: failed to publish node failed event")
			}

			err = w.AddFailedNodeExecutionEntry(execution, err)
			if err != nil {
				log.Error().Err(err).Str("workflow_id", w.workflow.ID).Msg("executor: failed to add failed node execution entry")
			}
		}

		if w.executionCountByNodeID[execution.NodeID] > MaxExecutionCount {
			log.Error().Msgf("node %s executed more than %d times", execution.NodeID, MaxExecutionCount)
			break
		}

		if len(w.executionQueue) == 0 && len(w.waitingExecutionTasks) > 0 {
			for _, task := range w.waitingExecutionTasks {
				w.AddExecutionTask(NodeExecutionTask{
					NodeID:           task.NodeID,
					PayloadByInputID: task.MergePayloadsByInputID(),
				})
			}

			w.waitingExecutionTasks = []WaitingExecutionTask{}
		}
	}

	// Convert domain types to flowbaker types using mappers
	nodeExecutions := mappers.DomainNodeExecutionsToFlowbaker(w.nodeExecutions)
	historyEntries := mappers.DomainNodeExecutionEntriesToFlowbaker(w.historyEntries)

	if executionContext, ok := domain.GetWorkflowExecutionContext(ctx); ok && executionContext != nil {
		agentHistoryEntries := mappers.DomainNodeExecutionEntriesToFlowbaker(executionContext.AgentNodeExecutions)
		historyEntries = append(historyEntries, agentHistoryEntries...)
	}

	completeParams := &flowbaker.CompleteExecutionRequest{
		ExecutionID:        w.executionID,
		WorkspaceID:        w.workflow.WorkspaceID,
		WorkflowID:         w.workflow.ID,
		TriggerNodeID:      nodeID,
		StartedAt:          w.WorkflowExecutionStartedAt,
		EndedAt:            time.Now(),
		NodeExecutions:     nodeExecutions,
		HistoryEntries:     historyEntries,
		IsTestingWorkflow:  w.IsTestingWorkflow,
	}

	err = w.client.CompleteWorkflowExecution(ctx, completeParams)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send complete workflow execution request")
	}

	err = w.PublishWorkflowExecutionCompletedEvent(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to publish workflow execution completed event")
	}

	log.Info().Msg("Execution stack is empty, workflow is finished")

	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return ExecutionResult{}, errors.New("workflow execution context not found")
	}

	return ExecutionResult{
		Payload:    executionContext.ResponsePayload,
		Headers:    executionContext.ResponseHeaders,
		StatusCode: executionContext.ResponseStatusCode,
	}, nil
}

func (w *WorkflowExecutor) ExecuteNode(ctx context.Context, execution NodeExecutionTask, executionOrder int64) error {
	log.Debug().Msgf("Executing node %s", execution.NodeID)

	nodeExecutionStartedAt := time.Now()

	if w.enableEvents {
		err := w.PublishNodeExecutionStartedEvent(ctx, execution.NodeID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish node execution started event")
		}
	}

	node, exists := w.workflow.GetActionNodeByID(execution.NodeID)
	if !exists {
		return fmt.Errorf("node %s not found in workflow", execution.NodeID)
	}

	// Skip non-executable nodes (like agent items)
	// Agent items have UsageContext other than "workflow"
	if node.UsageContext != "" && node.UsageContext != "workflow" {
		log.Debug().Msgf("Skipping agent item node %s with usage context %s", execution.NodeID, node.UsageContext)
		w.MarkNodeAsExecuted(node.ID)
		return nil
	}

	integrationCreator, err := w.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: node.NodeType,
	})
	if err != nil {
		return err
	}

	credentialID, exists := node.IntegrationSettings["credential_id"]
	if !exists {
		credentialID = ""
	}

	credentialIDString, ok := credentialID.(string)
	if !ok {
		return fmt.Errorf("credential_id is not a string")
	}

	integrationExecutor, err := integrationCreator.CreateIntegration(ctx, domain.CreateIntegrationParams{
		WorkspaceID:  w.workflow.WorkspaceID,
		CredentialID: credentialIDString,
	})
	if err != nil {
		return err
	}

	w.mutex.Lock()
	executionCount := w.executionCountByNodeID[node.ID]
	w.executionCountByNodeID[node.ID] = executionCount + 1
	w.mutex.Unlock()

	payloadByInputID := execution.PayloadByInputID.ToPayloadByInputID()

	output, err := integrationExecutor.Execute(ctx, domain.IntegrationInput{
		NodeID:           node.ID,
		PayloadByInputID: payloadByInputID,
		Workflow:         &w.workflow,
		IntegrationParams: domain.IntegrationParams{
			Settings: node.IntegrationSettings,
		},
		ActionType: node.ActionType,
	})
	if err != nil {
		return err
	}

	nodeExecutionEndedAt := time.Now()

	w.AddUsageEntry(AddUsageEntryParams{
		SourceNodePayloadByInputID: execution.PayloadByInputID,
		IntegrationOutput:          output,
		NodeID:                     node.ID,
		ExecutionOrder:             int64(executionOrder),
		NodeExecutionStartedAt:     nodeExecutionStartedAt,
		NodeExecutionEndedAt:       nodeExecutionEndedAt,
		IntegrationType:            domain.IntegrationType(node.NodeType),
		IntegrationActionType:      node.ActionType,
	})

	executedNode := node

	for outputIndex, payload := range output.ResultJSONByOutputID {
		outputID := fmt.Sprintf(OutputHandleFormat, executedNode.ID, outputIndex)

		nodes, ok := w.nodesByEventName[outputID]
		if !ok {
			log.Info().Msgf("No nodes registered for event %d", outputIndex)

			continue
		}

		for _, node := range nodes {
			err := w.AddTaskForDownstreamNode(ctx, AddTaskForDownstreamNodeParams{
				FromNodeID: executedNode.ID,
				Node:       node,
				OutputID:   outputID,
				Payload:    payload,
			})
			if err != nil {
				return err
			}
		}
	}

	w.MarkNodeAsExecuted(node.ID)

	itemsByOutputID := map[string]domain.NodeItems{}

	for outputIndex, payload := range output.ResultJSONByOutputID {
		items, err := payload.ToItems()
		if err != nil {
			return err
		}

		outputID := fmt.Sprintf(OutputHandleFormat, node.ID, outputIndex)

		itemsByOutputID[outputID] = domain.NodeItems{
			FromNodeID: node.ID,
			Items:      items,
		}

	}

	itemsByInputID := map[string]domain.NodeItems{}

	for inputID, payload := range execution.PayloadByInputID {
		items, err := payload.Payload.ToItems()
		if err != nil {
			return err
		}

		itemsByInputID[inputID] = domain.NodeItems{
			FromNodeID: payload.SourceNodeID,
			Items:      items,
		}
	}

	if w.enableEvents {
		err = w.PublishNodeExecutionCompletedEvent(ctx, executedNode.ID, itemsByInputID, itemsByOutputID, executionOrder)
		if err != nil {
			log.Error().Err(err).Msg("Failed to publish node execution completed event")
		}
	}

	if !w.IsTestingWorkflow {
		w.AddNodeExecutionEntry(domain.NodeExecutionEntry{
			NodeID:          node.ID,
			ItemsByInputID:  itemsByInputID,
			ItemsByOutputID: itemsByOutputID,
			EventType:       domain.NodeExecuted,
			Timestamp:       time.Now().UnixNano(),
			ExecutionOrder:  int(executionOrder),
		})
	}

	return nil
}

type AddTaskForDownstreamNodeParams struct {
	FromNodeID string
	Node       domain.WorkflowNode
	OutputID   string
	Payload    domain.Payload
}

// Check if node is currently waiting for this event
// If it is fill the payload
// Check if all events are received
// If they are add the task to the execution stack
// If node is not waiting for this event
// Check if next node has multiple inputs
// If it does add the task to the waiting execution tasks
// If it doesn't add the task to the execution stack
func (w *WorkflowExecutor) AddTaskForDownstreamNode(ctx context.Context, p AddTaskForDownstreamNodeParams) error {
	node := p.Node
	payload := p.Payload
	outputID := p.OutputID

	matchingInputID := ""

	for _, input := range node.Inputs {
		for _, subscribedOutputID := range input.SubscribedEvents {
			if subscribedOutputID == outputID {
				matchingInputID = input.InputID
			}
		}
	}

	if matchingInputID == "" {
		return fmt.Errorf("inputID not found for subscribed outputID %s", outputID)
	}

	waitingTask, exists := w.GetWaitingExecutionTask(p.Node.ID)
	if exists {
		waitingTask.AddPayload(p.FromNodeID, matchingInputID, outputID, payload)

		canExecute := w.shouldExecuteWaitingTask(ctx, waitingTask, node)

		if canExecute {
			payloadsByInputID := map[string]SourceNodePayload{}

			for inputID, p := range waitingTask.ReceivedPayloads {
				for _, payload := range p {
					payloadsByInputID[inputID] = payload
				}
			}

			w.AddExecutionTask(NodeExecutionTask{
				NodeID:           node.ID,
				PayloadByInputID: payloadsByInputID,
			})

			w.mutex.Lock()
			newWaitingTasks := []WaitingExecutionTask{}

			for _, task := range w.waitingExecutionTasks {
				if task.NodeID != node.ID {
					newWaitingTasks = append(newWaitingTasks, task)
				}
			}

			w.waitingExecutionTasks = newWaitingTasks
			w.mutex.Unlock()
		}

		return nil
	}

	isNextNodeHasMultipleInputs := len(node.Inputs) > 1 && node.NodeType != domain.IntegrationType_AIAgent

	if isNextNodeHasMultipleInputs {
		w.AddWaitingExecutionTask(WaitingExecutionTask{
			NodeID:  node.ID,
			Payload: payload,
			ReceivedPayloads: map[string]map[string]SourceNodePayload{
				matchingInputID: {
					outputID: {
						SourceNodeID: p.FromNodeID,
						Payload:      payload,
					},
				},
			},
			mutex: &sync.Mutex{},
		})

		return nil
	}

	fmt.Println("Adding task to execution stack", node.ID)

	// Node has only one input
	payloadByInputID := map[string]SourceNodePayload{}

	for _, input := range node.Inputs {
		sourceNodePayload := SourceNodePayload{
			SourceNodeID: p.FromNodeID,
			Payload:      payload,
		}

		payloadByInputID[input.InputID] = sourceNodePayload

		break
	}

	w.AddExecutionTask(NodeExecutionTask{
		NodeID:           node.ID,
		PayloadByInputID: payloadByInputID,
	})

	return nil
}

func (w *WorkflowExecutor) shouldExecuteWaitingTask(ctx context.Context, waitingTask WaitingExecutionTask, node domain.WorkflowNode) bool {
	waitingTask.mutex.Lock()
	defer waitingTask.mutex.Unlock()

	eventStatuses := []bool{}

	for _, input := range node.Inputs {
		payloadsForInput, exists := waitingTask.ReceivedPayloads[input.InputID]
		if !exists {
			return false
		}

		for _, subscribedOutputID := range input.SubscribedEvents {
			_, exists := payloadsForInput[subscribedOutputID]
			if !exists {
				log.Info().Msgf("Missing payload for input %s, output %s", input.InputID, subscribedOutputID)

				continue
			}

			eventStatuses = append(eventStatuses, true)
		}
	}

	isAllEventsReceived := false

	for _, status := range eventStatuses {
		if status {
			isAllEventsReceived = true

			break
		}
	}

	return isAllEventsReceived
}

func (w *WorkflowExecutor) GetWaitingExecutionTask(nodeID string) (WaitingExecutionTask, bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, task := range w.waitingExecutionTasks {
		if task.NodeID == nodeID {
			return task, true
		}
	}

	return WaitingExecutionTask{}, false
}

func (w *WorkflowExecutor) AddExecutionTask(execution NodeExecutionTask) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	log.Info().Msgf("Adding execution task for node %s", execution.NodeID)

	unshift := true

	if unshift {
		w.executionQueue = append([]NodeExecutionTask{execution}, w.executionQueue...)
		return
	}

	w.executionQueue = append(w.executionQueue, execution)
}

func (w *WorkflowExecutor) MarkNodeAsExecuted(nodeID string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.executedNodes[nodeID] = struct{}{}
}

func (w *WorkflowExecutor) IsNodeExecuted(nodeID string) bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	_, exists := w.executedNodes[nodeID]

	return exists
}

func (w *WorkflowExecutor) AddWaitingExecutionTask(task WaitingExecutionTask) {
	w.mutex.Lock()

	defer w.mutex.Unlock()

	log.Info().Msgf("Adding waiting task for node %s", task.NodeID)

	w.waitingExecutionTasks = append(w.waitingExecutionTasks, task)
}

func (w *WorkflowExecutor) PublishNodeFailedEvent(ctx context.Context, execution NodeExecutionTask, err error) error {
	if !w.enableEvents {
		return nil
	}

	itemsByInputID := map[string]domain.NodeItems{}

	for inputID, payload := range execution.PayloadByInputID {
		items, err := payload.Payload.ToItems()
		if err != nil {
			return err
		}

		itemsByInputID[inputID] = domain.NodeItems{
			FromNodeID: payload.SourceNodeID,
			Items:      items,
		}
	}

	event := domain.NodeFailedEvent{
		WorkflowID:          w.workflow.ID,
		WorkflowExecutionID: w.executionID,
		Timestamp:           time.Now().UnixNano(),
		NodeID:              execution.NodeID,
		Error:               err.Error(),
		ItemsByInputID:      itemsByInputID,
		ItemsByOutputID:     map[string]domain.NodeItems{},
	}

	err = w.orderedEventPublisher.PublishEvent(ctx, &event)
	if err != nil {
		return err
	}

	return nil
}

func (w *WorkflowExecutor) AddNodeExecutionEntry(execution domain.NodeExecutionEntry) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.historyEntries = append(w.historyEntries, execution)
}

func (w *WorkflowExecutor) AddFailedNodeExecutionEntry(execution NodeExecutionTask, err error) error {
	itemsByInputID := map[string]domain.NodeItems{}

	for inputID, payload := range execution.PayloadByInputID {
		items, err := payload.Payload.ToItems()
		if err != nil {
			return err
		}

		itemsByInputID[inputID] = domain.NodeItems{
			FromNodeID: payload.SourceNodeID,
			Items:      items,
		}
	}

	if !w.IsTestingWorkflow {
		w.AddNodeExecutionEntry(domain.NodeExecutionEntry{
			NodeID:          execution.NodeID,
			ItemsByInputID:  itemsByInputID,
			ItemsByOutputID: map[string]domain.NodeItems{},
			EventType:       domain.NodeFailed,
			Error:           err.Error(),
			Timestamp:       time.Now().UnixNano(),
		})
	}

	return nil
}

func (w *WorkflowExecutor) PublishNodeExecutionStartedEvent(ctx context.Context, nodeID string) error {
	event := &domain.NodeExecutionStartedEvent{
		WorkflowID:          w.workflow.ID,
		WorkflowExecutionID: w.executionID,
		NodeID:              nodeID,
		Timestamp:           time.Now().UnixNano(),
	}

	err := w.orderedEventPublisher.PublishEvent(ctx, event)
	if err != nil {
		return err
	}

	return nil
}

func (w *WorkflowExecutor) PublishNodeExecutionCompletedEvent(ctx context.Context, nodeID string, itemsByInputID map[string]domain.NodeItems, itemsByOutputID map[string]domain.NodeItems, executionOrder int64) error {
	event := &domain.NodeExecutedEvent{
		WorkflowID:          w.workflow.ID,
		WorkflowExecutionID: w.executionID,
		NodeID:              nodeID,
		Timestamp:           time.Now().UnixNano(),
		ItemsByInputID:      itemsByInputID,
		ItemsByOutputID:     itemsByOutputID,
		ExecutionOrder:      int(executionOrder),
	}

	err := w.orderedEventPublisher.PublishEvent(ctx, event)
	if err != nil {
		return err
	}

	return nil
}

func (w *WorkflowExecutor) PublishWorkflowExecutionCompletedEvent(ctx context.Context) error {
	if !w.enableEvents {
		return nil
	}

	event := &domain.WorkflowExecutionCompletedEvent{
		WorkflowID:          w.workflow.ID,
		WorkflowExecutionID: w.executionID,
		Timestamp:           time.Now().UnixNano(),
	}

	return w.orderedEventPublisher.PublishEvent(ctx, event)
}

type AddUsageEntryParams struct {
	SourceNodePayloadByInputID SourceNodePayloadByInputID
	IntegrationOutput          domain.IntegrationOutput
	NodeID                     string
	ExecutionOrder             int64
	NodeExecutionStartedAt     time.Time
	NodeExecutionEndedAt       time.Time
	IntegrationType            domain.IntegrationType
	IntegrationActionType      domain.IntegrationActionType
}

func (w *WorkflowExecutor) AddUsageEntry(p AddUsageEntryParams) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	inputItemsCount := domain.InputItemsCount{}
	inputItemsSizeInBytes := domain.InputItemsSizeInBytes{}

	for inputID, payload := range p.SourceNodePayloadByInputID {
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

	for outputID, payload := range p.IntegrationOutput.ResultJSONByOutputID {
		var items []domain.Item
		err := json.Unmarshal(payload, &items)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to parse JSON for output %d", outputID)
			continue
		}

		outputItemsCount[int64(outputID)] = int64(len(items))
		outputItemsSizeInBytes[int64(outputID)] = int64(len(payload))
	}

	w.nodeExecutions = append(w.nodeExecutions, domain.NodeExecution{
		ID:                     xid.New().String(),
		NodeID:                 p.NodeID,
		IntegrationType:        domain.IntegrationType(p.IntegrationType),
		IntegrationActionType:  domain.IntegrationActionType(p.IntegrationActionType),
		StartedAt:              p.NodeExecutionStartedAt,
		EndedAt:                p.NodeExecutionEndedAt,
		ExecutionOrder:         int64(p.ExecutionOrder),
		InputItemsCount:        inputItemsCount,
		InputItemsSizeInBytes:  inputItemsSizeInBytes,
		OutputItemsCount:       outputItemsCount,
		OutputItemsSizeInBytes: outputItemsSizeInBytes,
	})
}

func ConvertPayloadsToItems(payloads []domain.Payload) []domain.Item {
	allItems := []domain.Item{}

	for _, payload := range payloads {
		items, err := payload.ToItems()
		if err != nil {
			return nil
		}

		allItems = append(allItems, items...)
	}

	return allItems
}

