package mappers

import (
	"flowbaker/internal/domain"
	"flowbaker/pkg/clients/flowbaker"
	executortypes "flowbaker/pkg/clients/flowbaker-executor"
	"time"
)

// DomainNodeExecutionToFlowbaker converts a domain.NodeExecution to flowbaker.NodeExecution
func DomainNodeExecutionToFlowbaker(de domain.NodeExecution) flowbaker.NodeExecution {
	return flowbaker.NodeExecution{
		ID:                     de.ID,
		NodeID:                 de.NodeID,
		IntegrationType:        string(de.IntegrationType),
		IntegrationActionType:  string(de.IntegrationActionType),
		StartedAt:              de.StartedAt,
		EndedAt:                de.EndedAt,
		ExecutionOrder:         de.ExecutionOrder,
		InputItemsCount:        de.InputItemsCount,
		InputItemsSizeInBytes:  de.InputItemsSizeInBytes,
		OutputItemsCount:       de.OutputItemsCount,
		OutputItemsSizeInBytes: de.OutputItemsSizeInBytes,
	}
}

// DomainNodeExecutionsToFlowbaker converts a slice of domain.NodeExecution to flowbaker.NodeExecution
func DomainNodeExecutionsToFlowbaker(executions []domain.NodeExecution) []flowbaker.NodeExecution {
	result := make([]flowbaker.NodeExecution, len(executions))
	for i, exec := range executions {
		result[i] = DomainNodeExecutionToFlowbaker(exec)
	}
	return result
}

// DomainNodeItemsToFlowbaker converts domain.NodeItems to flowbaker.NodeItems
func DomainNodeItemsToFlowbaker(dni domain.NodeItems) flowbaker.NodeItems {
	items := make([]flowbaker.Item, len(dni.Items))
	for i, item := range dni.Items {
		items[i] = flowbaker.Item(item)
	}

	return flowbaker.NodeItems{
		FromNodeID: dni.FromNodeID,
		Items:      items,
	}
}

// DomainNodeItemsMapToFlowbaker converts a map of domain.NodeItems to flowbaker.NodeItems
func DomainNodeItemsMapToFlowbaker(itemsMap map[string]domain.NodeItems) map[string]flowbaker.NodeItems {
	result := make(map[string]flowbaker.NodeItems, len(itemsMap))
	for k, v := range itemsMap {
		result[k] = DomainNodeItemsToFlowbaker(v)
	}
	return result
}

// DomainNodeExecutionEntryToFlowbaker converts a domain.NodeExecutionEntry to flowbaker.NodeExecutionEntry
func DomainNodeExecutionEntryToFlowbaker(de domain.NodeExecutionEntry) flowbaker.NodeExecutionEntry {
	return flowbaker.NodeExecutionEntry{
		NodeID:          de.NodeID,
		ItemsByInputID:  DomainNodeItemsMapToFlowbaker(de.ItemsByInputID),
		ItemsByOutputID: DomainNodeItemsMapToFlowbaker(de.ItemsByOutputID),
		EventType:       flowbaker.EventType(de.EventType),
		Error:           de.Error,
		Timestamp:       de.Timestamp,
		ExecutionOrder:  de.ExecutionOrder,
	}
}

// DomainNodeExecutionEntriesToFlowbaker converts a slice of domain.NodeExecutionEntry to flowbaker.NodeExecutionEntry
func DomainNodeExecutionEntriesToFlowbaker(entries []domain.NodeExecutionEntry) []flowbaker.NodeExecutionEntry {
	result := make([]flowbaker.NodeExecutionEntry, len(entries))
	for i, entry := range entries {
		result[i] = DomainNodeExecutionEntryToFlowbaker(entry)
	}
	return result
}

// --- Reverse mappings: Flowbaker to Domain ---

