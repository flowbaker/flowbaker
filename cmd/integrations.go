package main

import (
	"context"
	"flowbaker/internal/domain"

	"github.com/rs/zerolog/log"
)

// IntegrationCommonDependencies contains dependencies that most integrations need

type SchemaRegistrationFunc func(ctx context.Context, integrationManager domain.ExecutorIntegrationManager) error

type IntegrationRegisterParams struct {
	IntegrationType              domain.IntegrationType
	NewCreator                   func(deps domain.IntegrationDeps) domain.IntegrationCreator
	NewPollingEventHandler       func(deps domain.IntegrationDeps) domain.IntegrationPoller
	NewHTTPOAuthClientProvider   func(deps domain.IntegrationDeps) domain.HTTPOauthClientProvider
	NewHTTPDefaultClientProvider func(deps domain.IntegrationDeps) domain.HTTPDefaultClientProvider
	NewConnectionTester          func(deps domain.IntegrationDeps) domain.IntegrationConnectionTester
}

var integrationRegisterParams = []IntegrationRegisterParams{}

type RegisterIntegrationParams struct {
	IntegrationSelector domain.IntegrationSelector
	Deps                domain.IntegrationDeps
}

func RegisterIntegrations(ctx context.Context,
	p RegisterIntegrationParams) error {
	integrationSelector := p.IntegrationSelector
	commonDeps := p.Deps

	for _, params := range integrationRegisterParams {

		if params.NewCreator != nil {
			log.Info().Msgf("Registering creator for %s", params.IntegrationType)

			creator := params.NewCreator(commonDeps)
			integrationSelector.RegisterCreator(params.IntegrationType, creator)
		}

		if params.NewPollingEventHandler != nil {
			log.Info().Msgf("Registering polling event handler for %s", params.IntegrationType)

			handler := params.NewPollingEventHandler(commonDeps)
			integrationSelector.RegisterPoller(params.IntegrationType, handler)
		}

		if params.NewHTTPOAuthClientProvider != nil {
			log.Info().Msgf("Registering http oauth client provider for %s", params.IntegrationType)

			httpOauthClientProvider := params.NewHTTPOAuthClientProvider(commonDeps)
			integrationSelector.RegisterHTTPOAuthClientProvider(params.IntegrationType, httpOauthClientProvider)
		}

		if params.NewHTTPDefaultClientProvider != nil {
			log.Info().Msgf("Registering http default client provider for %s", params.IntegrationType)

			httpDefaultClientProvider := params.NewHTTPDefaultClientProvider(commonDeps)
			integrationSelector.RegisterHTTPDefaultClientProvider(params.IntegrationType, httpDefaultClientProvider)
		}

		if params.NewConnectionTester != nil {
			log.Info().Msgf("Registering connection tester for %s", params.IntegrationType)

			connectionTester := params.NewConnectionTester(commonDeps)
			integrationSelector.RegisterConnectionTester(params.IntegrationType, connectionTester)
		}
	}

	return nil
}
