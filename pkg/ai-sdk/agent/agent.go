package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/memory"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/provider"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/tool"
	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
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
	doneChan  chan struct{}

	mu  sync.RWMutex
	err error

	hooks Hooks
}

type Hooks struct {
	OnBeforeGenerate   func(ctx context.Context, req *provider.GenerateRequest, step *Step)
	OnGenerationFailed func(ctx context.Context, req *provider.GenerateRequest, step *Step, err error)

	OnStepStart    func(ctx context.Context, step *Step)
	OnStepComplete func(ctx context.Context, step *Step)

	OnBeforeMemoryRetrieve  func(ctx context.Context, filter memory.Filter)
	OnMemoryRetrieved       func(ctx context.Context, filter memory.Filter, conversation types.Conversation)
	OnMemoryRetrievalFailed func(ctx context.Context, filter memory.Filter, err error)
	OnBeforeMemorySave      func(ctx context.Context, conversation types.Conversation)
	OnMemorySaved           func(ctx context.Context, conversation types.Conversation)
	OnMemorySaveFailed      func(ctx context.Context, conversation types.Conversation, err error)
}

func New(opts ...Option) (*Agent, error) {
	agent := &Agent{
		eventChan: make(chan types.StreamEvent),
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
		if adder, ok := t.(tool.ToolAdderTool); ok {
			adder.SetToolAdder(func(newTool tool.Tool) {
				agent.mu.Lock()
				agent.Tools = append(agent.Tools, newTool)
				agent.mu.Unlock()
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
	done      <-chan struct{}

	errFn func() error
}

func (s *ChatStream) Err() error {
	return s.errFn()
}

func (s *ChatStream) Wait() error {
	<-s.done
	return s.Err()
}

func (a *Agent) Chat(ctx context.Context, req ChatRequest) (ChatStream, error) {

	err := a.SetupConversation(ctx, req)
	if err != nil {
		return ChatStream{}, err
	}

	go func() {
		defer a.Cleanup()

		defer func() {
			finishReason := a.FinishReason
			if finishReason == "" {
				finishReason = "stop"
			}
			a.eventChan <- types.NewAgentEndedEvent(a.TotalUsage, finishReason)
		}()

		a.eventChan <- types.NewAgentStartedEvent(req.SessionID)

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

			conversation := a.conversation

			genReq := provider.GenerateRequest{
				Messages: conversation.Messages,
				System:   a.SystemPrompt,
				Tools:    tools,
			}

			a.OnBeforeGenerate(&genReq)

			providerStream, err := a.Model.Stream(a.cancelContext, genReq)
			if err != nil {
				a.OnError(err)
				return
			}

			for event := range providerStream.Events {
				a.OnEvent(event)
			}

			if err := providerStream.Err(); err != nil {
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
		done:      a.doneChan,
		errFn:     a.getError,
	}, nil
}

type ChatSyncResult struct {
	Steps        []*Step
	TotalUsage   types.Usage
	FinishReason string
}

func (a *Agent) ChatSync(ctx context.Context, req ChatRequest) (ChatSyncResult, error) {
	stream, err := a.Chat(ctx, req)
	if err != nil {
		return ChatSyncResult{}, err
	}

loop:
	for {
		select {
		case <-ctx.Done():
			return ChatSyncResult{}, ctx.Err()
		case _, ok := <-stream.EventChan:
			if !ok {
				break loop
			}
		}
	}

	if err := stream.Err(); err != nil {
		return ChatSyncResult{}, err
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
	// agent-ended is now emitted via defer in Chat() to ensure it's always sent
	// This method is kept for any additional completion logic if needed
	a.FinishReason = "stop"
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

func (a *Agent) setError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.err = err
}

func (a *Agent) getError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.err
}

func (a *Agent) OnError(err error) {
	a.setError(err)
	a.FinishReason = types.FinishReasonError

	select {
	case a.eventChan <- types.NewStreamErrorEvent(err, "", err.Error(), false):
	default:
	}

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

func (a *Agent) SetupConversation(ctx context.Context, req ChatRequest) error {
	conversation, err := a.GetConversation(ctx, memory.Filter{
		SessionID: req.SessionID,
		Limit:     a.ConversationHistory,
	})
	if err != nil {
		return err
	}

	if conversation.IsInterrupted() && len(req.ToolResults) > 0 {
		conversation.Messages = append(conversation.Messages, types.Message{
			Role:        types.RoleTool,
			ToolResults: req.ToolResults,
			Timestamp:   time.Now(),
		})

		conversation.Status = types.StatusActive

		err = a.SaveConversation(ctx, conversation)
		if err != nil {
			return fmt.Errorf("failed to save conversation: %w, conversation_id: %s", err, conversation.ID)
		}

		a.conversation = &conversation

		return nil
	}

	if conversation.IsInterrupted() && req.Prompt != "" {
		pendingToolCalls := a.getPendingToolCalls(conversation)

		if len(pendingToolCalls) > 0 {
			skippedResults := make([]types.ToolResult, 0, len(pendingToolCalls))

			for _, tc := range pendingToolCalls {
				skippedResults = append(skippedResults, types.ToolResult{
					ToolCallID: tc.ID,
					Content:    `{"skipped": true, "reason": "User sent a new message instead of responding to the input request"}`,
					IsError:    false,
				})
			}

			conversation.Messages = append(conversation.Messages, types.Message{
				Role:        types.RoleTool,
				ToolResults: skippedResults,
				Timestamp:   time.Now(),
			})
		}

		conversation.Status = types.StatusActive
	}

	if req.Prompt != "" {
		conversation.Messages = append(conversation.Messages, types.Message{
			Role:      types.RoleUser,
			Content:   req.Prompt,
			Timestamp: time.Now(),
		})
	}

	a.conversation = &conversation

	return nil
}

func (a *Agent) getPendingToolCalls(conversation types.Conversation) []types.ToolCall {
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		msg := conversation.Messages[i]

		if msg.Role == types.RoleAssistant && len(msg.ToolCalls) > 0 {
			return msg.ToolCalls
		}
	}

	return nil
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

	err := a.SaveConversation(ctx, *conversation)
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

	err := a.SaveConversation(ctx, *conversation)
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

func (a *Agent) GetSteps() []*Step {
	return a.steps
}

func (a *Agent) SaveConversation(ctx context.Context, conversation types.Conversation) error {
	if a.Memory == nil {
		return nil
	}

	if conversation.SessionID == "" {
		return nil
	}

	a.OnBeforeMemorySave(ctx, conversation)

	err := a.Memory.SaveConversation(ctx, conversation)
	if err != nil {
		a.OnMemorySaveFailed(ctx, conversation, err)

		return err
	}

	now := time.Now()

	for index, message := range conversation.Messages {
		if message.CreatedAt.IsZero() {
			message.CreatedAt = now
			conversation.Messages[index] = message
		}
	}

	a.OnMemorySaved(ctx, conversation)

	return nil
}

func (a *Agent) GetConversation(ctx context.Context, filter memory.Filter) (types.Conversation, error) {
	if a.Memory == nil {
		return types.Conversation{}, nil
	}

	a.OnBeforeMemoryRetrieve(ctx, filter)

	conversations, err := a.Memory.GetConversation(ctx, filter)
	if err != nil {
		a.OnMemoryRetrievalFailed(ctx, filter, err)

		return types.Conversation{}, err
	}

	a.OnMemoryRetrieved(ctx, filter, conversations)

	return conversations, nil
}

func (a *Agent) OnMemoryRetrieved(ctx context.Context, filter memory.Filter, conversation types.Conversation) {
	if a.hooks.OnMemoryRetrieved != nil {
		a.hooks.OnMemoryRetrieved(ctx, filter, conversation)
	}
}

func (a *Agent) OnBeforeMemoryRetrieve(ctx context.Context, filter memory.Filter) {
	if a.hooks.OnBeforeMemoryRetrieve != nil {
		a.hooks.OnBeforeMemoryRetrieve(ctx, filter)
	}
}

func (a *Agent) OnMemoryRetrievalFailed(ctx context.Context, filter memory.Filter, err error) {
	if a.hooks.OnMemoryRetrievalFailed != nil {
		a.hooks.OnMemoryRetrievalFailed(ctx, filter, err)
	}
}

func (a *Agent) OnBeforeMemorySave(ctx context.Context, conversation types.Conversation) {
	if a.hooks.OnBeforeMemorySave != nil {
		a.hooks.OnBeforeMemorySave(ctx, conversation)
	}
}

func (a *Agent) OnMemorySaved(ctx context.Context, conversation types.Conversation) {
	if a.hooks.OnMemorySaved != nil {
		a.hooks.OnMemorySaved(ctx, conversation)
	}
}

func (a *Agent) OnMemorySaveFailed(ctx context.Context, conversation types.Conversation, err error) {
	if a.hooks.OnMemorySaveFailed != nil {
		a.hooks.OnMemorySaveFailed(ctx, conversation, err)
	}
}
