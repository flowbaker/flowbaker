package executor

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

type ExecutionResult struct {
	Payload    []byte
	Headers    map[string][]string
	StatusCode int
}

type WorkflowExecutorService interface {
	Execute(ctx context.Context, params ExecuteParams) (ExecutionResult, error)
	HandlePollingEvent(ctx context.Context, event domain.PollingEvent) (domain.PollResult, error)
	TestConnection(ctx context.Context, params TestConnectionParams) (bool, error)
	PeekData(ctx context.Context, params PeekDataParams) (domain.PeekResult, error)
	RerunNode(ctx context.Context, params RerunNodeParams) (ExecutionResult, error)
}

type ActiveExecution struct {
	ExecutionID string
	CancelFunc  context.CancelFunc
}

type ExecutionRegistry struct {
	executions map[string]ActiveExecution
	mtx        *sync.RWMutex
}

func NewExecutionRegistry() ExecutionRegistry {
	return ExecutionRegistry{
		executions: make(map[string]ActiveExecution),
		mtx:        &sync.RWMutex{},
	}
}

func (r *ExecutionRegistry) RegisterExecution(activeExecution ActiveExecution) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.executions[activeExecution.ExecutionID] = activeExecution
}

func (r *ExecutionRegistry) UnregisterExecution(executionID string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	delete(r.executions, executionID)
}

func (r *ExecutionRegistry) GetExecution(executionID string) (ActiveExecution, bool) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	execution, ok := r.executions[executionID]
	return execution, ok
}

type workflowExecutorService struct {
	integrationSelector   domain.IntegrationSelector
	flowbakerClient       flowbaker.ClientInterface
	orderedEventPublisher domain.EventPublisher
	credentialManager     domain.ExecutorCredentialManager

	executionRegistry ExecutionRegistry
}

type WorkflowExecutorServiceDependencies struct {
	IntegrationSelector   domain.IntegrationSelector
	OrderedEventPublisher domain.EventPublisher
	FlowbakerClient       flowbaker.ClientInterface
	CredentialManager     domain.ExecutorCredentialManager
}

func NewWorkflowExecutorService(deps WorkflowExecutorServiceDependencies) WorkflowExecutorService {
	executionRegistry := NewExecutionRegistry()

	return &workflowExecutorService{
		integrationSelector:   deps.IntegrationSelector,
		orderedEventPublisher: deps.OrderedEventPublisher,
		flowbakerClient:       deps.FlowbakerClient,
		credentialManager:     deps.CredentialManager,
		executionRegistry:     executionRegistry,
	}
}

type ExecuteParams struct {
	ExecutionID       string
	Workflow          domain.Workflow
	EventName         string
	PayloadJSON       string
	EnableEvents      bool
	IsTestingWorkflow bool
}

func (s *workflowExecutorService) Execute(ctx context.Context, params ExecuteParams) (ExecutionResult, error) {
	workflowExecutor := NewWorkflowExecutor(WorkflowExecutorDeps{
		ExecutionID:           params.ExecutionID,
		Workflow:              params.Workflow,
		Selector:              s.integrationSelector,
		EnableEvents:          params.EnableEvents,
		IsTestingWorkflow:     params.IsTestingWorkflow,
		ExecutorClient:        s.flowbakerClient,
		OrderedEventPublisher: s.orderedEventPublisher,
	})

	p, err := ConvertToArray(params.PayloadJSON)
	if err != nil {
		return ExecutionResult{}, err
	}

	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return ExecutionResult{}, err
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	activeExecution := ActiveExecution{
		ExecutionID: params.ExecutionID,
		CancelFunc:  cancel,
	}

	s.executionRegistry.RegisterExecution(activeExecution)
	defer s.executionRegistry.UnregisterExecution(params.ExecutionID)

	executionResult, err := workflowExecutor.Execute(cancelCtx, params.EventName, []byte(payloadJSON))
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute workflow")

		return ExecutionResult{}, err
	}

	return executionResult, nil
}

func (s *workflowExecutorService) HandlePollingEvent(ctx context.Context, event domain.PollingEvent) (domain.PollResult, error) {
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, domain.NewContextWithWorkflowExecutionContextParams{
		WorkspaceID:         event.WorkspaceID,
		WorkflowID:          event.Workflow.ID,
		WorkflowExecutionID: "",
		EnableEvents:        false,
		Observer:            nil,
	})

	integrationPoller, err := s.integrationSelector.SelectPoller(ctx, domain.SelectIntegrationParams{
		IntegrationType: event.IntegrationType,
	})
	if err != nil {
		log.Error().Err(err).Msgf("Error selecting integration poller for type %s", event.IntegrationType)
		return domain.PollResult{}, err
	}

	result, err := integrationPoller.HandlePollingEvent(ctx, event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to handle polling event")
		return domain.PollResult{}, err
	}

	return result, nil
}

type TestConnectionParams struct {
	IntegrationType domain.IntegrationType
	CredentialID    string
	WorkspaceID     string
	Payload         map[string]any
}

