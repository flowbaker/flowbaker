package input

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationTriggerType_InputSubmit domain.IntegrationTriggerEventType = "input_submit"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_InputTrigger,
		Name:        "Input Trigger",
		Description: "A manual trigger with key-value inputs that can be used as form submission in a workflow",
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "on_submit",
				Name:        "Input Submit",
				EventType:   IntegrationTriggerType_InputSubmit,
				Description: "Triggered when the input form is submitted",
				Decoration: domain.NodeDecoration{
					HasButton: true,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "field_definitions",
						Name:        "Fields",
						Description: "Define the input fields for this trigger",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 0,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:               "label",
									Name:              "Label",
									Description:       "Display name shown to user",
									Type:              domain.NodePropertyType_String,
									Required:          true,
									DisableExpression: true,
								},
								{
									Key:               "key",
									Name:              "Key",
									Description:       "Field key for data submission (uses label if empty)",
									Type:              domain.NodePropertyType_String,
									Placeholder:       "Uses label if empty",
									DisableExpression: true,
								},
								{
									Key:               "type",
									Name:              "Type",
									Description:       "The type of input field",
									Type:              domain.NodePropertyType_String,
									Default:           "string",
									DisableExpression: true,
									Options: []domain.NodePropertyOption{
										{Label: "Text", Value: "string", Description: "Single-line text input"},
										{Label: "Number", Value: "number", Description: "Numeric input"},
										{Label: "Date", Value: "date", Description: "Date picker (ISO 8601)"},
										{Label: "Select", Value: "select", Description: "Dropdown selection"},
									},
								},
								{
									Key:               "options",
									Name:              "Options",
									Description:       "Options for the dropdown (comma-separated or one per line)",
									Type:              domain.NodePropertyType_TagInput,
									DisableExpression: true,
									ShowIf: &domain.ShowIf{
										PropertyKey: "type",
										Values:      []any{"select"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
)
