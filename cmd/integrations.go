package main

import (
	"context"
	"flowbaker/internal/domain"
	"flowbaker/pkg/integrations/ai_agent"
	claudeintegration "flowbaker/pkg/integrations/claude"
	"flowbaker/pkg/integrations/condition"
	cronintegration "flowbaker/pkg/integrations/cron"
	"flowbaker/pkg/integrations/discord"
	"flowbaker/pkg/integrations/dropbox"
	"flowbaker/pkg/integrations/flowbaker_agent_memory"
	githubintegration "flowbaker/pkg/integrations/github"
	"flowbaker/pkg/integrations/google/gmail"
	googledrive "flowbaker/pkg/integrations/google/google_drive"
	googlesheets "flowbaker/pkg/integrations/google/google_sheets"
	"flowbaker/pkg/integrations/google/youtube"
	"flowbaker/pkg/integrations/http"
	"flowbaker/pkg/integrations/jira"
	jwtintegration "flowbaker/pkg/integrations/jwt"
	"flowbaker/pkg/integrations/knowledge"
	"flowbaker/pkg/integrations/linear"
	mongodb "flowbaker/pkg/integrations/mongo"
	"flowbaker/pkg/integrations/openai"
	"flowbaker/pkg/integrations/postgresql"
	"flowbaker/pkg/integrations/redis"
	resendintegration "flowbaker/pkg/integrations/resend"
	s3integration "flowbaker/pkg/integrations/s3"
	sendresponse "flowbaker/pkg/integrations/send_response"
	slackintegration "flowbaker/pkg/integrations/slack"
	"flowbaker/pkg/integrations/storage"
	"flowbaker/pkg/integrations/stripe"

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

var integrationRegisterParams = []IntegrationRegisterParams{
	{
		IntegrationType:        domain.IntegrationType_Discord,
		NewCreator:             discord.NewDiscordIntegrationCreator,
		NewPollingEventHandler: discord.NewDiscordPollingHandler,
		NewConnectionTester:    discord.NewDiscordConnectionTester,
	},
	{
		IntegrationType: domain.IntegrationType_HTTP,
		NewCreator:      http.NewHTTPIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_PostgreSQL,
		NewCreator:      postgresql.NewPostgreSQLIntegrationCreator,
	},
	{
		IntegrationType:     domain.IntegrationType_Stripe,
		NewCreator:          stripe.NewStripeIntegrationCreator,
		NewConnectionTester: stripe.NewStripeConnectionTester,
	},
	{
		IntegrationType: domain.IntegrationType_MongoDB,
		NewCreator:      mongodb.NewMongoDBIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_OpenAI,
		NewCreator:      openai.NewOpenAIIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_Anthropic,
		NewCreator:      claudeintegration.NewClaudeIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_AIAgent,
		NewCreator:      ai_agent.NewAIAgentCreator,
	},
	{
		IntegrationType: domain.IntegrationType_FlowbakerStorage,
		NewCreator:      storage.NewStorageIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_FlowbakerAgentMemory,
		NewCreator:      flowbaker_agent_memory.NewFlowbakerAgentMemoryIntegrationCreator,
	},
	{
		IntegrationType:            domain.IntegrationType_Dropbox,
		NewCreator:                 dropbox.NewDropboxIntegrationCreator,
		NewHTTPOAuthClientProvider: dropbox.NewDropboxHTTPClientProvider,
	},
	{
		IntegrationType:            domain.IntegrationType_Gmail,
		NewCreator:                 gmail.NewGmailIntegrationCreator,
		NewHTTPOAuthClientProvider: gmail.NewGmailHTTPClientProvider,
		NewPollingEventHandler:     gmail.NewGmailPollingHandler,
	},
	{
		IntegrationType: domain.IntegrationType_GoogleSheets,
		NewCreator:      googlesheets.NewGoogleSheetsIntegrationCreator,
	},
	{
		IntegrationType:     domain.IntegrationType_Drive,
		NewCreator:          googledrive.NewGoogleDriveIntegrationCreator,
		NewConnectionTester: googledrive.NewGoogleDriveConnectionTester,
	},
	{
		IntegrationType: domain.IntegrationType_Youtube,
		NewCreator:      youtube.NewYoutubeIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_JWT,
		NewCreator:      jwtintegration.NewJWTIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_Knowledge,
		NewCreator:      knowledge.NewKnowledgeIntegrationCreator,
	},
	{
		IntegrationType:     domain.IntegrationType_Linear,
		NewCreator:          linear.NewLinearIntegrationCreator,
		NewConnectionTester: linear.NewLinearConnectionTester,
	},
	{
		IntegrationType:     domain.IntegrationType_Jira,
		NewCreator:          jira.NewJiraIntegrationCreator,
		NewConnectionTester: jira.NewJiraConnectionTester,
	},
	{
		IntegrationType:     domain.IntegrationType_Redis,
		NewCreator:          redis.NewRedisIntegrationCreator,
		NewConnectionTester: redis.NewRedisConnectionTester,
	},
	{
		IntegrationType: domain.IntegrationType_Slack,
		NewCreator:      slackintegration.NewSlackIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_Resend,
		NewCreator:      resendintegration.NewResendIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_AwsS3,
		NewCreator:      s3integration.NewS3IntegrationCreator,
	},
	{
		IntegrationType:     domain.IntegrationType_Github,
		NewCreator:          githubintegration.NewGithubIntegrationCreator,
		NewConnectionTester: githubintegration.NewGitHubConnectionTester,
	},
	{
		IntegrationType: domain.IntegrationType_SendResponse,
		NewCreator:      sendresponse.NewSendResponseIntegrationCreator,
	},
	{
		IntegrationType:        domain.IntegrationType_Cron,
		NewPollingEventHandler: cronintegration.NewCronPollingHandler,
	},
	{
		IntegrationType: domain.IntegrationType_Condition,
		NewCreator:      condition.NewConditionIntegrationCreator,
	},
}

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
