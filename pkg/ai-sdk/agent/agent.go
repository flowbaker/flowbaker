package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/tool"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/google/uuid"
)

type Agent struct {
	MaxIterations       int
	Tools               []tool.Tool
	UserInputTools      map[string]tool.UserInputTool
	SystemPrompt        string
	Model               provider.LanguageModel
	Memory              memory.Store
	ConversationHistory int

	conversation      *types.Conversation
	steps             []*Step
	currentStepNumber int

	TotalUsage   types.Usage
	FinishReason string

	cancelContext context.Context

	eventChan chan types.StreamEvent
	errChan   chan error
	doneChan  chan struct{}

	hooks Hooks
}

type Hooks struct {
	OnBeforeGenerate   func(ctx context.Context, req *provider.GenerateRequest, step *Step)
	OnGenerationFailed func(ctx context.Context, req *provider.GenerateRequest, step *Step, err error)

	OnStepStart    func(ctx context.Context, step *Step)
	OnStepComplete func(ctx context.Context, step *Step)
}

func New(opts ...Option) (*Agent, error) {
	agent := &Agent{
		eventChan: make(chan types.StreamEvent),
		errChan:   make(chan error),
		doneChan:  make(chan struct{}),
	}

	for _, opt := range opts {
		opt(agent)
	}

	if agent.Model == nil {
		return nil, errors.New("model is required")
	}

	if agent.MaxIterations <= 0 {
		agent.MaxIterations = 10
	}

	for _, t := range agent.Tools {
		if emitter, ok := t.(tool.EventEmittingTool); ok {
			emitter.SetEventEmitter(func(event types.StreamEvent) {
				agent.eventChan <- event
			})
		}
	}

	if agent.Memory == nil {
		agent.Memory = &memory.NoOpMemoryStore{}
	}

	return agent, nil
}

type ChatRequest struct {
	Prompt      string
	SessionID   string
	ToolResults []types.ToolResult // For resuming after user input pause
}

type ChatStream struct {
	EventChan <-chan types.StreamEvent
	ErrChan   <-chan error
	done      chan struct{}
}

func (a *Agent) Chat(ctx context.Context, req ChatRequest) (ChatStream, error) {
	conversation, err := a.SetupConversation(ctx, req)
	if err != nil {
		return ChatStream{}, err
	}

	go func() {
		defer a.Cleanup()

		for a.currentStepNumber < a.MaxIterations {
			if a.CanFinish() {
				break
			}

			a.IncrementStepNumber()

			a.OnStepStart()

			tools := []types.Tool{}

			for _, t := range a.Tools {
				tools = append(tools, tool.ToTypesTool(t))
			}

			req := provider.GenerateRequest{
				Messages: conversation.Messages,
				System:   a.SystemPrompt,
				Tools:    tools,
			}

			a.OnBeforeGenerate(&req)

			events, errs := a.Model.Stream(a.cancelContext, req)

			for event := range events {
				a.OnEvent(event)
			}

			if err := <-errs; err != nil {
				a.OnError(err)
				return
			}

			needsIntervention := a.NeedsIntervention()

			if needsIntervention {
				a.Intervene(ctx)
			}

			err = a.ApplyStepToConversation(ctx)
			if err != nil {
				a.OnError(err)
				return
			}

			a.HandleToolCalls(ctx)

			a.OnStepComplete()
		}

		a.OnComplete()
	}()

	return ChatStream{
		EventChan: a.eventChan,
		ErrChan:   a.errChan,
		done:      a.doneChan,
	}, nil
}

type ChatSyncResult struct {
	Steps        []*Step
	TotalUsage   types.Usage
	FinishReason string
}

func (a *Agent) ChatSync(ctx context.Context, req ChatRequest) (ChatSyncResult, error) {
	_, err := a.Chat(ctx, req)
	if err != nil {
		return ChatSyncResult{}, err
	}

loop:
	for {
		select {
		case err := <-a.errChan:
			if err == nil {
				continue
			}

			return ChatSyncResult{}, err
		case <-ctx.Done():
			return ChatSyncResult{}, ctx.Err()
		case <-a.eventChan:
		case <-a.doneChan:
			break loop
		}
	}

	return ChatSyncResult{
		Steps:        a.steps,
		TotalUsage:   a.TotalUsage,
		FinishReason: a.FinishReason,
	}, nil
}

func (a *Agent) OnStepStart() {
	step := a.CreateStep(a.currentStepNumber)

	if a.hooks.OnStepStart != nil {
		a.hooks.OnStepStart(a.cancelContext, step)
	}

	a.eventChan <- types.NewAgentStepStartEvent(a.currentStepNumber, "")
}

func (a *Agent) OnStepComplete() {
	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return
	}

	a.eventChan <- types.NewAgentStepCompleteEvent(
		a.currentStepNumber,
		currentStep.Content,
		currentStep.ToolCalls,
		currentStep.ToolResults,
		currentStep.Usage,
		currentStep.FinishReason,
	)

	if a.hooks.OnStepComplete != nil {
		a.hooks.OnStepComplete(a.cancelContext, currentStep)
	}
}

