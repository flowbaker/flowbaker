package sleep

import "github.com/flowbaker/flowbaker/pkg/domain"

const (
	SleepActionType_Sleep domain.IntegrationActionType = "sleep"

	DurationUnitSeconds = "seconds"
	DurationUnitMinutes = "minutes"
	DurationUnitHours   = "hours"
	DurationUnitDays    = "days"
)

var Schema = domain.Integration{
	ID:                   domain.IntegrationType_Sleep,
	Name:                 "Sleep",
	Description:          "Pause workflow execution for a specified duration",
	CredentialProperties: []domain.NodeProperty{},
	Triggers:             []domain.IntegrationTrigger{},
	Actions: []domain.IntegrationAction{
		{
			ID:          string(SleepActionType_Sleep),
			Name:        "Sleep",
			ActionType:  SleepActionType_Sleep,
			Description: "Pause workflow until duration elapses",
			Properties: []domain.NodeProperty{
				{
					Key:         "duration_value",
					Name:        "Duration",
					Description: "How long to sleep",
					Required:    true,
					Type:        domain.NodePropertyType_Integer,
					Default:     1,
				},
				{
					Key:         "duration_unit",
					Name:        "Unit",
					Description: "Time unit",
					Required:    true,
					Type:        domain.NodePropertyType_String,
					Options: []domain.NodePropertyOption{
						{Label: "Seconds", Value: DurationUnitSeconds},
						{Label: "Minutes", Value: DurationUnitMinutes},
						{Label: "Hours", Value: DurationUnitHours},
						{Label: "Days", Value: DurationUnitDays},
					},
					Default: DurationUnitMinutes,
				},
			},
		},
	},
}
