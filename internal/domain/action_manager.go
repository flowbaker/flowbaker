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
type PeekFunc func(ctx context.Context, params PeekParams) (PeekResult, error)

type IntegrationActionManager struct {
	mtx                        sync.RWMutex
	actionFuncs                map[IntegrationActionType]ActionFunc
	actionFuncsPerItem         map[IntegrationActionType]ActionFuncPerItem
	actionFuncsPerItemMulti    map[IntegrationActionType]ActionFuncPerItemMulti
	actionFuncsPerItemWithFile map[IntegrationActionType]ActionFuncPerItemWithFile
	actionFuncsMultiInput      map[IntegrationActionType]ActionFuncMultiInput
}

func NewIntegrationActionManager() *IntegrationActionManager {
	return &IntegrationActionManager{
		actionFuncs:                make(map[IntegrationActionType]ActionFunc),
		actionFuncsPerItem:         make(map[IntegrationActionType]ActionFuncPerItem),
		actionFuncsPerItemMulti:    make(map[IntegrationActionType]ActionFuncPerItemMulti),
		actionFuncsPerItemWithFile: make(map[IntegrationActionType]ActionFuncPerItemWithFile),
		actionFuncsMultiInput:      make(map[IntegrationActionType]ActionFuncMultiInput),
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

func (m *IntegrationActionManager) AddMultiInput(actionType IntegrationActionType, actionFunc ActionFuncMultiInput) *IntegrationActionManager {
	m.actionFuncsMultiInput[actionType] = actionFunc

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

func (m *IntegrationActionManager) GetPerItemWithFile(actionType IntegrationActionType) (ActionFuncPerItemWithFile, bool) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	actionFunc, ok := m.actionFuncsPerItemWithFile[actionType]
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

	if _, ok := m.GetPerItemWithFile(actionType); ok {
		return m.RunPerItemWithFile(ctx, actionType, params)
	}

	if _, ok := m.GetMultiInput(actionType); ok {
		return m.RunMultiInput(ctx, actionType, params)
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

	maxInputLength := 0

	for _, items := range itemsByInputID {
		if len(items) > maxInputLength {
			maxInputLength = len(items)
		}
	}

	outputs := make([]Item, 0)

	for i := 0; i < maxInputLength; i++ {
		itemsByInputOrder := make([][]Item, 0, len(itemsByInputID))

		for inputID, inputItems := range itemsByInputID {
			if i >= len(inputItems) {
				continue
			}

			order, err := GetInputOrder(inputID)
			if err != nil {
				return IntegrationOutput{}, err
			}

			inputItems := itemsByInputOrder[order]

			inputItems = append(inputItems, inputItems[i])
			itemsByInputOrder[order] = inputItems
		}

		output, err := actionFuncMultiInput(ctx, params, itemsByInputOrder)
		if err != nil {
			return IntegrationOutput{}, err
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