func (a *Agent) OnComplete() {
	a.eventChan <- types.NewStreamEndEvent("stop", a.TotalUsage)
}

type Step struct {
	StepNumber   int                `json:"step_number"`
	Content      string             `json:"content"`
	ToolCalls    []types.ToolCall   `json:"tool_calls"`
	ToolResults  []types.ToolResult `json:"tool_results"`
	Usage        types.Usage        `json:"usage"`
	FinishReason string             `json:"finish_reasons"`
	Warnings     []types.Warning    `json:"warnings,omitempty"`

	GenerateRequest provider.GenerateRequest `json:"generate_request"`
}

func (a *Agent) OnEvent(event types.StreamEvent) {
	a.eventChan <- event

	step, ok := a.GetStep(a.currentStepNumber)
	if !ok {
		step = a.CreateStep(a.currentStepNumber)
	}

	switch e := event.(type) {
	case *types.TextDeltaEvent:
		step.Content += e.Delta
	case *types.ToolCallCompleteEvent:
		step.ToolCalls = append(step.ToolCalls, e.ToolCall)
	case *types.UsageEvent:
		step.Usage = step.Usage.Add(e.Usage)

		a.TotalUsage = a.TotalUsage.Add(e.Usage)
	case *types.FinishReasonEvent:
		step.FinishReason = e.Reason
	}
}

func (a *Agent) GetStep(stepNumber int) (*Step, bool) {
	for _, step := range a.steps {
		if step.StepNumber == stepNumber {
			return step, true
		}
	}

	return nil, false
}

func (a *Agent) CreateStep(stepNumber int) *Step {
	step := &Step{
		StepNumber:  stepNumber,
		Content:     "",
		ToolCalls:   []types.ToolCall{},
		ToolResults: []types.ToolResult{},
		Usage:       types.Usage{},
	}

	a.steps = append(a.steps, step)

	return step
}

func (a *Agent) OnBeforeGenerate(req *provider.GenerateRequest) {
	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return
	}

	currentStep.GenerateRequest = *req

	if a.hooks.OnBeforeGenerate != nil {
		a.hooks.OnBeforeGenerate(a.cancelContext, req, currentStep)
	}
}

func (a *Agent) NeedsIntervention() bool {
	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return false
	}

	for _, toolCall := range currentStep.ToolCalls {
		_, exists := a.UserInputTools[toolCall.Name]
		if exists {
			return true
		}
	}

	return false
}

func (a *Agent) Intervene(ctx context.Context) {
	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return
	}

	conversation := a.conversation
	if conversation == nil {
		return
	}

	a.SetConversationStatus(types.StatusInterrupted)

	for _, toolCall := range currentStep.ToolCalls {
		userInputTool, exists := a.UserInputTools[toolCall.Name]
		if exists {
			err := userInputTool.SendInputEvent(ctx, toolCall)
			if err != nil {
				a.OnError(err)

				continue
			}
		}
	}

	a.FinishReason = types.FinishReasonHumanIntervention
	currentStep.FinishReason = types.FinishReasonHumanIntervention
}

func (a *Agent) SetConversationStatus(status types.ConversationStatus) {
	conversation := a.conversation
	if conversation == nil {
		return
	}

	conversation.Status = status
}

func (a *Agent) GetCurrentStep() (*Step, bool) {
	for _, step := range a.steps {
		if step.StepNumber == a.currentStepNumber {
			return step, true
		}
	}

	return nil, false
}

func (a *Agent) IncrementStepNumber() {
	a.currentStepNumber++
}

func (a *Agent) OnError(err error) {
	a.errChan <- err

	if a.hooks.OnGenerationFailed != nil {
		currentStep, ok := a.GetCurrentStep()
		if !ok {
			return
		}

		a.hooks.OnGenerationFailed(a.cancelContext, &currentStep.GenerateRequest, currentStep, err)
	}
}

func (a *Agent) Cleanup() {
	close(a.eventChan)
	close(a.errChan)
	close(a.doneChan)
}

func (a *Agent) HandleToolCalls(ctx context.Context) {
	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return
	}

	toolResults := []types.ToolResult{}

	for _, toolCall := range currentStep.ToolCalls {
		if a.IsHumanInterventionToolCall(toolCall) {
			continue
		}

		toolResult, err := a.HandleToolCall(ctx, currentStep, toolCall)
		if err != nil {
			a.OnError(err)
			return
		}

		toolResults = append(toolResults, toolResult)
	}

	currentStep.ToolResults = toolResults

	err := a.ApplyToolResultsToConversation(ctx)
	if err != nil {
		a.OnError(err)
		return
	}
}

