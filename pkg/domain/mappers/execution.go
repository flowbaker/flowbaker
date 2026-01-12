package mappers

import (
	"time"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	executortypes "github.com/flowbaker/flowbaker/pkg/clients/flowbaker-executor"

	"github.com/flowbaker/flowbaker/pkg/domain"
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
	nodes := make([]domain.WorkflowNode, len(w.Nodes))
	for i, node := range w.Nodes {
		nodes[i] = ExecutorWorkflowNodeToDomain(node)
	}

	loops := []domain.WorkflowLoop{}
	if len(w.Loops) > 0 {
		loops = make([]domain.WorkflowLoop, len(w.Loops))
		for i, loop := range w.Loops {
			loops[i] = ExecutorWorkflowLoopToDomain(loop)
		}
	}

	return domain.Workflow{
		ID:               w.ID,
		Name:             w.Name,
		Description:      w.Description,
		Slug:             w.Slug,
		WorkspaceID:      w.WorkspaceID,
		AuthorUserID:     w.AuthorUserID,
		Nodes:            nodes,
		Loops:            loops,
		LastUpdatedAt:    time.Unix(w.LastUpdatedAt, 0),
		ActivationStatus: domain.WorkflowActivationStatus(w.ActivationStatus),
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
		ID:                  n.ID,
		WorkflowID:          n.WorkflowID,
		Name:                n.Name,
		SubscribedEvents:    []string{},
		Type:                domain.NodeType(n.Type),
		IntegrationType:     domain.IntegrationType(n.IntegrationType),
		IntegrationSettings: n.IntegrationSettings,
		Settings: domain.Settings{
			ReturnErrorAsItem:    n.Settings.ReturnErrorAsItem,
			ContainPreviousItems: n.Settings.ContainPreviousItems,
		},
		ExpressionSelectedProperties: n.ExpressionSelectedProperties,
		ProvidedByAgent:              n.ProvidedByAgent,
		Positions: domain.NodePositions{
			XPosition: n.XPosition,
			YPosition: n.YPosition,
		},
		Inputs:       inputs,
		UsageContext: n.UsageContext,
		ParentID:     n.ParentID,
		ActionNodeOpts: domain.ActionNodeOpts{
			ActionType: domain.IntegrationActionType(n.ActionNodeOpts.ActionType),
		},
		TriggerNodeOpts: domain.TriggerNodeOpts{
			EventType: domain.IntegrationTriggerEventType(n.TriggerNodeOpts.EventType),
		},
	}
}

// ExecutorWorkflowLoopToDomain converts an executor WorkflowLoop to domain.WorkflowLoop
func ExecutorWorkflowLoopToDomain(l executortypes.WorkflowLoop) domain.WorkflowLoop {
	return domain.WorkflowLoop{
		ID:        l.ID,
		Threshold: l.Threshold,
		EdgeIDs:   l.EdgeIDs,
		NodeIDs:   l.NodeIDs,
	}
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
	loops := make([]executortypes.WorkflowLoop, len(w.Loops))
	for i, loop := range w.Loops {
		loops[i] = DomainWorkflowLoopToExecutor(loop)
	}

	return executortypes.Workflow{
		ID:               w.ID,
		Name:             w.Name,
		Description:      w.Description,
		WorkspaceID:      w.WorkspaceID,
		AuthorUserID:     w.AuthorUserID,
		Slug:             w.Slug,
		Loops:            loops,
		LastUpdatedAt:    w.LastUpdatedAt.Unix(),
		ActivationStatus: executortypes.WorkflowActivationStatus(w.ActivationStatus),
		Nodes:            DomainWorkflowNodesToExecutor(w.Nodes),
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
			ID:                  node.ID,
			WorkflowID:          node.WorkflowID,
			Name:                node.Name,
			Type:                executortypes.NodeType(node.Type),
			IntegrationType:     executortypes.IntegrationType(node.IntegrationType),
			IntegrationSettings: node.IntegrationSettings,
			Settings: executortypes.Settings{
				ReturnErrorAsItem:    node.Settings.ReturnErrorAsItem,
				ContainPreviousItems: node.Settings.ContainPreviousItems,
			},
			ExpressionSelectedProperties: node.ExpressionSelectedProperties,
			ProvidedByAgent:              node.ProvidedByAgent,
			XPosition:                    node.Positions.XPosition,
			YPosition:                    node.Positions.YPosition,
			Inputs:                       inputs,
			UsageContext:                 node.UsageContext,
			ParentID:                     node.ParentID,
			ActionNodeOpts: executortypes.ActionNodeOpts{
				ActionType: executortypes.IntegrationActionType(node.ActionNodeOpts.ActionType),
			},
			TriggerNodeOpts: executortypes.TriggerNodeOpts{
				EventType: executortypes.IntegrationTriggerEventType(node.TriggerNodeOpts.EventType),
			},
		}
	}
	return executorNodes
}

// DomainWorkflowLoopToExecutor converts a domain.WorkflowLoop to executortypes.WorkflowLoop
func DomainWorkflowLoopToExecutor(l domain.WorkflowLoop) executortypes.WorkflowLoop {
	return executortypes.WorkflowLoop{
		ID:        l.ID,
		Threshold: l.Threshold,
		EdgeIDs:   l.EdgeIDs,
		NodeIDs:   l.NodeIDs,
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
