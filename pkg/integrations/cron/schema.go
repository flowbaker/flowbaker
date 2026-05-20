package cronintegration

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationTriggerType_Cron   domain.IntegrationTriggerEventType = "cron"
	IntegrationTriggerType_Simple domain.IntegrationTriggerEventType = "simple"
)

var (
	Schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Cron,
		Name:                 "Schedule",
		Description:          "Schedule workflows to run at specific times.",
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
								Label: "Second",
								Value: "second",
							},
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
						Key:         "second",
						Name:        "Second Value",
						Description: "The second interval value to trigger the workflow (min 10)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
						DependsOn: &domain.DependsOn{
							PropertyKey: "interval",
							Value:       "second",
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
				Description: "Triggered when the cron schedule is met.",
				Properties: []domain.NodeProperty{
					{
						Key:         "cron",
						Name:        "Cron",
						Description: "Standard 5-field cron expression (minute hour day-of-month month day-of-week).",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						RegexKey:    "cron",
					},
					{
						Key:         "timezone",
						Name:        "Timezone",
						Description: "UTC offset for cron interpretation. Defaults to UTC (server default). Fixed offset — daylight saving not applied.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "UTC", Value: "0"},
							{Label: "UTC-12", Value: "-12"},
							{Label: "UTC-11", Value: "-11"},
							{Label: "UTC-10", Value: "-10"},
							{Label: "UTC-9", Value: "-9"},
							{Label: "UTC-8", Value: "-8"},
							{Label: "UTC-7", Value: "-7"},
							{Label: "UTC-6", Value: "-6"},
							{Label: "UTC-5", Value: "-5"},
							{Label: "UTC-4", Value: "-4"},
							{Label: "UTC-3", Value: "-3"},
							{Label: "UTC-2", Value: "-2"},
							{Label: "UTC-1", Value: "-1"},
							{Label: "UTC+1", Value: "1"},
							{Label: "UTC+2", Value: "2"},
							{Label: "UTC+3", Value: "3"},
							{Label: "UTC+4", Value: "4"},
							{Label: "UTC+5", Value: "5"},
							{Label: "UTC+6", Value: "6"},
							{Label: "UTC+7", Value: "7"},
							{Label: "UTC+8", Value: "8"},
							{Label: "UTC+9", Value: "9"},
							{Label: "UTC+10", Value: "10"},
							{Label: "UTC+11", Value: "11"},
							{Label: "UTC+12", Value: "12"},
							{Label: "UTC+13", Value: "13"},
							{Label: "UTC+14", Value: "14"},
						},
					},
				},
			},
		},
	}
)