// FlowbakerNodeExecutionToDomain converts a flowbaker.NodeExecution to domain.NodeExecution
func FlowbakerNodeExecutionToDomain(fe flowbaker.NodeExecution) domain.NodeExecution {
	return domain.NodeExecution{
		ID:                     fe.ID,
		NodeID:                 fe.NodeID,
		IntegrationType:        domain.IntegrationType(fe.IntegrationType),
		IntegrationActionType:  domain.IntegrationActionType(fe.IntegrationActionType),
		StartedAt:              fe.StartedAt,
		EndedAt:                fe.EndedAt,
		ExecutionOrder:         fe.ExecutionOrder,
		InputItemsCount:        fe.InputItemsCount,
		InputItemsSizeInBytes:  fe.InputItemsSizeInBytes,
		OutputItemsCount:       fe.OutputItemsCount,
		OutputItemsSizeInBytes: fe.OutputItemsSizeInBytes,
	}
}

// FlowbakerNodeExecutionsToDomain converts a slice of flowbaker.NodeExecution to domain.NodeExecution
func FlowbakerNodeExecutionsToDomain(executions []flowbaker.NodeExecution) []domain.NodeExecution {
	result := make([]domain.NodeExecution, len(executions))
	for i, exec := range executions {
		result[i] = FlowbakerNodeExecutionToDomain(exec)
	}
	return result
}

// FlowbakerNodeItemsToDomain converts flowbaker.NodeItems to domain.NodeItems
func FlowbakerNodeItemsToDomain(fni flowbaker.NodeItems) domain.NodeItems {
	items := make([]domain.Item, len(fni.Items))
	for i, item := range fni.Items {
		items[i] = domain.Item(item)
	}

	return domain.NodeItems{
		FromNodeID: fni.FromNodeID,
		Items:      items,
	}
}

// FlowbakerNodeItemsMapToDomain converts a map of flowbaker.NodeItems to domain.NodeItems
func FlowbakerNodeItemsMapToDomain(itemsMap map[string]flowbaker.NodeItems) map[string]domain.NodeItems {
	result := make(map[string]domain.NodeItems, len(itemsMap))
	for k, v := range itemsMap {
		result[k] = FlowbakerNodeItemsToDomain(v)
	}
	return result
}

// FlowbakerNodeExecutionEntryToDomain converts a flowbaker.NodeExecutionEntry to domain.NodeExecutionEntry
func FlowbakerNodeExecutionEntryToDomain(fe flowbaker.NodeExecutionEntry) domain.NodeExecutionEntry {
	return domain.NodeExecutionEntry{
		NodeID:          fe.NodeID,
		ItemsByInputID:  FlowbakerNodeItemsMapToDomain(fe.ItemsByInputID),
		ItemsByOutputID: FlowbakerNodeItemsMapToDomain(fe.ItemsByOutputID),
		EventType:       domain.EventType(fe.EventType),
		Error:           fe.Error,
		Timestamp:       fe.Timestamp,
		ExecutionOrder:  fe.ExecutionOrder,
	}
}

// FlowbakerNodeExecutionEntriesToDomain converts a slice of flowbaker.NodeExecutionEntry to domain.NodeExecutionEntry
func FlowbakerNodeExecutionEntriesToDomain(entries []flowbaker.NodeExecutionEntry) []domain.NodeExecutionEntry {
	result := make([]domain.NodeExecutionEntry, len(entries))
	for i, entry := range entries {
		result[i] = FlowbakerNodeExecutionEntryToDomain(entry)
	}
	return result
}

// --- Executor Types Mappings ---

