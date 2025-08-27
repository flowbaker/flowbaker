package domain

import "time"

type Schedule struct {
	ID                           string
	WorkflowID                   string
	ScheduleCreatedAt            time.Time
	TriggerID                    string
	UserID                       string
	WorkflowType                 WorkflowType
	IntegrationType              IntegrationType
	LastCheckedAt                time.Time
	NextScheduledCheckAt         time.Time
	IsActive                     bool
	LastModifiedData             string
	PollingScheduleGap_AsSeconds int
}
