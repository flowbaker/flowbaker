package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
}

type workflowExecutorService struct {
	integrationSelector   domain.IntegrationSelector
	flowbakerClient       flowbaker.ClientInterface
	orderedEventPublisher domain.EventPublisher
	credentialManager     domain.ExecutorCredentialManager
}

type WorkflowExecutorServiceDependencies struct {
	IntegrationSelector   domain.IntegrationSelector
	OrderedEventPublisher domain.EventPublisher
	FlowbakerClient       flowbaker.ClientInterface
	CredentialManager     domain.ExecutorCredentialManager
}

func NewWorkflowExecutorService(deps WorkflowExecutorServiceDependencies) WorkflowExecutorService {
	return &workflowExecutorService{
		integrationSelector:   deps.IntegrationSelector,
		orderedEventPublisher: deps.OrderedEventPublisher,
		flowbakerClient:       deps.FlowbakerClient,
		credentialManager:     deps.CredentialManager,
	}
}

type ExecuteParams struct {
	Workflow          domain.Workflow
	EventName         string
	PayloadJSON       string
	EnableEvents      bool
	IsTestingWorkflow bool
}

func (s *workflowExecutorService) Execute(ctx context.Context, params ExecuteParams) (ExecutionResult, error) {
	workflowExecutor := NewWorkflowExecutor(WorkflowExecutorDeps{
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

	executionResult, err := workflowExecutor.Execute(ctx, params.EventName, []byte(payloadJSON))
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute workflow")

		return ExecutionResult{}, err
	}

	return executionResult, nil
}

func (s *workflowExecutorService) HandlePollingEvent(ctx context.Context, event domain.PollingEvent) (domain.PollResult, error) {
	log.Info().
		Str("workspaceID", event.WorkspaceID).
		Str("workflowID", event.Workflow.ID).
		Str("integrationType", string(event.IntegrationType)).
		Str("triggerID", event.Trigger.ID).
		Str("eventType", string(event.Trigger.EventType)).
		Str("userID", event.UserID).
		Msg("WorkflowExecutorService: Starting polling event handling")

	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, event.WorkspaceID, event.Workflow.ID, "", false)

	log.Debug().
		Str("workspaceID", event.WorkspaceID).
		Str("integrationType", string(event.IntegrationType)).
		Msg("WorkflowExecutorService: Selecting integration poller")

	integrationPoller, err := s.integrationSelector.SelectPoller(ctx, domain.SelectIntegrationParams{
		IntegrationType: event.IntegrationType,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", event.WorkspaceID).
			Str("integrationType", string(event.IntegrationType)).
			Str("workflowID", event.Workflow.ID).
			Msgf("WorkflowExecutorService: Error selecting integration poller for type %s", event.IntegrationType)
		return domain.PollResult{}, err
	}

	log.Info().
		Str("workspaceID", event.WorkspaceID).
		Str("integrationType", string(event.IntegrationType)).
		Str("pollerType", fmt.Sprintf("%T", integrationPoller)).
		Msg("WorkflowExecutorService: Integration poller selected, calling HandlePollingEvent")

	result, err := integrationPoller.HandlePollingEvent(ctx, event)
	if err != nil {
		log.Error().
			Err(err).
			Str("workspaceID", event.WorkspaceID).
			Str("integrationType", string(event.IntegrationType)).
			Str("triggerID", event.Trigger.ID).
			Str("workflowID", event.Workflow.ID).
			Msg("WorkflowExecutorService: Failed to handle polling event")
		return domain.PollResult{}, err
	}

	log.Info().
		Str("workspaceID", event.WorkspaceID).
		Str("integrationType", string(event.IntegrationType)).
		Str("lastModifiedData", result.LastModifiedData).
		Msg("WorkflowExecutorService: Successfully handled polling event")

	return result, nil
}

type TestConnectionParams struct {
	IntegrationType domain.IntegrationType
	CredentialID    string
	WorkspaceID     string
	Payload         map[string]any
}

func (s *workflowExecutorService) TestConnection(ctx context.Context, params TestConnectionParams) (bool, error) {
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, params.WorkspaceID, "", "", false)

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
	ctx = domain.NewContextWithWorkflowExecutionContext(ctx, params.WorkspaceID, "", "", false)

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
