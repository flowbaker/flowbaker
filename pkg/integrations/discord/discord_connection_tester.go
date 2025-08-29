package discord

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

type DiscordConnectionTester struct {
	credentialGetter domain.CredentialGetter[DiscordCredential]
}

func NewDiscordConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	credentialGetter := managers.NewExecutorCredentialGetter[DiscordCredential](deps.ExecutorCredentialManager)

	return &DiscordConnectionTester{
		credentialGetter: credentialGetter,
	}
}

func (c *DiscordConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	data, err := json.Marshal(params.Credential.DecryptedPayload)
	if err != nil {
		return false, err
	}

	var discordCredential DiscordCredential

	if err := json.Unmarshal(data, &discordCredential); err != nil {
		return false, err
	}

	log.Info().Msgf("Testing connection to Discord with token: %s", discordCredential.Token)

	session, err := discordgo.New(fmt.Sprintf("Bot %s", discordCredential.Token))
	if err != nil {
		return false, err
	}

	_, err = session.User("@me")
	if err != nil {
		return false, err
	}

	return true, nil
}
