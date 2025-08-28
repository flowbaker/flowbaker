package mappers

import (
	"flowbaker/internal/domain"
	"flowbaker/pkg/flowbaker"
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
