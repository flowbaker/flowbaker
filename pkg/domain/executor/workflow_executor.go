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

	userID *string // Optional, Only filled in testing workflows

	integrationSelector domain.IntegrationSelector
	enableEvents        bool
	enableStreaming     bool

	IsTestingWorkflow bool

	WorkflowExecutionStartedAt time.Time

	client   flowbaker.ClientInterface
	observer *executionObserver

	historyRecorder *HistoryRecorder
	usageCollector  *UsageCollector

	streamEventPublisher domain.StreamEventPublisher
}

type WorkflowExecutorDeps struct {
	ExecutionID           string
	UserID                *string
	Workflow              domain.Workflow
	Selector              domain.IntegrationSelector
	EnableEvents          bool
	EnableStreaming       bool
	IsTestingWorkflow     bool
	ExecutorClient        flowbaker.ClientInterface
	OrderedEventPublisher domain.EventPublisher
}

func NewWorkflowExecutor(deps WorkflowExecutorDeps) (WorkflowExecutor, error) {
	nodesByEventName := map[string][]domain.WorkflowNode{}

	actionNodes := deps.Workflow.GetActionNodes()

	for _, node := range actionNodes {
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

	streamEventPublisher, err := domain.NewStreamEventPublisher(context.TODO(), deps.Workflow.WorkspaceID, deps.ExecutorClient)
	if err != nil {
		return WorkflowExecutor{}, fmt.Errorf("failed to create stream event publisher: %w", err)
	}

	streamEventPublisher.Initialize()

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
	streamBroadcaster := NewStreamEventBroadcaster(streamEventPublisher)

	// Subscribe handlers to observer
	observer.Subscribe(historyRecorder)
	observer.Subscribe(usageCollector)
	observer.Subscribe(eventBroadcaster)
	observer.SubscribeStream(streamBroadcaster)

	return WorkflowExecutor{
		executionID:                deps.ExecutionID,
		userID:                     deps.UserID,
		workflow:                   deps.Workflow,
		waitingExecutionTasks:      []WaitingExecutionTask{},
		executionQueue:             []NodeExecutionTask{},
		executedNodes:              map[string]struct{}{},
		nodesByEventName:           nodesByEventName,
		integrationSelector:        deps.Selector,
		executionCountByNodeID:     map[string]int{},
		enableEvents:               deps.EnableEvents,
		enableStreaming:            deps.EnableStreaming,
		IsTestingWorkflow:          deps.IsTestingWorkflow,
		WorkflowExecutionStartedAt: time.Now(),
		client:                     deps.ExecutorClient,
		observer:                   observer,
		historyRecorder:            historyRecorder,
		usageCollector:             usageCollector,
		streamEventPublisher:       streamEventPublisher,
	}, nil
}

const (
	InputHandleFormat  = "input-%s-%d"
	OutputHandleFormat = "output-%s-%d"

	DefaultNodeExecutionLimit = 1000
)

func (w *WorkflowExecutor) Execute(ctx context.Context, nodeID string, payload domain.Payload) (ExecutionResult, error) {
	workspaceID := w.workflow.WorkspaceID
	defer w.streamEventPublisher.Close()

	isErrorTrigger := w.IsErrorTrigger(nodeID)

	triggerNode, exists := w.workflow.GetNodeByID(nodeID)
	if !exists {
		return ExecutionResult{}, fmt.Errorf("node %s not found in workflow", nodeID)
	}

	ctx = domain.NewContextWithEventOrder(ctx)
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, domain.NewContextWithWorkflowExecutionContextParams{
		UserID:              w.userID,
		InputPayload:        payload,
		WorkspaceID:         workspaceID,
		WorkflowID:          w.workflow.ID,
		WorkflowExecutionID: w.executionID,
		EnableEvents:        w.enableEvents,
		Observer:            w.observer,
		IsFromErrorTrigger:  isErrorTrigger,
		IsTesting:           w.IsTestingWorkflow,
		TriggerNode:         triggerNode,
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

			errNotify := w.observer.Notify(ctx, NodeExecutionFailedEvent{
				NodeID:         execution.NodeID,
				ItemsByInputID: execution.PayloadByInputID.ToItemsByInputID(),
				Error:          err,
				Timestamp:      time.Now(),
			})
			if errNotify != nil {
				log.Error().Err(errNotify).Str("workflow_id", w.workflow.ID).Msg("executor: failed to notify node failed event")
			}

			break
		}

		node, exists := w.workflow.GetNodeByID(execution.NodeID)
		if exists {
			limit := w.getNodeExecutionLimit(node)
			if w.executionCountByNodeID[execution.NodeID] >= limit {
				log.Error().Msgf("node %s executed more than %d times (limit reached)", execution.NodeID, limit)
				// return edilip edilmeyecegini tartis
				return ExecutionResult{}, fmt.Errorf("node %s executed more than %d times (limit reached)", execution.NodeID, limit)
			}
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

	var result NodeExecutionResult
	var nodeID string

	node, exists := w.workflow.GetNodeByID(execution.NodeID)
	if !exists {
		return ExecuteNodeResult{}, fmt.Errorf("node %s not found in workflow", execution.NodeID)
	}

	nodeID = node.ID

	w.mutex.Lock()
	executionCount := w.executionCountByNodeID[nodeID]
	w.executionCountByNodeID[nodeID] = executionCount + 1
	w.mutex.Unlock()

	nodeExecutionStartedAt := time.Now()

	err := w.observer.Notify(ctx, NodeExecutionStartedEvent{
		NodeID:    execution.NodeID,
		Timestamp: nodeExecutionStartedAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution started")
	}

	switch node.Type {
	case domain.NodeTypeAction:
		result, err = w.ExecuteActionNode(ctx, node, execution)
	case domain.NodeTypeTrigger:
		result, err = w.ExecuteTriggerNode(ctx, node, execution)
	default:
		return ExecuteNodeResult{}, fmt.Errorf("node type is invalid: %s", node.Type)
	}

	if err != nil {
		result, err = w.HandleNodeExecutionError(HandleNodeExecutionErrorParams{
			Err:  err,
			Node: node,
		})
		if err != nil {
			return ExecuteNodeResult{}, err
		}
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

func (w *WorkflowExecutor) ExecuteTriggerNode(ctx context.Context, node domain.WorkflowNode, execution NodeExecutionTask) (NodeExecutionResult, error) {

	inputID := fmt.Sprintf(InputHandleFormat, node.ID, 0)
	inputPayload, exists := execution.PayloadByInputID[inputID]
	if !exists {
		err := fmt.Errorf("trigger input payload not found")
		return NodeExecutionResult{}, err
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ResultJSONByOutputID: []domain.Payload{inputPayload.Payload},
		},
		IntegrationType:       domain.IntegrationType(node.Type),
		IntegrationActionType: domain.IntegrationActionType(node.TriggerNodeOpts.EventType),
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
			IntegrationType:       domain.IntegrationType(node.IntegrationType),
			IntegrationActionType: node.ActionNodeOpts.ActionType,
		}, nil
	}

	integrationCreator, err := w.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: node.IntegrationType,
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

	payloadByInputID := execution.PayloadByInputID.ToPayloadByInputID()

	output, err := integrationExecutor.Execute(ctx, domain.IntegrationInput{
		NodeID:           node.ID,
		PayloadByInputID: payloadByInputID,
		Workflow:         &w.workflow,
		IntegrationParams: domain.IntegrationParams{
			Settings: node.IntegrationSettings,
		},
		ActionType: node.ActionNodeOpts.ActionType,
	})
	if err != nil {

		return NodeExecutionResult{}, err
	}

	return NodeExecutionResult{
		Output:                output,
		IntegrationType:       domain.IntegrationType(node.IntegrationType),
		IntegrationActionType: node.ActionNodeOpts.ActionType,
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

	isNextNodeHasMultipleInputs := len(node.Inputs) > 1 && node.IntegrationType != domain.IntegrationType_AIAgent

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
	Err  error
	Node domain.WorkflowNode
}

func (w *WorkflowExecutor) HandleNodeExecutionError(p HandleNodeExecutionErrorParams) (NodeExecutionResult, error) {
	settings := p.Node.Settings

	if !settings.ReturnErrorAsItem {
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

	integrationType := domain.IntegrationType(p.Node.Type)

	actionType := ""

	switch p.Node.Type {
	case domain.NodeTypeAction:
		actionType = string(p.Node.ActionNodeOpts.ActionType)
	case domain.NodeTypeTrigger:
		actionType = string(p.Node.TriggerNodeOpts.EventType)
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ResultJSONByOutputID: []domain.Payload{errorPayload},
		},
		IntegrationType:       integrationType,
		IntegrationActionType: domain.IntegrationActionType(actionType),
	}, nil
}

func (w *WorkflowExecutor) IsErrorTrigger(nodeID string) bool {
	node, exists := w.workflow.GetNodeByID(nodeID)
	if !exists {
		return false
	}

	return node.Type == domain.NodeTypeTrigger && node.TriggerNodeOpts.EventType == "on_error"
}

func (w *WorkflowExecutor) getNodeExecutionLimit(node domain.WorkflowNode) int {
	if node.Settings.OverwriteExecutionLimit && node.Settings.ExecutionLimit > 0 {
		return node.Settings.ExecutionLimit
	}

	if w.workflow.Settings.NodeExecutionLimit > 0 {
		return w.workflow.Settings.NodeExecutionLimit
	}

	return DefaultNodeExecutionLimit
}
