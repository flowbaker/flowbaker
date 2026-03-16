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

type NodePayload struct {
	SourceNodeID string
	Payload      domain.Payload
}

type NodePayloadByInputIndex map[int]NodePayload

type NodeExecutionResult struct {
	Output                domain.IntegrationOutput
	IntegrationType       domain.IntegrationType
	IntegrationActionType domain.IntegrationActionType
}

func (p NodePayloadByInputIndex) ToPayloadByInputIndex() map[int]domain.Payload {
	payloadByInputIndex := map[int]domain.Payload{}

	for inputIndex, payload := range p {
		payloadByInputIndex[inputIndex] = payload.Payload
	}

	return payloadByInputIndex
}

// ToItemsByInputIndex converts payloads to items by input Index
func (p NodePayloadByInputIndex) ToItemsByInputIndex() map[int]domain.NodeItems {
	itemsByInputIndex := map[int]domain.NodeItems{}

	for inputIndex, nodePayload := range p {
		items, err := nodePayload.Payload.ToItems()
		if err != nil {
			log.Error().Err(err).Msgf("Failed to convert payload to items for input %d", inputIndex)
			continue
		}

		itemsByInputIndex[inputIndex] = domain.NodeItems{
			FromNodeID: nodePayload.SourceNodeID,
			Items:      items,
		}
	}

	return itemsByInputIndex
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

	nodesBySourceOutputKey map[sourceOutputKey][]domain.WorkflowNode
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

type sourceOutputKey struct {
	SourceNodeID string
	OutputIndex  int
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
	nodesBySourceOutputKey := map[sourceOutputKey][]domain.WorkflowNode{}

	actionNodes := deps.Workflow.GetActionNodes()

	for _, node := range actionNodes {
		for _, input := range node.Inputs {
			for _, subscribedOutput := range input.SubscribedOutputs {
				key := sourceOutputKey{
					SourceNodeID: subscribedOutput.NodeID,
					OutputIndex:  subscribedOutput.Index,
				}

				nodes, ok := nodesBySourceOutputKey[key]
				if !ok {
					nodes = []domain.WorkflowNode{}
				}

				nodes = append(nodes, node)

				nodesBySourceOutputKey[key] = nodes
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
		nodesBySourceOutputKey:     nodesBySourceOutputKey,
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

	if err := w.observer.Notify(ctx, WorkflowExecutionStartedEvent{
		Timestamp: time.Now(),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to notify workflow execution started")
	}

	log.Info().Msgf("Executing workflow triggered by node %s", nodeID)

	// Queue trigger node as first execution task
	w.AddExecutionTask(NodeExecutionTask{
		NodeID: nodeID,
		PayloadByInputIndex: map[int]NodePayload{
			0: {
				SourceNodeID: nodeID,
				Payload:      payload,
			}},
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

		node, exists := w.workflow.GetNodeByID(execution.NodeID)
		if exists {
			limit := w.getNodeExecutionLimit(node)
			if w.executionCountByNodeID[execution.NodeID] >= limit {
				log.Error().Msgf("node %s executed more than %d times (limit reached)", execution.NodeID, limit)
				break
			}
		}
		_, err := w.ExecuteNode(ctx, ExecuteNodeParams{
			Task:           execution,
			ExecutionOrder: int64(executionCount),
			Propagate:      true,
		})
		if err != nil {
			log.Error().Err(err).Msg("Error executing node")

			errNotify := w.observer.Notify(ctx, NodeExecutionFailedEvent{
				NodeID:            execution.NodeID,
				ItemsByInputIndex: execution.PayloadByInputIndex.ToItemsByInputIndex(),
				Error:             err,
				Timestamp:         time.Now(),
			})
			if errNotify != nil {
				log.Error().Err(errNotify).Str("workflow_id", w.workflow.ID).Msg("executor: failed to notify node failed event")
			}

			break
		}

		if len(w.executionQueue) == 0 && len(w.waitingExecutionTasks) > 0 {
			for _, task := range w.waitingExecutionTasks {
				w.AddExecutionTask(NodeExecutionTask{
					NodeID:              task.NodeID,
					PayloadByInputIndex: task.MergePayloadsByInputIndex(),
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
	ItemsByOutputIndex map[int]domain.NodeItems
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
		for outputIndex, payload := range result.Output.ResultJSONByOutputIndex {
			key := sourceOutputKey{
				SourceNodeID: nodeID,
				OutputIndex:  outputIndex,
			}

			nodes, ok := w.nodesBySourceOutputKey[key]
			if !ok {
				continue
			}

			if !payload.IsEmpty() {
				for _, node := range nodes {
					err := w.AddTaskForDownstreamNode(ctx, AddTaskForDownstreamNodeParams{
						FromNodeID:  nodeID,
						Node:        node,
						Payload:     payload,
						OutputIndex: outputIndex,
					})
					if err != nil {
						return ExecuteNodeResult{}, err
					}
				}
			}
		}
	}

	w.MarkNodeAsExecuted(nodeID)

	itemsByOutputIndex := result.Output.ToItemsByOutputIndex(nodeID)
	itemsByInputIndex := execution.PayloadByInputIndex.ToItemsByInputIndex()

	err = w.observer.Notify(ctx, NodeExecutionCompletedEvent{
		NodeID:                nodeID,
		PayloadByInputIndex:   execution.PayloadByInputIndex,
		IntegrationOutput:     result.Output,
		ItemsByInputIndex:     itemsByInputIndex,
		ItemsByOutputIndex:    itemsByOutputIndex,
		ExecutionOrder:        executionOrder,
		IntegrationType:       result.IntegrationType,
		IntegrationActionType: result.IntegrationActionType,
		StartedAt:             nodeExecutionStartedAt,
		EndedAt:               nodeExecutionEndedAt,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to notify node execution completed")
	}

	return ExecuteNodeResult{
		ItemsByOutputIndex: itemsByOutputIndex,
	}, nil
}

func (w *WorkflowExecutor) ExecuteTriggerNode(ctx context.Context, node domain.WorkflowNode, execution NodeExecutionTask) (NodeExecutionResult, error) {

	inputPayload, exists := execution.PayloadByInputIndex[0]
	if !exists {
		err := fmt.Errorf("trigger input payload not found")
		return NodeExecutionResult{}, err
	}

	return NodeExecutionResult{
		Output: domain.IntegrationOutput{
			ResultJSONByOutputIndex: []domain.Payload{inputPayload.Payload},
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
				ResultJSONByOutputIndex: []domain.Payload{},
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

	payloadByInputIndex := execution.PayloadByInputIndex.ToPayloadByInputIndex()

	output, err := integrationExecutor.Execute(ctx, domain.IntegrationInput{
		NodeID:              node.ID,
		PayloadByInputIndex: payloadByInputIndex,
		Workflow:            &w.workflow,
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
	FromNodeID  string
	Node        domain.WorkflowNode
	OutputIndex int
	Payload     domain.Payload
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
	outputIndex := p.OutputIndex

	matchingInputIndex := -1 // Default to no match

	for _, input := range node.Inputs {
		for _, subscribedOutput := range input.SubscribedOutputs {
			isNodeIDMatch := subscribedOutput.NodeID == p.FromNodeID
			isOutputIndexMatch := subscribedOutput.Index == outputIndex

			isMatch := isNodeIDMatch && isOutputIndexMatch

			if isMatch {
				matchingInputIndex = input.Input.Index
			}
		}
	}

	if matchingInputIndex == -1 {
		return fmt.Errorf("input index not found for subscribed output index %d", outputIndex)
	}

	waitingTask, exists := w.GetWaitingExecutionTask(p.Node.ID)
	if exists {
		waitingTask.AddPayload(p.FromNodeID, matchingInputIndex, outputIndex, payload)

		canExecute := w.shouldExecuteWaitingTask(ctx, waitingTask, node)

		if canExecute {
			payloadsByInputIndex := map[int]NodePayload{}

			for inputIndex, p := range waitingTask.ReceivedPayloads {
				for _, payload := range p {
					payloadsByInputIndex[inputIndex] = payload
				}
			}

			w.AddExecutionTask(NodeExecutionTask{
				NodeID:              node.ID,
				PayloadByInputIndex: payloadsByInputIndex,
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
			ReceivedPayloads: map[int]map[int]NodePayload{
				matchingInputIndex: {
					outputIndex: {
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
	payloadByInputIndex := map[int]NodePayload{}

	for _, input := range node.Inputs {
		sourceNodePayload := NodePayload{
			SourceNodeID: p.FromNodeID,
			Payload:      payload,
		}

		payloadByInputIndex[input.Input.Index] = sourceNodePayload

		break
	}

	w.AddExecutionTask(NodeExecutionTask{
		NodeID:              node.ID,
		PayloadByInputIndex: payloadByInputIndex,
	})

	return nil
}

func (w *WorkflowExecutor) shouldExecuteWaitingTask(ctx context.Context, waitingTask WaitingExecutionTask, node domain.WorkflowNode) bool {
	waitingTask.mutex.Lock()
	defer waitingTask.mutex.Unlock()

	eventStatuses := []bool{}

	for _, input := range node.Inputs {
		payloadsForInput, exists := waitingTask.ReceivedPayloads[input.Input.Index]
		if !exists {
			return false
		}

		for _, subscribedOutput := range input.SubscribedOutputs {
			_, exists := payloadsForInput[subscribedOutput.Index]
			if !exists {
				log.Info().Msgf("Missing payload for input %d, output %d", input.Input.Index, subscribedOutput.Index)

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
			ResultJSONByOutputIndex: []domain.Payload{errorPayload},
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
