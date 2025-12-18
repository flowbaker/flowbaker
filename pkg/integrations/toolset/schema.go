package toolset

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const ()

var (
	Schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Toolset,
		Name:                 "Toolset",
		Description:          "Give your AI Agent access to a set of tools to perform actions.",
		CredentialProperties: []domain.NodeProperty{},
		Actions:              []domain.IntegrationAction{},
		IsGroup:              true,
	}
)
