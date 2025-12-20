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

	"github.com/rs/zerolog/log"
)

type SourceNodePayload struct {
	SourceNodeID string
	Payload      domain.Payload
}

type SourceNodePayloadByInputID map[string]SourceNodePayload

type NodeExecutionResult struct {
	Output                domain.IntegrationOutput
	IntegrationType       domain.IntegrationType
	IntegrationActionType domain.IntegrationActionType
}

func (p SourceNodePayloadByInputID) ToPayloadByInputID() map[string]domain.Payload {
	payloadByInputID := map[string]domain.Payload{}

	for inputID, payload := range p {
		payloadByInputID[inputID] = payload.Payload
	}

	return payloadByInputID
}

// ToItemsByInputID converts payloads to items by input ID
func (p SourceNodePayloadByInputID) ToItemsByInputID() map[string]domain.NodeItems {
	itemsByInputID := map[string]domain.NodeItems{}

	for inputID, payload := range p {
		items, err := payload.Payload.ToItems()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to convert payload to items for input %s", inputID)
			continue
		}

		itemsByInputID[inputID] = domain.NodeItems{
			FromNodeID: payload.SourceNodeID,
			Items:      items,
		}
	}

	return itemsByInputID
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

	nodesByEventName       map[string][]domain.WorkflowNode
	executionCountByNodeID map[string]int

	mutex sync.Mutex

	integrationSelector domain.IntegrationSelector
	enableEvents        bool

	IsTestingWorkflow bool

	WorkflowExecutionStartedAt time.Time

	client   flowbaker.ClientInterface
	observer domain.ExecutionObserver

	historyRecorder *HistoryRecorder
	usageCollector  *UsageCollector
}

type WorkflowExecutorDeps struct {
	ExecutionID           string
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

	// Initialize execution observer
	observer := NewExecutionObserver()

	// Create handlers
	historyRecorder := NewHistoryRecorder()
	usageCollector := NewUsageCollector()
	eventBroadcaster := NewEventBroadcaster(
		deps.OrderedEventPublisher,
		deps.EnableEvents,
		deps.Workflow.ID,
		deps.ExecutionID,
	)

	// Subscribe handlers to observer
	observer.Subscribe(historyRecorder)
	observer.Subscribe(usageCollector)
	observer.Subscribe(eventBroadcaster)

	return WorkflowExecutor{
		executionID:                deps.ExecutionID,
		workflow:                   deps.Workflow,
		waitingExecutionTasks:      []WaitingExecutionTask{},
		executionQueue:             []NodeExecutionTask{},
		executedNodes:              map[string]struct{}{},
		nodesByEventName:           nodesByEventName,
		integrationSelector:        deps.Selector,
		executionCountByNodeID:     map[string]int{},
		enableEvents:               deps.EnableEvents,
		IsTestingWorkflow:          deps.IsTestingWorkflow,
		WorkflowExecutionStartedAt: time.Now(),
		client:                     deps.ExecutorClient,
		observer:                   observer,
		historyRecorder:            historyRecorder,
		usageCollector:             usageCollector,
	}
}

const (
	InputHandleFormat  = "input-%s-%d"
	OutputHandleFormat = "output-%s-%d"

	MaxExecutionCount = 1000
)

