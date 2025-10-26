package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type TransformIntegrationCreator struct {
	binder domain.IntegrationParameterBinder
}

func NewTransformIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &TransformIntegrationCreator{
		binder: deps.ParameterBinder,
	}
}

func (c *TransformIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewTransformIntegration(TransformIntegrationDependencies{
		ParameterBinder: c.binder,
	})
}

type TransformIntegration struct {
	binder domain.IntegrationParameterBinder

	actionManager *domain.IntegrationActionManager
	actionFuncs   map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error)

	fieldParser *FieldPathParser
}

type TransformIntegrationDependencies struct {
	ParameterBinder domain.IntegrationParameterBinder
}

func NewTransformIntegration(deps TransformIntegrationDependencies) (*TransformIntegration, error) {
	integration := &TransformIntegration{
		binder:      deps.ParameterBinder,
		fieldParser: NewFieldPathParser(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddMultiInput(IntegrationActionType_InnerJoin, integration.InnerJoin).
		AddMultiInput(IntegrationActionType_OuterJoin, integration.OuterJoin).
		AddMultiInput(IntegrationActionType_LeftJoin, integration.LeftJoin).
		AddMultiInput(IntegrationActionType_RightJoin, integration.RightJoin).
		AddMultiInput(IntegrationActionType_ExcludeMatching, integration.ReverseInnerJoin).
		AddMultiInput(IntegrationActionType_Append, integration.AppendStreams).
		AddMultiInput(IntegrationActionType_MergeByOrder, integration.MergeByOrder)

	actionFuncs := map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error){}

	integration.actionFuncs = actionFuncs
	integration.actionManager = actionManager

	return integration, nil
}

func (i *TransformIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *TransformIntegration) AppendStreams(ctx context.Context, params domain.IntegrationInput, itemsByInput [][]domain.Item) ([]domain.Item, error) {
	var outputItems []domain.Item

	for _, items := range itemsByInput {
		outputItems = append(outputItems, items...)
	}

	return outputItems, nil
}

type JoinCriteria struct {
	LeftFieldKey  string `json:"left_field_key"`
	RightFieldKey string `json:"right_field_key"`
}

type JoinParams struct {
	Criteria         []JoinCriteria `json:"criteria"`
	HandleCollisions bool           `json:"handle_collisions"`
}

type itemWithCriteria struct {
	item             domain.Item
	criteria         []JoinCriteria
	index            int
	handleCollisions bool
}

type itemWithIndex struct {
	item  domain.Item
	index int
}

func (i *TransformIntegration) InnerJoin(ctx context.Context, params domain.IntegrationInput, itemsByInput [][]domain.Item) ([]domain.Item, error) {
	if len(itemsByInput) != 2 {
		return nil, fmt.Errorf("inner join requires exactly 2 inputs, got %d", len(itemsByInput))
	}

	firstInputItems := itemsByInput[0]
	secondInputItems := itemsByInput[1]

	// Phase 1: Group left items by their join criteria - O(n)
	leftGroups := make(map[string][]itemWithCriteria)

	for index, item := range firstInputItems {
		var joinParams JoinParams

		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		leftGroups[criteriaKey] = append(leftGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 2: Group right items by their join criteria - O(m)
	rightGroups := make(map[string][]itemWithCriteria)

	for index, item := range secondInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		rightGroups[criteriaKey] = append(rightGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 3: Join matching groups - O(groups × items_in_group)
	var outputItems []domain.Item

	for criteriaKey, leftItems := range leftGroups {
		rightItems, exists := rightGroups[criteriaKey]
		if !exists {
			continue
		}

		// Use hash optimization within this criteria group
		if len(leftItems) > 0 && len(rightItems) > 0 {
			criteria := leftItems[0].criteria

			// Build hash index for right items in this group
			rightIndex := make(map[string][]domain.Item)
			for _, rightItemWithCriteria := range rightItems {
				key, err := i.buildCompositeKey(rightItemWithCriteria.item, criteria, false)
				if err != nil {
					continue
				}
				rightIndex[key] = append(rightIndex[key], rightItemWithCriteria.item)
			}

			// Probe with left items in this group
			for _, leftItemWithCriteria := range leftItems {
				key, err := i.buildCompositeKey(leftItemWithCriteria.item, criteria, true)
				if err != nil {
					continue
				}

				for _, matchedItem := range rightIndex[key] {
					merged := i.mergeItems(leftItemWithCriteria.item, matchedItem, leftItemWithCriteria.handleCollisions)
					outputItems = append(outputItems, merged)
				}
			}
		}
	}

	return outputItems, nil
}

func (i *TransformIntegration) getCriteriaKey(criterias []JoinCriteria) string {
	hasher := fnv.New64a()

	for _, criteria := range criterias {
		hasher.Write([]byte(criteria.LeftFieldKey))
		hasher.Write([]byte{0xFF})
		hasher.Write([]byte(criteria.RightFieldKey))
		hasher.Write([]byte{0xFF})
	}

	return fmt.Sprintf("criteria_%016x", hasher.Sum64())
}

func (i *TransformIntegration) buildCompositeKey(item domain.Item, criterias []JoinCriteria, isLeft bool) (string, error) {
	hasher := fnv.New64a()

	for idx, criteria := range criterias {
		fieldKey := criteria.RightFieldKey
		if isLeft {
			fieldKey = criteria.LeftFieldKey
		}

		val, err := i.getFieldValue(item, fieldKey)
		if err != nil {
			return "", err
		}

		if idx > 0 {
			hasher.Write([]byte{0xFF})
		}

		data, _ := json.Marshal(val)
		hasher.Write(data)
	}

	return fmt.Sprintf("%016x", hasher.Sum64()), nil
}

func (i *TransformIntegration) getFieldValue(item domain.Item, fieldPath string) (interface{}, error) {
	return i.fieldParser.GetValue(item, fieldPath)
}

func (i *TransformIntegration) OuterJoin(ctx context.Context, params domain.IntegrationInput, itemsByInput [][]domain.Item) ([]domain.Item, error) {
	if len(itemsByInput) != 2 {
		return nil, fmt.Errorf("outer join requires exactly 2 inputs, got %d", len(itemsByInput))
	}

	firstInputItems := itemsByInput[0]
	secondInputItems := itemsByInput[1]

	// Phase 1: Group left items by their join criteria - O(n)
	leftGroups := make(map[string][]itemWithCriteria)
	for index, item := range firstInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("left item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		leftGroups[criteriaKey] = append(leftGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 2: Group right items by their join criteria - O(m)
	rightGroups := make(map[string][]itemWithCriteria)
	for index, item := range secondInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("right item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		rightGroups[criteriaKey] = append(rightGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 3: Join matching groups and track matched items - O(groups × items_in_group)
	var outputItems []domain.Item
	matchedLeft := make(map[int]bool)
	matchedRight := make(map[int]bool)

	for criteriaKey, leftItems := range leftGroups {
		rightItems, exists := rightGroups[criteriaKey]
		if !exists {
			continue // These left items will be added as unmatched later
		}

		// Use hash optimization within this criteria group
		if len(leftItems) > 0 && len(rightItems) > 0 {
			criteria := leftItems[0].criteria

			// Build hash index for right items in this group
			rightIndex := make(map[string][]itemWithIndex)
			for _, rightItemWithCriteria := range rightItems {
				key, err := i.buildCompositeKey(rightItemWithCriteria.item, criteria, false)
				if err != nil {
					continue
				}
				rightIndex[key] = append(rightIndex[key], itemWithIndex{
					item:  rightItemWithCriteria.item,
					index: rightItemWithCriteria.index,
				})
			}

			// Probe with left items in this group
			for _, leftItemWithCriteria := range leftItems {
				key, err := i.buildCompositeKey(leftItemWithCriteria.item, criteria, true)
				if err != nil {
					continue
				}

				for _, matchedItemWithIndex := range rightIndex[key] {
					merged := i.mergeItems(leftItemWithCriteria.item, matchedItemWithIndex.item, leftItemWithCriteria.handleCollisions)
					outputItems = append(outputItems, merged)
					matchedLeft[leftItemWithCriteria.index] = true
					matchedRight[matchedItemWithIndex.index] = true
				}
			}
		}
	}

	// Phase 4: Add unmatched items from both sides
	for index, item := range firstInputItems {
		if !matchedLeft[index] {
			outputItems = append(outputItems, item)
		}
	}

	for index, item := range secondInputItems {
		if !matchedRight[index] {
			outputItems = append(outputItems, item)
		}
	}

	return outputItems, nil
}

func (i *TransformIntegration) LeftJoin(ctx context.Context, params domain.IntegrationInput, itemsByInput [][]domain.Item) ([]domain.Item, error) {
	if len(itemsByInput) != 2 {
		return nil, fmt.Errorf("left join requires exactly 2 inputs, got %d", len(itemsByInput))
	}

	firstInputItems := itemsByInput[0]
	secondInputItems := itemsByInput[1]

	// Phase 1: Group left items by their join criteria - O(n)
	leftGroups := make(map[string][]itemWithCriteria)
	for index, item := range firstInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("left item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		leftGroups[criteriaKey] = append(leftGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 2: Group right items by their join criteria - O(m)
	rightGroups := make(map[string][]itemWithCriteria)
	for index, item := range secondInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("right item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		rightGroups[criteriaKey] = append(rightGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 3: Join matching groups and track matched left items - O(groups × items_in_group)
	var outputItems []domain.Item
	matchedLeft := make(map[int]bool)

	for criteriaKey, leftItems := range leftGroups {
		rightItems, exists := rightGroups[criteriaKey]
		if !exists {
			continue // These left items will be added as unmatched later
		}

		// Use hash optimization within this criteria group
		if len(leftItems) > 0 && len(rightItems) > 0 {
			criteria := leftItems[0].criteria

			// Build hash index for right items in this group
			rightIndex := make(map[string][]domain.Item)
			for _, rightItemWithCriteria := range rightItems {
				key, err := i.buildCompositeKey(rightItemWithCriteria.item, criteria, false)
				if err != nil {
					continue
				}
				rightIndex[key] = append(rightIndex[key], rightItemWithCriteria.item)
			}

			// Probe with left items in this group
			for _, leftItemWithCriteria := range leftItems {
				key, err := i.buildCompositeKey(leftItemWithCriteria.item, criteria, true)
				if err != nil {
					continue
				}

				for _, matchedRightItem := range rightIndex[key] {
					merged := i.mergeItems(leftItemWithCriteria.item, matchedRightItem, leftItemWithCriteria.handleCollisions)
					outputItems = append(outputItems, merged)
					matchedLeft[leftItemWithCriteria.index] = true
				}
			}
		}
	}

	// Phase 4: Add unmatched left items only (left join characteristic)
	for index, item := range firstInputItems {
		if !matchedLeft[index] {
			outputItems = append(outputItems, item)
		}
	}

	return outputItems, nil
}

func (i *TransformIntegration) RightJoin(ctx context.Context, params domain.IntegrationInput, itemsByInput [][]domain.Item) ([]domain.Item, error) {
	if len(itemsByInput) != 2 {
		return nil, fmt.Errorf("right join requires exactly 2 inputs, got %d", len(itemsByInput))
	}

	firstInputItems := itemsByInput[0]
	secondInputItems := itemsByInput[1]

	// Phase 1: Group left items by their join criteria - O(n)
	leftGroups := make(map[string][]itemWithCriteria)
	for index, item := range firstInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("left item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		leftGroups[criteriaKey] = append(leftGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 2: Group right items by their join criteria - O(m)
	rightGroups := make(map[string][]itemWithCriteria)
	for index, item := range secondInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("right item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		rightGroups[criteriaKey] = append(rightGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 3: Join matching groups and track matched right items - O(groups × items_in_group)
	var outputItems []domain.Item
	matchedRight := make(map[int]bool)

	for criteriaKey, rightItems := range rightGroups {
		leftItems, exists := leftGroups[criteriaKey]
		if !exists {
			continue // These right items will be added as unmatched later
		}

		// Use hash optimization within this criteria group
		if len(leftItems) > 0 && len(rightItems) > 0 {
			criteria := rightItems[0].criteria

			// Build hash index for left items in this group
			leftIndex := make(map[string][]domain.Item)
			for _, leftItemWithCriteria := range leftItems {
				key, err := i.buildCompositeKey(leftItemWithCriteria.item, criteria, true)
				if err != nil {
					continue
				}
				leftIndex[key] = append(leftIndex[key], leftItemWithCriteria.item)
			}

			// Probe with right items in this group
			for _, rightItemWithCriteria := range rightItems {
				key, err := i.buildCompositeKey(rightItemWithCriteria.item, criteria, false)
				if err != nil {
					continue
				}

				for _, matchedLeftItem := range leftIndex[key] {
					merged := i.mergeItems(matchedLeftItem, rightItemWithCriteria.item, rightItemWithCriteria.handleCollisions)
					outputItems = append(outputItems, merged)
					matchedRight[rightItemWithCriteria.index] = true
				}
			}
		}
	}

	// Phase 4: Add unmatched right items only (right join characteristic)
	for index, item := range secondInputItems {
		if !matchedRight[index] {
			outputItems = append(outputItems, item)
		}
	}

	return outputItems, nil
}

func (i *TransformIntegration) ReverseInnerJoin(ctx context.Context, params domain.IntegrationInput, itemsByInput [][]domain.Item) ([]domain.Item, error) {
	if len(itemsByInput) != 2 {
		return nil, fmt.Errorf("reverse inner join requires exactly 2 inputs, got %d", len(itemsByInput))
	}

	firstInputItems := itemsByInput[0]
	secondInputItems := itemsByInput[1]

	// Phase 1: Group left items by their join criteria - O(n)
	leftGroups := make(map[string][]itemWithCriteria)
	for index, item := range firstInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("left item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		leftGroups[criteriaKey] = append(leftGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 2: Group right items by their join criteria - O(m)
	rightGroups := make(map[string][]itemWithCriteria)
	for index, item := range secondInputItems {
		var joinParams JoinParams
		if err := i.binder.BindToStruct(ctx, item, &joinParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("right item at index %d failed to bind join params: %w", index, err)
		}

		criteriaKey := i.getCriteriaKey(joinParams.Criteria)
		rightGroups[criteriaKey] = append(rightGroups[criteriaKey], itemWithCriteria{
			item:             item,
			criteria:         joinParams.Criteria,
			index:            index,
			handleCollisions: joinParams.HandleCollisions,
		})
	}

	// Phase 3: Find matches within shared criteria groups and mark them - O(groups × items_in_group)
	matchedLeft := make(map[int]bool)
	matchedRight := make(map[int]bool)

	for criteriaKey, leftItems := range leftGroups {
		rightItems, exists := rightGroups[criteriaKey]
		if !exists {
			continue // No matching criteria group - these left items are automatically unmatched
		}

		// Use hash optimization within this criteria group
		if len(leftItems) > 0 && len(rightItems) > 0 {
			criteria := leftItems[0].criteria

			// Build hash index for right items in this group
			rightIndex := make(map[string][]itemWithIndex)
			for _, rightItemWithCriteria := range rightItems {
				key, err := i.buildCompositeKey(rightItemWithCriteria.item, criteria, false)
				if err != nil {
					continue
				}
				rightIndex[key] = append(rightIndex[key], itemWithIndex{
					item:  rightItemWithCriteria.item,
					index: rightItemWithCriteria.index,
				})
			}

			// Probe with left items in this group to find matches
			for _, leftItemWithCriteria := range leftItems {
				key, err := i.buildCompositeKey(leftItemWithCriteria.item, criteria, true)
				if err != nil {
					continue
				}

				// If we find matches, mark both sides as matched (to be excluded)
				if matchedItems, exists := rightIndex[key]; exists && len(matchedItems) > 0 {
					matchedLeft[leftItemWithCriteria.index] = true
					for _, matchedItem := range matchedItems {
						matchedRight[matchedItem.index] = true
					}
				}
			}
		}
	}

	// Phase 4: Add all unmatched items from both sides (reverse inner join characteristic)
	var outputItems []domain.Item

	// Add unmatched left items
	for index, item := range firstInputItems {
		if !matchedLeft[index] {
			outputItems = append(outputItems, item)
		}
	}

	// Add unmatched right items
	for index, item := range secondInputItems {
		if !matchedRight[index] {
			outputItems = append(outputItems, item)
		}
	}

	return outputItems, nil
}

type MergeByOrderParams struct {
	HandleCollisions bool   `json:"handle_collisions"`
	UnmatchedItems   string `json:"unmatched_items"`
}

type itemWithMergeParams struct {
	item             domain.Item
	index            int
	handleCollisions bool
	unmatchedItems   string
}

func (i *TransformIntegration) MergeByOrder(ctx context.Context, params domain.IntegrationInput, itemsByInput [][]domain.Item) ([]domain.Item, error) {
	if len(itemsByInput) != 2 {
		return nil, fmt.Errorf("merge by order requires exactly 2 inputs, got %d", len(itemsByInput))
	}

	firstInputItems := itemsByInput[0]
	secondInputItems := itemsByInput[1]

	leftItemsWithParams := make([]itemWithMergeParams, 0, len(firstInputItems))

	for index, item := range firstInputItems {
		var mergeParams MergeByOrderParams

		if err := i.binder.BindToStruct(ctx, item, &mergeParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("left item at index %d failed to bind merge params: %w", index, err)
		}

		leftItemsWithParams = append(leftItemsWithParams, itemWithMergeParams{
			item:             item,
			index:            index,
			handleCollisions: mergeParams.HandleCollisions,
			unmatchedItems:   mergeParams.UnmatchedItems,
		})
	}

	rightItemsWithParams := make([]itemWithMergeParams, 0, len(secondInputItems))

	for index, item := range secondInputItems {
		var mergeParams MergeByOrderParams

		if err := i.binder.BindToStruct(ctx, item, &mergeParams, params.IntegrationParams.Settings); err != nil {
			return nil, fmt.Errorf("right item at index %d failed to bind merge params: %w", index, err)
		}

		rightItemsWithParams = append(rightItemsWithParams, itemWithMergeParams{
			item:             item,
			index:            index,
			handleCollisions: mergeParams.HandleCollisions,
			unmatchedItems:   mergeParams.UnmatchedItems,
		})
	}

	minLen := len(leftItemsWithParams)

	if len(rightItemsWithParams) < minLen {
		minLen = len(rightItemsWithParams)
	}

	var outputItems []domain.Item

	for idx := 0; idx < minLen; idx++ {
		merged := i.mergeItems(leftItemsWithParams[idx].item, rightItemsWithParams[idx].item, leftItemsWithParams[idx].handleCollisions)
		outputItems = append(outputItems, merged)
	}

	for idx := minLen; idx < len(leftItemsWithParams); idx++ {
		unmatchedItems := leftItemsWithParams[idx].unmatchedItems
		if unmatchedItems == "" {
			unmatchedItems = "truncate"
		}

		if unmatchedItems == "keep_all" {
			outputItems = append(outputItems, leftItemsWithParams[idx].item)
		}
	}

	for idx := minLen; idx < len(rightItemsWithParams); idx++ {
		unmatchedItems := rightItemsWithParams[idx].unmatchedItems
		if unmatchedItems == "" {
			unmatchedItems = "truncate"
		}

		if unmatchedItems == "keep_all" {
			outputItems = append(outputItems, rightItemsWithParams[idx].item)
		}
	}

	return outputItems, nil
}

func (i *TransformIntegration) mergeItems(item1, item2 domain.Item, handleCollisions bool) domain.Item {
	map1, ok1 := item1.(map[string]any)
	map2, ok2 := item2.(map[string]any)

	merged := make(map[string]interface{})

	if !handleCollisions {
		// Original behavior: right side overwrites left side
		if ok1 {
			for k, v := range map1 {
				merged[k] = v
			}
		}
		if ok2 {
			for k, v := range map2 {
				merged[k] = v
			}
		}
	} else {
		// Collision handling: prefix conflicting fields
		if ok1 {
			for k, v := range map1 {
				merged["left_"+k] = v
			}
		}
		if ok2 {
			for k, v := range map2 {
				if ok1 {
					// Check if this key exists in left side
					if _, exists := map1[k]; exists {
						// Collision detected - use prefix
						merged["right_"+k] = v
					} else {
						// No collision - use original key
						merged[k] = v
					}
				} else {
					// No left side to conflict with
					merged[k] = v
				}
			}
		}
	}

	// If neither item is a map, return item2 (arbitrary choice)
	if !ok1 && !ok2 {
		return item2
	}

	return merged
}
