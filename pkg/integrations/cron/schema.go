package cronintegration

import (
	"github.com/flowbaker/flowbaker/internal/domain"
)

const (
	IntegrationTriggerType_Cron   domain.IntegrationTriggerEventType = "cron"
	IntegrationTriggerType_Simple domain.IntegrationTriggerEventType = "simple"
)

var (
	Schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Cron,
		Name:                 "Schedule",
		Description:          "Schedule workflows to run at specific times",
		CredentialProperties: []domain.NodeProperty{},
		Actions:              []domain.IntegrationAction{},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "simple",
				Name:        "Simple",
				EventType:   IntegrationTriggerType_Simple,
				Description: "Triggered when the simple cron schedule is met",
				Properties: []domain.NodeProperty{
					{
						Key:         "interval",
						Name:        "Interval",
						Description: "The interval of the workflow",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{
								Label: "Minute",
								Value: "minute",
							},
							{
								Label: "Hour",
								Value: "hour",
							},
							{
								Label: "Day",
								Value: "day",
							},
						},
					},
					{
						Key:         "minute",
						Name:        "Minute Value",
						Description: "The minute interval value to trigger the workflow",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
						DependsOn: &domain.DependsOn{
							PropertyKey: "interval",
							Value:       "minute",
						},
					},
					{
						Key:         "hour",
						Name:        "Hour Value",
						Description: "The hour interval value to trigger the workflow",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
						DependsOn: &domain.DependsOn{
							PropertyKey: "interval",
							Value:       "hour",
						},
					},
					{
						Key:         "day",
						Name:        "Day Value",
						Description: "The day interval value to trigger the workflow",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
						DependsOn: &domain.DependsOn{
							PropertyKey: "interval",
							Value:       "day",
						},
					},
				},
			},
			{
				ID:          "cron",
				Name:        "Cron",
				EventType:   IntegrationTriggerType_Cron,
				Description: "Triggered when the cron schedule is met",
				Properties: []domain.NodeProperty{
					{
						Key:              "cron",
						Name:             "Cron",
						Description:      "The cron schedule",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						RegexKey:         "cron",
						ExpressionChoice: true,
					},
				},
			},
		},
	}
)
