package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

type ActionFunc func(ctx context.Context, params IntegrationInput) (IntegrationOutput, error)
type ActionFuncPerItem func(ctx context.Context, params IntegrationInput, item Item) (Item, error)
type ActionFuncPerItemMulti func(ctx context.Context, params IntegrationInput, item Item) ([]Item, error)
type ActionFuncPerItemWithFile func(ctx context.Context, params IntegrationInput, item Item) (ItemWithFile, error)
type ActionFuncPerItemMultiWithFile func(ctx context.Context, params IntegrationInput, item Item) ([]ItemWithFile, error)
type ActionFuncMultiInput func(ctx context.Context, params IntegrationInput, items [][]Item) ([]Item, error)
type ActionFuncItemsToItem func(ctx context.Context, params IntegrationInput, items []Item) (Item, error)
type ActionFuncPerItemRoutable func(ctx context.Context, params IntegrationInput, item Item) (RoutableOutput, error)
type PeekFunc func(ctx context.Context, params PeekParams) (PeekResult, error)

type RoutableOutput struct {
	Item        any
	OutputIndex int
}

type IntegrationActionManager struct {
	mtx                        sync.RWMutex
	actionFuncs                map[IntegrationActionType]ActionFunc
	actionFuncsPerItem         map[IntegrationActionType]ActionFuncPerItem
	actionFuncsPerItemMulti    map[IntegrationActionType]ActionFuncPerItemMulti
	actionFuncsPerItemWithFile map[IntegrationActionType]ActionFuncPerItemWithFile
	actionFuncsMultiInput      map[IntegrationActionType]ActionFuncMultiInput
	actionFuncsPerItemRoutable map[IntegrationActionType]ActionFuncPerItemRoutable
	actionFuncsItemsToItem     map[IntegrationActionType]ActionFuncItemsToItem
}

func NewIntegrationActionManager() *IntegrationActionManager {
	return &IntegrationActionManager{
		actionFuncs:                make(map[IntegrationActionType]ActionFunc),
		actionFuncsPerItem:         make(map[IntegrationActionType]ActionFuncPerItem),
		actionFuncsPerItemMulti:    make(map[IntegrationActionType]ActionFuncPerItemMulti),
		actionFuncsPerItemWithFile: make(map[IntegrationActionType]ActionFuncPerItemWithFile),
		actionFuncsMultiInput:      make(map[IntegrationActionType]ActionFuncMultiInput),
		actionFuncsPerItemRoutable: make(map[IntegrationActionType]ActionFuncPerItemRoutable),
		actionFuncsItemsToItem:     make(map[IntegrationActionType]ActionFuncItemsToItem),
	}
}

func (m *IntegrationActionManager) Add(actionType IntegrationActionType, actionFunc ActionFunc) *IntegrationActionManager {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.actionFuncs[actionType] = actionFunc

	return m
}

func (m *IntegrationActionManager) Get(actionType IntegrationActionType) (ActionFunc, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncs[actionType]
	return actionFunc, ok
}

func (m *IntegrationActionManager) AddPerItem(actionType IntegrationActionType, actionFunc ActionFuncPerItem) *IntegrationActionManager {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.actionFuncsPerItem[actionType] = actionFunc

	return m
}

func (m *IntegrationActionManager) AddPerItemMulti(actionType IntegrationActionType, actionFunc ActionFuncPerItemMulti) *IntegrationActionManager {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.actionFuncsPerItemMulti[actionType] = actionFunc

	return m
}

func (m *IntegrationActionManager) AddPerItemWithFile(actionType IntegrationActionType, actionFunc ActionFuncPerItemWithFile) *IntegrationActionManager {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.actionFuncsPerItemWithFile[actionType] = actionFunc

	return m
}

func (m *IntegrationActionManager) AddPerItemRoutable(actionType IntegrationActionType, actionFunc ActionFuncPerItemRoutable) *IntegrationActionManager {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.actionFuncsPerItemRoutable[actionType] = actionFunc

	return m
}