// ExecutorWorkflowToDomain converts an executor Workflow to domain.Workflow
func ExecutorWorkflowToDomain(w *executortypes.Workflow) domain.Workflow {
	triggers := make([]domain.WorkflowTrigger, len(w.Triggers))
	for i, trigger := range w.Triggers {
		triggers[i] = ExecutorWorkflowTriggerToDomain(trigger)
	}

	actions := make([]domain.WorkflowNode, len(w.Nodes))
	for i, node := range w.Nodes {
		actions[i] = ExecutorWorkflowNodeToDomain(node)
	}

	return domain.Workflow{
		ID:               w.ID,
		Name:             w.Name,
		Description:      w.Description,
		Slug:             w.Slug,
		WorkspaceID:      w.WorkspaceID,
		AuthorUserID:     w.AuthorUserID,
		Triggers:         triggers,
		Actions:          actions,
		LastUpdatedAt:    time.Unix(w.LastUpdatedAt, 0),
		ActivationStatus: domain.WorkflowActivationStatus(w.ActivationStatus),
	}
}

// ExecutorWorkflowTriggerToDomain converts an executor WorkflowTrigger to domain.WorkflowTrigger
func ExecutorWorkflowTriggerToDomain(t executortypes.WorkflowTrigger) domain.WorkflowTrigger {
	return domain.WorkflowTrigger{
		ID:                  t.ID,
		WorkflowID:          t.WorkflowID,
		Name:                t.Name,
		Description:         t.Description,
		Type:                domain.IntegrationType(t.Type),
		EventType:           domain.IntegrationTriggerEventType(t.EventType),
		IntegrationSettings: t.IntegrationSettings,
		Positions: domain.NodePositions{
			XPosition: t.XPosition,
			YPosition: t.YPosition,
		},
	}
}

// ExecutorWorkflowNodeToDomain converts an executor WorkflowNode to domain.WorkflowNode
func ExecutorWorkflowNodeToDomain(n executortypes.WorkflowNode) domain.WorkflowNode {
	inputs := make([]domain.NodeInput, len(n.Inputs))
	for i, input := range n.Inputs {
		inputs[i] = domain.NodeInput{
			InputID:          input.InputID,
			SubscribedEvents: input.SubscribedEvents,
		}
	}

	return domain.WorkflowNode{
		ID:                           n.ID,
		WorkflowID:                   n.WorkflowID,
		Name:                         n.Name,
		SubscribedEvents:             []string{}, // This might need to be mapped differently based on your domain structure
		NodeType:                     domain.IntegrationType(n.IntegrationType),
		ActionType:                   domain.IntegrationActionType(n.IntegrationActionType),
		IntegrationSettings:          n.IntegrationSettings,
		ExpressionSelectedProperties: n.ExpressionSelectedProperties,
		ProvidedByAgent:              n.ProvidedByAgent,
		Positions: domain.NodePositions{
			XPosition: n.XPosition,
			YPosition: n.YPosition,
		},
		Inputs: inputs,
	}
}

// ExecutorTriggerToDomain converts an executor WorkflowTrigger pointer to domain.WorkflowTrigger
func ExecutorTriggerToDomain(t *executortypes.WorkflowTrigger) domain.WorkflowTrigger {
	return ExecutorWorkflowTriggerToDomain(*t)
}

// ExecutorWorkflowTypeToDomain converts an executor WorkflowType to domain.WorkflowType
func ExecutorWorkflowTypeToDomain(wt executortypes.WorkflowType) domain.WorkflowType {
	switch wt {
	case executortypes.WorkflowTypeTesting:
		return domain.WorkflowTypeTesting
	default:
		return domain.WorkflowTypeDefault
	}
}

// --- Domain to Executor Types Mappings ---

// DomainWorkflowToExecutor converts a domain.Workflow to executor.Workflow
func DomainWorkflowToExecutor(w domain.Workflow) executortypes.Workflow {
	return executortypes.Workflow{
		ID:               w.ID,
		Name:             w.Name,
		Description:      w.Description,
		WorkspaceID:      w.WorkspaceID,
		AuthorUserID:     w.AuthorUserID,
		Slug:             w.Slug,
		LastUpdatedAt:    w.LastUpdatedAt.Unix(),
		ActivationStatus: executortypes.WorkflowActivationStatus(w.ActivationStatus),
		Nodes:            DomainWorkflowNodesToExecutor(w.Actions),
		Triggers:         DomainWorkflowTriggersToExecutor(w.Triggers),
	}
}