func (s *workflowExecutorService) TestConnection(ctx context.Context, params TestConnectionParams) (bool, error) {
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, domain.NewContextWithWorkflowExecutionContextParams{
		WorkspaceID:         params.WorkspaceID,
		WorkflowID:          "",
		WorkflowExecutionID: "",
		EnableEvents:        false,
		Observer:            nil,
	})

	credential, err := s.credentialManager.GetFullCredential(ctx, params.CredentialID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get credential for connection test")
		return false, err
	}

	if len(params.Payload) > 0 {
		if credential.DecryptedPayload == nil {
			credential.DecryptedPayload = make(map[string]any)
		}
		for key, value := range params.Payload {
			credential.DecryptedPayload[key] = value
		}
	}

	connectionTester, err := s.integrationSelector.SelectConnectionTester(ctx, domain.SelectIntegrationParams{
		IntegrationType: params.IntegrationType,
	})
	if err != nil {
		log.Error().Err(err).Msgf("Failed to select connection tester for type %s", params.IntegrationType)
		return false, err
	}

	success, err := connectionTester.TestConnection(ctx, domain.TestConnectionParams{
		Credential: credential,
	})
	if err != nil {
		log.Error().Err(err).Msg("Connection test failed")
		return false, err
	}

	return success, nil
}

type PeekDataParams struct {
	IntegrationType domain.IntegrationType
	CredentialID    string
	WorkspaceID     string
	UserID          string
	PeekableType    string
	Cursor          string
	PayloadJSON     []byte
}

func (s *workflowExecutorService) PeekData(ctx context.Context, params PeekDataParams) (domain.PeekResult, error) {
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, domain.NewContextWithWorkflowExecutionContextParams{
		WorkspaceID:         params.WorkspaceID,
		WorkflowID:          "",
		WorkflowExecutionID: "",
		EnableEvents:        false,
		Observer:            nil,
	})

	integrationCreator, err := s.integrationSelector.SelectCreator(ctx, domain.SelectIntegrationParams{
		IntegrationType: params.IntegrationType,
	})
	if err != nil {
		log.Error().Err(err).Msgf("Failed to select integration creator for type %s", params.IntegrationType)
		return domain.PeekResult{}, err
	}

	integration, err := integrationCreator.CreateIntegration(ctx, domain.CreateIntegrationParams{
		CredentialID: params.CredentialID,
		WorkspaceID:  params.WorkspaceID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create integration")
		return domain.PeekResult{}, err
	}

	integrationPeeker, ok := integration.(domain.IntegrationPeeker)
	if !ok {
		return domain.PeekResult{}, errors.New("integration is not peekable")
	}

	result, err := integrationPeeker.Peek(ctx, domain.PeekParams{
		PeekableType: domain.IntegrationPeekableType(params.PeekableType),
		PayloadJSON:  params.PayloadJSON,
		Cursor:       params.Cursor,
		UserID:       params.UserID,
		WorkspaceID:  params.WorkspaceID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to peek data")
		return domain.PeekResult{}, err
	}

	return result, nil
}

type RerunNodeParams struct {
	ExecutionID        string
	WorkspaceID        string
	NodeID             string
	NodeExecutionEntry domain.NodeExecutionEntry
	Workflow           domain.Workflow
}

func (s *workflowExecutorService) RerunNode(ctx context.Context, params RerunNodeParams) (ExecutionResult, error) {
	workflowExecutor := NewWorkflowExecutor(WorkflowExecutorDeps{
		ExecutionID:           params.ExecutionID,
		Selector:              s.integrationSelector,
		EnableEvents:          true,
		Workflow:              params.Workflow,
		IsTestingWorkflow:     true,
		ExecutorClient:        s.flowbakerClient,
		OrderedEventPublisher: s.orderedEventPublisher,
	})

	ctx = domain.NewContextWithEventOrder(ctx)
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, domain.NewContextWithWorkflowExecutionContextParams{
		WorkspaceID:         params.WorkspaceID,
		WorkflowID:          params.Workflow.ID,
		WorkflowExecutionID: params.ExecutionID,
		EnableEvents:        true,
		Observer:            workflowExecutor.observer,
		IsReExecution:       true,
	})

	executionEntry := params.NodeExecutionEntry

	payloadByInputID := make(SourceNodePayloadByInputID)

	for inputID, items := range executionEntry.ItemsByInputID {
		itemsJSON, err := json.Marshal(items.Items)
		if err != nil {
			return ExecutionResult{}, err
		}

		payloadByInputID[inputID] = SourceNodePayload{
			SourceNodeID: items.FromNodeID,
			Payload:      itemsJSON,
		}
	}

	task := NodeExecutionTask{
		NodeID:           params.NodeID,
		PayloadByInputID: payloadByInputID,
	}

	err := workflowExecutor.ExecuteNode(ctx, ExecuteNodeParams{
		Task:           task,
		ExecutionOrder: int64(executionEntry.ExecutionOrder),
		Propagate:      true,
	})
	if err != nil {
		return ExecutionResult{}, err
	}

	return ExecutionResult{}, nil
}
