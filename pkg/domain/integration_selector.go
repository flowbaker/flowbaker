package domain

import (
	"context"
	"fmt"
	"net/http"
)

type CreateIntegrationParams struct {
	CredentialID string
	WorkspaceID  string
}

type IntegrationCreator interface {
	CreateIntegration(ctx context.Context, p CreateIntegrationParams) (IntegrationExecutor, error)
}

type IntegrationExecutor interface {
	Execute(ctx context.Context, params IntegrationInput) (IntegrationOutput, error)
}

type IntegrationPeeker interface {
	Peek(ctx context.Context, params PeekParams) (PeekResult, error)
}

type IntegrationPoller interface {
	HandlePollingEvent(ctx context.Context, event PollingEvent) (PollResult, error)
}

type IntegrationConnectionTester interface {
	TestConnection(ctx context.Context, params TestConnectionParams) (bool, error)
}

type TestConnectionParams struct {
	Credential Credential
}

// we use this interface to attach the credential to the request
// for example, for dropbox, we need to attach the access token to the request
type HTTPRequestProvider interface {
	Attach(req *http.Request, credential *Credential) (*http.Request, error)
}

type IntegrationChatReplier interface {
	OnTypingStarted(ctx context.Context) error
	Reply(ctx context.Context, message string) error
}

type SelectIntegrationParams struct {
	IntegrationType IntegrationType
}

type IntegrationSelector interface {
	Select(ctx context.Context, params SelectIntegrationParams) (IntegrationExecutor, error)
	SelectPeeker(ctx context.Context, params SelectIntegrationParams) (IntegrationPeeker, error)
	RegisterIntegration(integrationType IntegrationType, executor IntegrationExecutor)
	RegisterCreator(integrationType IntegrationType, creator IntegrationCreator)
	SelectCreator(ctx context.Context, params SelectIntegrationParams) (IntegrationCreator, error)
	SelectPoller(ctx context.Context, params SelectIntegrationParams) (IntegrationPoller, error)
	RegisterPoller(integrationType IntegrationType, poller IntegrationPoller)
	SelectHTTPRequestProvider(ctx context.Context, params SelectIntegrationParams) (HTTPRequestProvider, error)
	RegisterHTTPRequestProvider(integrationType IntegrationType, httpRequestProvider HTTPRequestProvider)
	SelectConnectionTester(ctx context.Context, params SelectIntegrationParams) (IntegrationConnectionTester, error)
	RegisterConnectionTester(integrationType IntegrationType, connectionTester IntegrationConnectionTester)
}

type integrationSelector struct {
	integrationsByType         map[IntegrationType]IntegrationExecutor
	creatorsByType             map[IntegrationType]IntegrationCreator
	pollingEventHandlersByType map[IntegrationType]IntegrationPoller
	httpRequestProvidersByType map[IntegrationType]HTTPRequestProvider
	connectionTestersByType    map[IntegrationType]IntegrationConnectionTester
}

func NewIntegrationSelector() IntegrationSelector {
	return &integrationSelector{
		integrationsByType:         make(map[IntegrationType]IntegrationExecutor),
		creatorsByType:             make(map[IntegrationType]IntegrationCreator),
		pollingEventHandlersByType: make(map[IntegrationType]IntegrationPoller),
		httpRequestProvidersByType: make(map[IntegrationType]HTTPRequestProvider),
		connectionTestersByType:    make(map[IntegrationType]IntegrationConnectionTester),
	}
}

func (s *integrationSelector) RegisterIntegration(integrationType IntegrationType, executor IntegrationExecutor) {
	s.integrationsByType[integrationType] = executor
}

func (s *integrationSelector) Select(ctx context.Context, params SelectIntegrationParams) (IntegrationExecutor, error) {
	executor, ok := s.integrationsByType[params.IntegrationType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrIntegrationNotFound, params.IntegrationType)
	}

	return executor, nil
}

func (s *integrationSelector) RegisterCreator(integrationType IntegrationType, creator IntegrationCreator) {
	s.creatorsByType[integrationType] = creator
}

func (s *integrationSelector) SelectCreator(ctx context.Context, params SelectIntegrationParams) (IntegrationCreator, error) {
	creator, ok := s.creatorsByType[params.IntegrationType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrIntegrationNotFound, params.IntegrationType)
	}

	return creator, nil
}

func (s *integrationSelector) SelectPeeker(ctx context.Context, params SelectIntegrationParams) (IntegrationPeeker, error) {
	creator, ok := s.creatorsByType[params.IntegrationType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrIntegrationNotFound, params.IntegrationType)
	}

	peeker, ok := creator.(IntegrationPeeker)
	if !ok {
		return nil, fmt.Errorf("integration %s is not peekable", params.IntegrationType)
	}

	return peeker, nil
}

func (s *integrationSelector) RegisterPoller(integrationType IntegrationType, poller IntegrationPoller) {
	s.pollingEventHandlersByType[integrationType] = poller
}

func (s *integrationSelector) SelectPoller(ctx context.Context, params SelectIntegrationParams) (IntegrationPoller, error) {
	poller, ok := s.pollingEventHandlersByType[params.IntegrationType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrIntegrationNotFound, params.IntegrationType)
	}

	return poller, nil
}

func (s *integrationSelector) RegisterHTTPRequestProvider(integrationType IntegrationType, httpRequestProvider HTTPRequestProvider) {
	s.httpRequestProvidersByType[integrationType] = httpRequestProvider
}

func (s *integrationSelector) SelectHTTPRequestProvider(ctx context.Context, params SelectIntegrationParams) (HTTPRequestProvider, error) {
	httpRequestProvider, ok := s.httpRequestProvidersByType[params.IntegrationType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrIntegrationNotFound, params.IntegrationType)
	}

	return httpRequestProvider, nil
}

func (s *integrationSelector) SelectConnectionTester(ctx context.Context, params SelectIntegrationParams) (IntegrationConnectionTester, error) {
	connectionTester, ok := s.connectionTestersByType[params.IntegrationType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrIntegrationNotFound, params.IntegrationType)
	}

	return connectionTester, nil
}

func (s *integrationSelector) RegisterConnectionTester(integrationType IntegrationType, connectionTester IntegrationConnectionTester) {
	s.connectionTestersByType[integrationType] = connectionTester
}