// DomainWorkflowNodesToExecutor converts domain workflow nodes to executor nodes
func DomainWorkflowNodesToExecutor(nodes []domain.WorkflowNode) []executortypes.WorkflowNode {
	executorNodes := make([]executortypes.WorkflowNode, len(nodes))
	for i, node := range nodes {
		inputs := make([]executortypes.NodeInput, len(node.Inputs))
		for j, input := range node.Inputs {
			inputs[j] = executortypes.NodeInput{
				InputID:          input.InputID,
				SubscribedEvents: input.SubscribedEvents,
			}
		}
		executorNodes[i] = executortypes.WorkflowNode{
			ID:                           node.ID,
			WorkflowID:                   node.WorkflowID,
			Name:                         node.Name,
			IntegrationType:              executortypes.IntegrationType(node.NodeType),
			IntegrationActionType:        executortypes.IntegrationActionType(node.ActionType),
			IntegrationSettings:          node.IntegrationSettings,
			ExpressionSelectedProperties: node.ExpressionSelectedProperties,
			ProvidedByAgent:              node.ProvidedByAgent,
			XPosition:                    node.Positions.XPosition,
			YPosition:                    node.Positions.YPosition,
			Inputs:                       inputs,
		}
	}
	return executorNodes
}

// DomainWorkflowTriggersToExecutor converts domain workflow triggers to executor triggers
func DomainWorkflowTriggersToExecutor(triggers []domain.WorkflowTrigger) []executortypes.WorkflowTrigger {
	executorTriggers := make([]executortypes.WorkflowTrigger, len(triggers))
	for i, trigger := range triggers {
		executorTriggers[i] = executortypes.WorkflowTrigger{
			ID:                  trigger.ID,
			WorkflowID:          trigger.WorkflowID,
			Name:                trigger.Name,
			Description:         trigger.Description,
			Type:                executortypes.IntegrationType(trigger.Type),
			EventType:           executortypes.IntegrationTriggerEventType(trigger.EventType),
			IntegrationSettings: trigger.IntegrationSettings,
			XPosition:           trigger.Positions.XPosition,
			YPosition:           trigger.Positions.YPosition,
		}
	}
	return executorTriggers
}

// DomainWorkflowTriggerToExecutor converts a single domain.WorkflowTrigger to executortypes.WorkflowTrigger
func DomainWorkflowTriggerToExecutor(trigger domain.WorkflowTrigger) executortypes.WorkflowTrigger {
	return executortypes.WorkflowTrigger{
		ID:                  trigger.ID,
		WorkflowID:          trigger.WorkflowID,
		Name:                trigger.Name,
		Description:         trigger.Description,
		Type:                executortypes.IntegrationType(trigger.Type),
		EventType:           executortypes.IntegrationTriggerEventType(trigger.EventType),
		IntegrationSettings: trigger.IntegrationSettings,
		XPosition:           trigger.Positions.XPosition,
		YPosition:           trigger.Positions.YPosition,
	}
}

// DomainWorkflowTypeToExecutor converts a domain.WorkflowType to executortypes.WorkflowType
func DomainWorkflowTypeToExecutor(workflowType domain.WorkflowType) executortypes.WorkflowType {
	switch workflowType {
	case domain.WorkflowTypeTesting:
		return executortypes.WorkflowTypeTesting
	default:
		return executortypes.WorkflowTypeDefault
	}
}

// DomainPeekResultItemsToExecutor converts domain PeekResultItem to executor types
func DomainPeekResultItemsToExecutor(items []domain.PeekResultItem) []executortypes.PeekResultItem {
	result := make([]executortypes.PeekResultItem, len(items))
	for i, item := range items {
		result[i] = executortypes.PeekResultItem{
			Key:     item.Key,
			Value:   item.Value,
			Content: item.Content,
		}
	}
	return result
}
