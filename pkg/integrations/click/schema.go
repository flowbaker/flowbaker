package click

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationTriggerType_Click = "click"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Click,
		Name:        "Click Trigger",
		Description: "A click trigger that can be used to start flows manually",
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "on_click",
				Name:        "On Click",
				EventType:   IntegrationTriggerType_Click,
				Description: "Triggered when clicked",
				Decoration: domain.TriggerNodeDecoration{
					HasButton:        true,
					DoesNotHasEditor: true,
				},
			},
		},
	}
)
