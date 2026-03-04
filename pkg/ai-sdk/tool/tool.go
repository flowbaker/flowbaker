package tool

import (
	"context"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args string) (string, error)
}

type UserInputTool interface {
	Tool
	SendInputEvent(ctx context.Context, toolCall types.ToolCall) error
}

// EventEmittingTool is an optional interface that tools can implement
// to emit custom stream events during execution
type EventEmittingTool interface {
	Tool
	SetEventEmitter(emitter func(types.StreamEvent))
	HasEventEmitter() bool
}

// ToolAdderTool is an optional interface that tools can implement
// to dynamically add new tools to the agent during execution
type ToolAdderTool interface {
	Tool
	SetToolAdder(adder func(Tool))
}

type FuncTool struct {
	name        string
	description string
	parameters  map[string]any
	fn          func(string) (string, error)
}

func (t *FuncTool) Name() string {
	return t.name
}

func (t *FuncTool) Description() string {
	return t.description
}

func (t *FuncTool) Parameters() map[string]any {
	return t.parameters
}

func (t *FuncTool) Execute(ctx context.Context, args string) (string, error) {
	return t.fn(args)
}

func Define(name, description string, parameters map[string]any, fn func(string) (string, error)) Tool {
	return &FuncTool{
		name:        name,
		description: description,
		parameters:  parameters,
		fn:          fn,
	}
}

func ToTypesTool(t Tool) types.Tool {
	return types.Tool{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.Parameters(),
	}
}