func (m *IntegrationActionManager) AddMultiInput(actionType IntegrationActionType, actionFunc ActionFuncMultiInput) *IntegrationActionManager {
	m.actionFuncsMultiInput[actionType] = actionFunc

	return m
}

func (m *IntegrationActionManager) AddItemsToItem(actionType IntegrationActionType, actionFunc ActionFuncItemsToItem) *IntegrationActionManager {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.actionFuncsItemsToItem[actionType] = actionFunc

	return m
}

func (m *IntegrationActionManager) GetMultiInput(actionType IntegrationActionType) (ActionFuncMultiInput, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncsMultiInput[actionType]
	return actionFunc, ok
}

func (m *IntegrationActionManager) GetPerItem(actionType IntegrationActionType) (ActionFuncPerItem, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncsPerItem[actionType]
	return actionFunc, ok
}

func (m *IntegrationActionManager) GetPerItemMulti(actionType IntegrationActionType) (ActionFuncPerItemMulti, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncsPerItemMulti[actionType]
	return actionFunc, ok
}

func (m *IntegrationActionManager) GetItemsToItem(actionType IntegrationActionType) (ActionFuncItemsToItem, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncsItemsToItem[actionType]
	return actionFunc, ok
}

func (m *IntegrationActionManager) GetPerItemWithFile(actionType IntegrationActionType) (ActionFuncPerItemWithFile, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncsPerItemWithFile[actionType]
	return actionFunc, ok
}

func (m *IntegrationActionManager) GetPerItemRoutable(actionType IntegrationActionType) (ActionFuncPerItemRoutable, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncsPerItemRoutable[actionType]
	return actionFunc, ok
}

func (m *IntegrationActionManager) Run(ctx context.Context, actionType IntegrationActionType, params IntegrationInput) (IntegrationOutput, error) {

	_, ok := m.GetPerItem(actionType)
	if ok {
		return m.RunPerItem(ctx, actionType, params)
	}

	if _, ok := m.GetPerItemMulti(actionType); ok {
		return m.RunPerItemMulti(ctx, actionType, params)
	}

	if _, ok := m.GetItemsToItem(actionType); ok {
		return m.RunItemsToItem(ctx, actionType, params)
	}

	if _, ok := m.GetPerItemWithFile(actionType); ok {
		return m.RunPerItemWithFile(ctx, actionType, params)
	}

	if _, ok := m.GetMultiInput(actionType); ok {
		return m.RunMultiInput(ctx, actionType, params)
	}

	if _, ok := m.GetPerItemRoutable(actionType); ok {
		return m.RunPerItemRoutable(ctx, actionType, params)
	}

	actionFunc, ok := m.Get(actionType)
	if !ok {
		return IntegrationOutput{}, fmt.Errorf("action not found")
	}

	return actionFunc(ctx, params)
}

func (m *IntegrationActionManager) RunPerItem(ctx context.Context, actionType IntegrationActionType, params IntegrationInput) (IntegrationOutput, error) {
	actionFuncPerItem, ok := m.GetPerItem(actionType)
	if !ok {
		return IntegrationOutput{}, fmt.Errorf("action not found")
	}

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputs := make([]Item, 0)

	for _, item := range allItems {
		output, err := actionFuncPerItem(ctx, params, item)
		if err != nil {
			return IntegrationOutput{}, err
		}

		if output == nil {
			continue
		}

		if array, isArray := output.([]any); isArray {
			if len(array) == 0 {
				continue
			}
		}

		if object, isObject := output.(map[string]any); isObject {
			if len(object) == 0 {
				continue
			}
		}

		outputs = append(outputs, output)
	}

	resultJSON, err := json.Marshal(outputs)
	if err != nil {
		return IntegrationOutput{}, err
	}

	return IntegrationOutput{
		ResultJSONByOutputID: []Payload{
			resultJSON,
		},
	}, nil
}

