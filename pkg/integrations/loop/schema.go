package loop

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationContainerType_ForEach domain.IntegrationContainerType = "for_each"
)

var (
	Schema = schema

	schema = domain.Integration{
		ID:                   domain.IntegrationType_Loop,
		Name:                 "Loop",
		Description:          "Iterate over a set of actions until a condition is met",
		IsCredentialOptional: true,
		Containers: []domain.IntegrationContainer{
			{
				ID:            "for_each",
				ContainerType: IntegrationContainerType_ForEach,
				Name:          "For Each",
				Description:   "Iterate over each item in the input array",
				Properties: []domain.NodeProperty{
					{
						Key:         "max_iterations",
						Name:        "Max Iterations",
						Description: "Maximum number of loop iterations",
						Required:    true,
						Type:        domain.NodePropertyType_Number,
						Default:     100,
					},
					{
						Key:         "delay_ms",
						Name:        "Delay (ms)",
						Description: "Delay in milliseconds between iterations",
						Required:    false,
						Type:        domain.NodePropertyType_Number,
						Default:     0,
					},
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Index: 0, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input", UsageContext: domain.UsageContextWorkflow},
						},
						Output: []domain.NodeHandle{
							{Index: 0, Type: domain.NodeHandleTypeSuccess, Position: domain.NodeHandlePositionBottom, Text: "Output", UsageContext: domain.UsageContextWorkflow},
						},
					},
				},
				Controls: []domain.ContainerControl{
					{
						ID:       "start",
						Role:     domain.ContainerControlRoleEntrypoint,
						Label:    "Start",
						Subtitle: "Loop",
						Handles: []domain.NodeHandle{
							{Index: 0, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionBottom},
						},
					},
					{
						ID:       "continue",
						Role:     domain.ContainerControlRoleFeedback,
						Label:    "Continue",
						Subtitle: "Next iteration",
						Handles: []domain.NodeHandle{
							{Index: 0, Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop},
						},
					},
					{
						ID:       "exit",
						Role:     domain.ContainerControlRoleTerminal,
						Label:    "Exit",
						Subtitle: "End loop",
						Handles: []domain.NodeHandle{
							{Index: 0, Type: domain.NodeHandleTypeSuccess, Position: domain.NodeHandlePositionTop},
						},
					},
				},
			},
		},
	}
)
