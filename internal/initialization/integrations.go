package initialization

import (
	router "github.com/flowbaker/flowbaker/pkg/integrations/content_classifier"
	"github.com/flowbaker/flowbaker/pkg/integrations/items_to_item"
	"github.com/flowbaker/flowbaker/pkg/integrations/transform"

	"github.com/flowbaker/flowbaker/pkg/integrations/ai_agent"
	"github.com/flowbaker/flowbaker/pkg/integrations/base64"
	"github.com/flowbaker/flowbaker/pkg/integrations/brightdata"
	claudeintegration "github.com/flowbaker/flowbaker/pkg/integrations/claude"
	"github.com/flowbaker/flowbaker/pkg/integrations/condition"
	cronintegration "github.com/flowbaker/flowbaker/pkg/integrations/cron"
	"github.com/flowbaker/flowbaker/pkg/integrations/discord"
	"github.com/flowbaker/flowbaker/pkg/integrations/dropbox"
	"github.com/flowbaker/flowbaker/pkg/integrations/rawfiletoitem"
	githubintegration "github.com/flowbaker/flowbaker/pkg/integrations/github"
	gitlabintegration "github.com/flowbaker/flowbaker/pkg/integrations/gitlab"
	"github.com/flowbaker/flowbaker/pkg/integrations/google/gmail"
	googledrive "github.com/flowbaker/flowbaker/pkg/integrations/google/google_drive"
	googlesheets "github.com/flowbaker/flowbaker/pkg/integrations/google/google_sheets"
	"github.com/flowbaker/flowbaker/pkg/integrations/google/youtube"
	"github.com/flowbaker/flowbaker/pkg/integrations/http"
	"github.com/flowbaker/flowbaker/pkg/integrations/jira"
	jwtintegration "github.com/flowbaker/flowbaker/pkg/integrations/jwt"
	"github.com/flowbaker/flowbaker/pkg/integrations/knowledge"
	"github.com/flowbaker/flowbaker/pkg/integrations/linear"
	"github.com/flowbaker/flowbaker/pkg/integrations/manipulation"
	mongodb "github.com/flowbaker/flowbaker/pkg/integrations/mongo"
	notionintegration "github.com/flowbaker/flowbaker/pkg/integrations/notion"
	"github.com/flowbaker/flowbaker/pkg/integrations/openai"
	pipedriveintegration "github.com/flowbaker/flowbaker/pkg/integrations/pipedrive"
	"github.com/flowbaker/flowbaker/pkg/integrations/postgresql"
	"github.com/flowbaker/flowbaker/pkg/integrations/redis"
	resendintegration "github.com/flowbaker/flowbaker/pkg/integrations/resend"
	s3integration "github.com/flowbaker/flowbaker/pkg/integrations/s3"
	sendresponse "github.com/flowbaker/flowbaker/pkg/integrations/send_response"
	slackintegration "github.com/flowbaker/flowbaker/pkg/integrations/slack"
	"github.com/flowbaker/flowbaker/pkg/integrations/split_array"
	startupswatchintegration "github.com/flowbaker/flowbaker/pkg/integrations/startups_watch"
	"github.com/flowbaker/flowbaker/pkg/integrations/storage"
	"github.com/flowbaker/flowbaker/pkg/integrations/stripe"
	"github.com/flowbaker/flowbaker/pkg/integrations/teams"
	telegramintegration "github.com/flowbaker/flowbaker/pkg/integrations/telegram"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type integrationRegisterParams struct {
	IntegrationType              domain.IntegrationType
	NewCreator                   func(deps domain.IntegrationDeps) domain.IntegrationCreator
	NewPollingEventHandler       func(deps domain.IntegrationDeps) domain.IntegrationPoller
	NewHTTPOAuthClientProvider   func(deps domain.IntegrationDeps) domain.HTTPOauthClientProvider
	NewHTTPDefaultClientProvider func(deps domain.IntegrationDeps) domain.HTTPDefaultClientProvider
	NewConnectionTester          func(deps domain.IntegrationDeps) domain.IntegrationConnectionTester
}

var integrationRegisterParamsList = []integrationRegisterParams{
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
		IntegrationType:     domain.IntegrationType_Gitlab,
		NewCreator:          gitlabintegration.NewGitlabIntegrationCreator,
		NewConnectionTester: gitlabintegration.NewGitLabConnectionTester,
	},
	{
		IntegrationType:     domain.IntegrationType_Teams,
		NewCreator:          teams.NewTeamsIntegrationCreator,
		NewConnectionTester: teams.NewTeamsConnectionTester,
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
	{
		IntegrationType: domain.IntegrationType_Transform,
		NewCreator:      transform.NewTransformIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_Base64,
		NewCreator:      base64.NewBase64IntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_RawFileToItem,
		NewCreator:      rawfiletoitem.NewRawFileToItemIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_BrightData,
		NewCreator:      brightdata.NewBrightDataIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_ContentClassifier,
		NewCreator:      router.NewRouterIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_Manipulation,
		NewCreator:      manipulation.NewManipulationIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_SplitArray,
		NewCreator:      split_array.NewSplitArrayIntegrationCreator,
	},
	{
		IntegrationType:        domain.IntegrationType_StartupsWatch,
		NewCreator:             startupswatchintegration.NewStartupsWatchIntegrationCreator,
		NewPollingEventHandler: startupswatchintegration.NewStartupsWatchPollingHandler,
	},
	{
		IntegrationType: domain.IntegrationType_Pipedrive,
		NewCreator:      pipedriveintegration.NewPipedriveIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_ItemsToItem,
		NewCreator:      items_to_item.NewItemsToItemIntegrationCreator,
	},
	{
		IntegrationType: domain.IntegrationType_Telegram,
		NewCreator:      telegramintegration.NewTelegramIntegrationCreator,
	},
	{
		IntegrationType:     domain.IntegrationType_Notion,
		NewCreator:          notionintegration.NewNotionIntegrationCreator,
		NewConnectionTester: notionintegration.NewNotionConnectionTester,
	},
}

func registerIntegrations(integrationSelector domain.IntegrationSelector, commonDeps domain.IntegrationDeps) error {
	for _, params := range integrationRegisterParamsList {

		if params.NewCreator != nil {
			creator := params.NewCreator(commonDeps)
			integrationSelector.RegisterCreator(params.IntegrationType, creator)
		}

		if params.NewPollingEventHandler != nil {
			handler := params.NewPollingEventHandler(commonDeps)
			integrationSelector.RegisterPoller(params.IntegrationType, handler)
		}

		if params.NewHTTPOAuthClientProvider != nil {
			httpOauthClientProvider := params.NewHTTPOAuthClientProvider(commonDeps)
			integrationSelector.RegisterHTTPOAuthClientProvider(params.IntegrationType, httpOauthClientProvider)
		}

		if params.NewHTTPDefaultClientProvider != nil {
			httpDefaultClientProvider := params.NewHTTPDefaultClientProvider(commonDeps)
			integrationSelector.RegisterHTTPDefaultClientProvider(params.IntegrationType, httpDefaultClientProvider)
		}

		if params.NewConnectionTester != nil {
			connectionTester := params.NewConnectionTester(commonDeps)
			integrationSelector.RegisterConnectionTester(params.IntegrationType, connectionTester)
		}
	}

	return nil
}