func (m *IntegrationActionManager) RunPerItemMulti(ctx context.Context, actionType IntegrationActionType, params IntegrationInput) (IntegrationOutput, error) {
	actionFuncPerItemMulti, ok := m.GetPerItemMulti(actionType)
	if !ok {
		return IntegrationOutput{}, fmt.Errorf("action not found")
	}

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputs := make([]Item, 0)

	for _, item := range allItems {
		outputItems, err := actionFuncPerItemMulti(ctx, params, item)
		if err != nil {
			return IntegrationOutput{}, err
		}

		if len(outputItems) == 0 {
			continue
		}

		nonEmptyOutputItems := make([]Item, 0)

		for _, outputItem := range outputItems {
			if outputItem == nil {
				continue
			}

			if array, isArray := outputItem.([]any); isArray {
				if len(array) == 0 {
					continue
				}
			}

			if object, isObject := outputItem.(map[string]any); isObject {
				if len(object) == 0 {
					continue
				}
			}

			nonEmptyOutputItems = append(nonEmptyOutputItems, outputItem)
		}

		outputs = append(outputs, nonEmptyOutputItems...)
	}

	resultJSON, err := json.Marshal(outputs)
	if err != nil {
		return IntegrationOutput{}, err
	}

	return IntegrationOutput{
		ResultJSONByOutputID: []Payload{
			resultJSON,
		},
	}, nil
}

const (
	DefaultFileItemFieldKey = "file"
)

func (m *IntegrationActionManager) RunPerItemWithFile(ctx context.Context, actionType IntegrationActionType, params IntegrationInput) (IntegrationOutput, error) {
	actionFuncPerItemWithFile, ok := m.GetPerItemWithFile(actionType)
	if !ok {
		return IntegrationOutput{}, fmt.Errorf("action not found")
	}

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputs := make([]Item, 0)

	for _, item := range allItems {
		output, err := actionFuncPerItemWithFile(ctx, params, item)
		if err != nil {
			return IntegrationOutput{}, err
		}

		if output.Item == nil {
			continue
		}

		if array, isArray := output.Item.([]any); isArray {
			if len(array) == 0 {
				continue
			}
		}

		if object, isObject := output.Item.(map[string]any); isObject {
			if len(object) == 0 {
				continue
			}

			fileFieldKey := output.UseFileFieldKey

			if fileFieldKey == "" {
				fileFieldKey = DefaultFileItemFieldKey
			}

			_, ok := object[fileFieldKey]
			if ok {
				log.Warn().Str("action_type", string(params.ActionType)).Str("file_field_key", fileFieldKey).Msg("collision detected when placing file item in field")

				continue
			}

			object[fileFieldKey] = output.File
		}

		outputs = append(outputs, output.Item)
	}

	resultJSON, err := json.Marshal(outputs)
	if err != nil {
		return IntegrationOutput{}, err
	}

	return IntegrationOutput{
		ResultJSONByOutputID: []Payload{
			resultJSON,
		},
	}, nil
}