func (a *Agent) HandleToolCall(ctx context.Context, step *Step, toolCall types.ToolCall) (types.ToolResult, error) {
	a.OnToolCallStart(step, toolCall)

	tool, exists := a.GetTool(toolCall.Name)
	if !exists {
		return types.ToolResult{}, fmt.Errorf("tool %s not found", toolCall.Name)
	}

	argsJSON, err := json.Marshal(toolCall.Arguments)
	if err != nil {
		return types.ToolResult{}, fmt.Errorf("failed to marshal tool call arguments: %w", err)
	}

	content, err := tool.Execute(ctx, string(argsJSON))

	toolResult := types.ToolResult{
		ToolCallID: toolCall.ID,
		Content:    content,
		IsError:    err != nil,
	}

	if err != nil {
		toolResult.Content = fmt.Sprintf("Error: %v", err)
	}

	a.OnToolCallComplete(step, toolCall, toolResult)

	return toolResult, nil
}

func (a *Agent) OnToolCallStart(step *Step, toolCall types.ToolCall) {
	a.eventChan <- types.NewToolExecutionStartEvent(toolCall)
}

func (a *Agent) OnToolCallComplete(step *Step, toolCall types.ToolCall, toolResult types.ToolResult) {
	a.eventChan <- types.NewToolExecutionCompleteEvent(toolCall, toolResult)
}

func (a *Agent) GetTool(toolName string) (tool.Tool, bool) {
	for _, t := range a.Tools {
		if t.Name() == toolName {
			return t, true
		}
	}

	return nil, false
}

func (a *Agent) GetSessionHistory(ctx context.Context, sessionID string) ([]types.Message, error) {
	conversations, err := a.Memory.GetConversations(ctx, memory.Filter{
		SessionID: sessionID,
		Limit:     a.ConversationHistory,
	})
	if err != nil {
		return nil, err
	}

	messages := []types.Message{}

	for _, conversation := range conversations {
		messages = append(messages, conversation.Messages...)
	}

	return messages, nil
}

func (a *Agent) SetupConversation(ctx context.Context, req ChatRequest) (*types.Conversation, error) {
	messages := []types.Message{}

	if a.Memory != nil {
		interruptedConversations, err := a.Memory.GetConversations(ctx, memory.Filter{
			SessionID: req.SessionID,
			Status:    types.StatusInterrupted,
			Limit:     1,
		})
		if err != nil {
			return nil, err
		}

		if len(interruptedConversations) > 0 {
			conversation := interruptedConversations[0]

			conversation.Messages = append(conversation.Messages, types.Message{
				Role:        types.RoleTool,
				ToolResults: req.ToolResults,
				Timestamp:   time.Now(),
			})

			err = a.Memory.SaveConversation(ctx, conversation)
			if err != nil {
				return nil, fmt.Errorf("failed to save conversation: %w, conversation_id: %s", err, conversation.ID)
			}

			a.conversation = conversation

			return conversation, nil
		}

		messages, err = a.GetSessionHistory(ctx, req.SessionID)
		if err != nil {
			return nil, err
		}
	}

	if req.Prompt != "" {
		messages = append(messages, types.Message{
			Role:      types.RoleUser,
			Content:   req.Prompt,
			Timestamp: time.Now(),
		})
	}

	conversation := &types.Conversation{
		ID:        uuid.New().String(),
		SessionID: req.SessionID,
		Messages:  messages,
		Status:    types.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	a.conversation = conversation

	return conversation, nil
}

func (a *Agent) ApplyStepToConversation(ctx context.Context) error {
	conversation := a.conversation

	if conversation == nil {
		return errors.New("conversation not found")
	}

	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return errors.New("current step not found")
	}

	conversation.Messages = append(conversation.Messages, types.Message{
		Role:      types.RoleAssistant,
		Content:   currentStep.Content,
		ToolCalls: currentStep.ToolCalls,
		Timestamp: time.Now(),
	})

	err := a.Memory.SaveConversation(ctx, conversation)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w, conversation_id: %s", err, conversation.ID)
	}

	return nil
}

func (a *Agent) ApplyToolResultsToConversation(ctx context.Context) error {
	conversation := a.conversation
	if conversation == nil {
		return errors.New("conversation not found")
	}

	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return errors.New("current step not found")
	}

	if len(currentStep.ToolResults) == 0 {
		return nil
	}

	conversation.Messages = append(conversation.Messages, types.Message{
		Role:        types.RoleTool,
		ToolResults: currentStep.ToolResults,
		Timestamp:   time.Now(),
	})

	err := a.Memory.SaveConversation(ctx, conversation)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w, conversation_id: %s", err, conversation.ID)
	}

	return nil

}

func (a *Agent) IsHumanInterventionToolCall(toolCall types.ToolCall) bool {
	_, exists := a.UserInputTools[toolCall.Name]
	return exists
}

func (a *Agent) CanFinish() bool {
	currentStep, ok := a.GetCurrentStep()
	if !ok {
		return false
	}

	if len(currentStep.ToolCalls) == 0 || len(currentStep.ToolResults) == 0 {
		return true
	}

	finishableReasons := []string{
		types.FinishReasonStop,
		types.FinishReasonLength,
		types.FinishReasonContentFilter,
		types.FinishReasonError,
		types.FinishReasonHumanIntervention,
	}

	for _, reason := range finishableReasons {
		if currentStep.FinishReason == reason {
			return true
		}
	}

	return false
}
