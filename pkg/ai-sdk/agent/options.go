package agent

import (
	"context"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/tool"
)

type Option func(*Agent)

func WithModel(m provider.LanguageModel) Option {
	return func(a *Agent) {
		a.Model = m
	}
}

func WithCancelContext(ctx context.Context) Option {
	return func(a *Agent) {
		a.cancelContext = ctx
	}
}

func WithMemory(m memory.Store) Option {
	return func(a *Agent) {
		a.Memory = m
	}
}

func WithSystemPrompt(prompt string) Option {
	return func(a *Agent) {
		a.SystemPrompt = prompt
	}
}

func WithMaxIterations(iterations int) Option {
	return func(a *Agent) {
		a.MaxIterations = iterations
	}
}

func WithTools(tools ...tool.Tool) Option {
	return func(a *Agent) {
		for _, t := range tools {
			toolName := t.Name()

			a.Tools = append(a.Tools, t)

			if a.UserInputTools == nil {
				a.UserInputTools = make(map[string]tool.UserInputTool)
			}

			if userInputTool, ok := t.(tool.UserInputTool); ok {
				a.UserInputTools[toolName] = userInputTool
			}
		}
	}
}

func WithConversationHistoryLimit(count int) Option {
	return func(a *Agent) {
		a.ConversationHistory = count
	}
}

func WithHooks(hooks Hooks) Option {
	return func(a *Agent) {
		a.hooks = hooks
	}
}