func (m *IntegrationActionManager) RunMultiInput(ctx context.Context, actionType IntegrationActionType, params IntegrationInput) (IntegrationOutput, error) {
	actionFuncMultiInput, ok := m.GetMultiInput(actionType)
	if !ok {
		return IntegrationOutput{}, fmt.Errorf("action not found")
	}

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return IntegrationOutput{}, err
	}

	maxInputOrder := -1

	for inputID := range itemsByInputID {
		inputOrder, err := GetInputOrder(inputID)
		if err != nil {
			return IntegrationOutput{}, err
		}

		if inputOrder > maxInputOrder {
			maxInputOrder = inputOrder
		}
	}

	itemsByInputOrder := make([][]Item, maxInputOrder+1)

	for inputID, inputItems := range itemsByInputID {
		inputOrder, err := GetInputOrder(inputID)
		if err != nil {
			return IntegrationOutput{}, err
		}

		itemsForInput := itemsByInputOrder[inputOrder]

		if len(itemsForInput) == 0 {
			itemsForInput = make([]Item, 0)
		}

		itemsForInput = append(itemsForInput, inputItems...)

		itemsByInputOrder[inputOrder] = itemsForInput
	}

	outputs, err := actionFuncMultiInput(ctx, params, itemsByInputOrder)
	if err != nil {
		return IntegrationOutput{}, err
	}

	if outputs == nil {
		outputs = []Item{}
	}

	resultJSON, err := json.Marshal(outputs)
	if err != nil {
		return IntegrationOutput{}, err
	}

	return IntegrationOutput{
		ResultJSONByOutputID: []Payload{
			resultJSON,
		},
	}, nil
}
func (m *IntegrationActionManager) RunPerItemRoutable(ctx context.Context, actionType IntegrationActionType, params IntegrationInput) (IntegrationOutput, error) {
	actionFuncPerItemRoutable, ok := m.GetPerItemRoutable(actionType)
	if !ok {
		return IntegrationOutput{}, fmt.Errorf("action not found")
	}

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputs := make([]RoutableOutput, 0)

	for _, item := range allItems {
		output, err := actionFuncPerItemRoutable(ctx, params, item)
		if err != nil {
			return IntegrationOutput{}, err
		}

		if output.Item == nil {
			continue
		}

		if array, isArray := output.Item.([]any); isArray {
			if len(array) == 0 {
				continue
			}
		}

		if object, isObject := output.Item.(map[string]any); isObject {
			if len(object) == 0 {
				continue
			}
		}

		outputs = append(outputs, output)
	}

	outputsByIndex := make(map[int][]Item)

	for _, output := range outputs {
		outputsByIndex[output.OutputIndex] = append(outputsByIndex[output.OutputIndex], output.Item)
	}

	maxOutputIndex := -1

	for outputIndex := range outputsByIndex {
		if outputIndex > maxOutputIndex {
			maxOutputIndex = outputIndex
		}
	}

	resultJSONs := make([]Payload, maxOutputIndex+1)

	for i := range resultJSONs {
		resultJSONs[i] = []byte(`[]`)
	}

	for outputIndex, outputs := range outputsByIndex {
		resultJSON, err := json.Marshal(outputs)
		if err != nil {
			return IntegrationOutput{}, err
		}

		resultJSONs[outputIndex] = resultJSON
	}

	return IntegrationOutput{
		ResultJSONByOutputID: resultJSONs,
	}, nil
}

func (m *IntegrationActionManager) RunItemsToItem(ctx context.Context, actionType IntegrationActionType, params IntegrationInput) (IntegrationOutput, error) {
	actionFuncItemsToItem, ok := m.GetItemsToItem(actionType)
	if !ok {
		return IntegrationOutput{}, fmt.Errorf("action not found")
	}

	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return IntegrationOutput{}, err
	}

	items := make([]Item, 0)
	for _, inputItems := range itemsByInputID {
		items = append(items, inputItems...)
	}

	output, err := actionFuncItemsToItem(ctx, params, items)
	if err != nil {
		return IntegrationOutput{}, err
	}

	outputItems := []Item{output}
	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return IntegrationOutput{}, err
	}

	return IntegrationOutput{
		ResultJSONByOutputID: []Payload{
			resultJSON,
		}}, nil
}

func GetInputOrder(inputID string) (int, error) {
	parts := strings.Split(inputID, "-")
	if len(parts) != 4 {
		return 0, fmt.Errorf("invalid input ID")
	}

	order, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, err
	}

	return order, nil
}

type IntegrationPeekableManager struct {
	mtx       sync.RWMutex
	peekFuncs map[IntegrationPeekableType]PeekFunc
}

func NewIntegrationPeekableManager() *IntegrationPeekableManager {
	return &IntegrationPeekableManager{
		peekFuncs: make(map[IntegrationPeekableType]PeekFunc),
	}
}

func (m *IntegrationPeekableManager) Add(peekableType IntegrationPeekableType, peekFunc PeekFunc) *IntegrationPeekableManager {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.peekFuncs[peekableType] = peekFunc

	return m
}

func (m *IntegrationPeekableManager) Get(peekableType IntegrationPeekableType) (PeekFunc, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	peekFunc, ok := m.peekFuncs[peekableType]
	return peekFunc, ok
}

func (m *IntegrationPeekableManager) Run(ctx context.Context, peekableType IntegrationPeekableType, params PeekParams) (PeekResult, error) {
	peekFunc, ok := m.Get(peekableType)
	if !ok {
		return PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}