func (w *WorkflowExecutor) Execute(ctx context.Context, nodeID string, payload domain.Payload) (ExecutionResult, error) {
	workspaceID := w.workflow.WorkspaceID

	ctx = domain.NewContextWithEventOrder(ctx)
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, domain.NewContextWithWorkflowExecutionContextParams{
		WorkspaceID:         workspaceID,
		WorkflowID:          w.workflow.ID,
		WorkflowExecutionID: w.executionID,
		EnableEvents:        w.enableEvents,
		Observer:            w.observer,
	})

	log.Info().Msgf("Executing workflow triggered by node %s", nodeID)

	// Queue trigger node as first execution task
	inputID := fmt.Sprintf(InputHandleFormat, nodeID, 0)
	w.AddExecutionTask(NodeExecutionTask{
		NodeID: nodeID,
		PayloadByInputID: SourceNodePayloadByInputID{
			inputID: {
				SourceNodeID: nodeID,
				Payload:      payload,
			},
		},
	})

	executionCount := 0

	for len(w.executionQueue) > 0 {
		if ctx.Err() != nil {
			return ExecutionResult{}, ctx.Err()
		}

		log.Info().Msgf("Execution stack: %v", len(w.executionQueue))

		execution := w.executionQueue[0]
		w.executionQueue = w.executionQueue[1:]

		executionCount++

		_, err := w.ExecuteNode(ctx, ExecuteNodeParams{
			Task:           execution,
			ExecutionOrder: int64(executionCount),
			Propagate:      true,
		})
		if err != nil {
			log.Error().Err(err).Msg("Error executing node")

			// Notify observers about node failure
			errNotify := w.observer.Notify(ctx, NodeExecutionFailedEvent{
				NodeID:         execution.NodeID,
				ItemsByInputID: execution.PayloadByInputID.ToItemsByInputID(),
				Error:          err,
				Timestamp:      time.Now(),
			})
			if errNotify != nil {
				log.Error().Err(errNotify).Str("workflow_id", w.workflow.ID).Msg("executor: failed to notify node failed event")
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

	executionResults := w.historyRecorder.GetHistoryEntries()

	// Convert domain types to flowbaker types using mappers
	nodeExecutions := mappers.DomainNodeExecutionsToFlowbaker(w.usageCollector.GetNodeExecutions())
	historyEntries := mappers.DomainNodeExecutionEntriesToFlowbaker(executionResults)

	completeParams := &flowbaker.CompleteExecutionRequest{
		ExecutionID:       w.executionID,
		WorkspaceID:       w.workflow.WorkspaceID,
		WorkflowID:        w.workflow.ID,
		TriggerNodeID:     nodeID,
		StartedAt:         w.WorkflowExecutionStartedAt,
		EndedAt:           time.Now(),
		NodeExecutions:    nodeExecutions,
		HistoryEntries:    historyEntries,
		IsTestingWorkflow: w.IsTestingWorkflow,
	}

	if err := w.client.CompleteWorkflowExecution(ctx, completeParams); err != nil {
		log.Error().Err(err).Msg("Failed to send complete workflow execution request")
	}

	if err := w.observer.Notify(ctx, WorkflowExecutionCompletedEvent{
		Timestamp: time.Now(),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to notify workflow execution completed")
	}

	log.Info().Msg("Execution stack is empty, workflow is finished")

	executionContext, ok := domain.GetWorkflowExecutionContext(ctx)
	if !ok {
		return ExecutionResult{}, errors.New("workflow execution context not found")
	}

	return ExecutionResult{
		Payload:              executionContext.ResponsePayload,
		Headers:              executionContext.ResponseHeaders,
		StatusCode:           executionContext.ResponseStatusCode,
		NodeExecutionResults: executionResults,
	}, nil
}

type ExecutionNodeType string

const (
	ExecutionNodeTypeAction  ExecutionNodeType = "action"
	ExecutionNodeTypeTrigger ExecutionNodeType = "trigger"
)

type ExecuteNodeParams struct {
	Task           NodeExecutionTask
	ExecutionOrder int64
	Propagate      bool
}

type ExecuteNodeResult struct {
	ItemsByOutputID map[string]domain.NodeItems
}

func (w *WorkflowExecutor) ExecuteNode(ctx context.Context, p ExecuteNodeParams) (ExecuteNodeResult, error) {
	execution := p.Task
	executionOrder := p.ExecutionOrder
	propagate := p.Propagate

	nodeExecutionStartedAt := time.Now()

	err := w.observer.Notify(ctx, NodeExecutionStartedEvent{
		NodeID:    execution.NodeID,
		Timestamp: nodeExecutionStartedAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution started")
	}

	var result NodeExecutionResult
	var nodeID string

	if action, exists := w.workflow.GetActionNodeByID(execution.NodeID); exists {
		result, err = w.ExecuteActionNode(ctx, action, execution)
		if err != nil {
			result, err = w.HandleNodeExecutionError(HandleNodeExecutionErrorParams{
				Err:             err,
				IntegrationType: domain.IntegrationType(action.NodeType),
				ActionType:      domain.IntegrationActionType(action.ActionType),
				Settings:        action.Settings,
			})
			if err != nil {
				return ExecuteNodeResult{}, err
			}
		}

		nodeID = action.ID
	} else if trigger, exists := w.workflow.GetTriggerByID(execution.NodeID); exists {
		result, err = w.ExecuteTriggerNode(ctx, trigger, execution)
		if err != nil {
			result, err = w.HandleNodeExecutionError(HandleNodeExecutionErrorParams{
				Err:             err,
				IntegrationType: domain.IntegrationType(trigger.Type),
				ActionType:      domain.IntegrationActionType(trigger.EventType),
				Settings:        trigger.Settings,
			})
			if err != nil {
				return ExecuteNodeResult{}, err
			}
		}

		nodeID = trigger.ID
	} else {
		return ExecuteNodeResult{}, fmt.Errorf("node %s not found in workflow", execution.NodeID)
	}

	if err != nil {
		return ExecuteNodeResult{}, err
	}

	nodeExecutionEndedAt := time.Now()

	if propagate {
		for outputIndex, payload := range result.Output.ResultJSONByOutputID {
			outputID := fmt.Sprintf(OutputHandleFormat, nodeID, outputIndex)

			nodes, ok := w.nodesByEventName[outputID]
			if !ok {
				log.Info().Msgf("No nodes registered for event %d", outputIndex)
				continue
			}

			if !payload.IsEmpty() {
				for _, node := range nodes {
					err := w.AddTaskForDownstreamNode(ctx, AddTaskForDownstreamNodeParams{
						FromNodeID: nodeID,
						Node:       node,
						OutputID:   outputID,
						Payload:    payload,
					})
					if err != nil {
						return ExecuteNodeResult{}, err
					}
				}
			}
		}
	}

	w.MarkNodeAsExecuted(nodeID)

	itemsByOutputID := result.Output.ToItemsByOutputID(nodeID)
	itemsByInputID := execution.PayloadByInputID.ToItemsByInputID()

	err = w.observer.Notify(ctx, NodeExecutionCompletedEvent{
		NodeID:                     nodeID,
		SourceNodePayloadByInputID: execution.PayloadByInputID,
		IntegrationOutput:          result.Output,
		ItemsByInputID:             itemsByInputID,
		ItemsByOutputID:            itemsByOutputID,
		ExecutionOrder:             executionOrder,
		IntegrationType:            result.IntegrationType,
		IntegrationActionType:      result.IntegrationActionType,
		StartedAt:                  nodeExecutionStartedAt,
		EndedAt:                    nodeExecutionEndedAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution completed")
	}

	return ExecuteNodeResult{
		ItemsByOutputID: itemsByOutputID,
	}, nil
}

func (w *WorkflowExecutor) ExecuteTriggerNode(ctx context.Context, trigger domain.WorkflowTrigger, execution NodeExecutionTask) (NodeExecutionResult, error) {

	inputID := fmt.Sprintf(InputHandleFormat, trigger.ID, 0)
	inputPayload, exists := execution.PayloadByInputID[inputID]
	if !exists {
		err := fmt.Errorf("trigger input payload not found")
		return NodeExecutionResult{}, err
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ResultJSONByOutputID: []domain.Payload{inputPayload.Payload},
		},
		IntegrationType:       domain.IntegrationType(trigger.Type),
		IntegrationActionType: domain.IntegrationActionType(trigger.EventType),
	}, nil
}

func (w *WorkflowExecutor) ExecuteActionNode(ctx context.Context, node domain.WorkflowNode, execution NodeExecutionTask) (NodeExecutionResult, error) {
	// Skip non-executable nodes (like agent items)
	// Agent items have UsageContext other than "workflow"
	if node.UsageContext != "" && node.UsageContext != "workflow" {
		log.Debug().Msgf("Skipping agent item node %s with usage context %s", execution.NodeID, node.UsageContext)
		return NodeExecutionResult{
			Output: domain.IntegrationOutput{
				ResultJSONByOutputID: []domain.Payload{},
			},
			IntegrationType:       domain.IntegrationType(node.NodeType),
			IntegrationActionType: node.ActionType,
		}, nil
	}

	integrationCreator, err := w.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: node.NodeType,
	})
	if err != nil {
		return NodeExecutionResult{}, err
	}

	credentialID, exists := node.IntegrationSettings["credential_id"]
	if !exists {
		credentialID = ""
	}

	credentialIDString, ok := credentialID.(string)
	if !ok {
		err := fmt.Errorf("credential_id is not a string")
		return NodeExecutionResult{}, err
	}

	integrationExecutor, err := integrationCreator.CreateIntegration(ctx, domain.CreateIntegrationParams{
		WorkspaceID:  w.workflow.WorkspaceID,
		CredentialID: credentialIDString,
	})
	if err != nil {
		return NodeExecutionResult{}, err
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

		return NodeExecutionResult{}, err
	}

	return NodeExecutionResult{
		Output:                output,
		IntegrationType:       domain.IntegrationType(node.NodeType),
		IntegrationActionType: node.ActionType,
	}, nil
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

type ErrorItem struct {
	ErrorMessage string `json:"error_message"`
}

type HandleNodeExecutionErrorParams struct {
	Err             error
	IntegrationType domain.IntegrationType
	ActionType      domain.IntegrationActionType
	Settings        domain.Settings
}

func (w *WorkflowExecutor) HandleNodeExecutionError(p HandleNodeExecutionErrorParams) (NodeExecutionResult, error) {
	if !p.Settings.ReturnErrorAsItem {
		return NodeExecutionResult{}, p.Err
	}

	errorItem := ErrorItem{
		ErrorMessage: p.Err.Error(),
	}

	errorItems := []ErrorItem{errorItem}
	errorPayload, marshalErr := json.Marshal(errorItems)
	if marshalErr != nil {
		return NodeExecutionResult{}, fmt.Errorf("failed to marshal error as item: %w", marshalErr)
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ResultJSONByOutputID: []domain.Payload{errorPayload},
		},
		IntegrationType:       p.IntegrationType,
		IntegrationActionType: p.ActionType,
	}, nil
}
