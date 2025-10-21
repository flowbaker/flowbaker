package switchintegration

import "github.com/flowbaker/flowbaker/pkg/domain"

const (
	IntegrationActionType_Switch domain.IntegrationActionType = "switch"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Switch,
		Name:                 "Switch",
		Description:          "Switch node to switch between different actions based on a condition",
		CanTestConnection:    false,
		IsCredentialOptional: true,
		CredentialProperties: []domain.NodeProperty{},
		Triggers:             []domain.IntegrationTrigger{},
		Actions: []domain.IntegrationAction{
			{
				ID:          "switch",
				Name:        "Switch",
				ActionType:  IntegrationActionType_Switch,
				Description: "Switch between different actions based on a condition", SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "condition_type",
						Name:        "Condition Type",
						Description: "The type of condition to evaluate",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "String", Value: "string"},
							{Label: "Number", Value: "number"},
							{Label: "Boolean", Value: "boolean"},
							{Label: "Date", Value: "date"},
							{Label: "Array", Value: "array"},
							{Label: "Object", Value: "object"},
							{Label: "Deep Equal", Value: "deep_equal"},
						},
					},
					{
						Key:         "condition_string",
						Name:        "Condition String",
						Description: "The string value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						DependsOn: &domain.DependsOn{
							PropertyKey: "condition_type",
							Value:       "string",
						},
					},
					{
						Key:         "condition_number",
						Name:        "Condition Number",
						Description: "The number value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_Number,
						DependsOn: &domain.DependsOn{
							PropertyKey: "condition_type",
							Value:       "number",
						},
					},
					{
						Key:         "condition_boolean",
						Name:        "Condition Boolean",
						Description: "The boolean value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_Boolean,
						DependsOn: &domain.DependsOn{
							PropertyKey: "condition_type",
							Value:       "boolean",
						},
					},
					{
						Key:         "condition_date",
						Name:        "Condition Date",
						Description: "The date value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_Date,
						DependsOn: &domain.DependsOn{
							PropertyKey: "condition_type",
							Value:       "date",
						},
					},
					{
						Key:         "condition_array",
						Name:        "Condition Array",
						Description: "The array value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
						DependsOn: &domain.DependsOn{
							PropertyKey: "condition_type",
							Value:       "array",
						},
					},
					{
						Key:         "condition_object",
						Name:        "Condition Object",
						Description: "The object value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
						DependsOn: &domain.DependsOn{
							PropertyKey: "condition_type",
							Value:       "object",
						},
					},
					{
						Key:         "condition_deep_equal",
						Name:        "Condition Deep Equal",
						Description: "The deep equal value to match the condition against. Any type of value can be used.",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						DependsOn: &domain.DependsOn{
							PropertyKey: "condition_type",
							Value:       "deep_equal",
						},
					},
					{
						Key:              "routes",
						Name:             "Routes",
						Description:      "The routes to switch to based on the condition",
						Required:         true,
						Type:             domain.NodePropertyType_Array,
						GeneratesHandles: true,
						HandleGenerationOpts: &domain.HandleGenerationOptions{
							HandleType:        "output",
							NameFromProperty:  "Name",
							DefaultHandleType: domain.NodeHandleTypeDefault,
							Position:          domain.NodeHandlePositionBottom,
						},
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "Name",
									Name:        "Name",
									Description: "The name of the route",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},

								{
									Key:         "route_string",
									Name:        "Route String",
									Description: "The type of route to execute if the condition is met",
									Type:        domain.NodePropertyType_String,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "condition_type",
										Value:       "string",
									},
								},
								{
									Key:         "route_number",
									Name:        "Route Number",
									Description: "The number value to match the route against",
									Type:        domain.NodePropertyType_Number,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "condition_type",
										Value:       "number",
									},
								},
								{
									Key:         "route_boolean",
									Name:        "Route Boolean",
									Description: "The boolean value to match the route against",
									Type:        domain.NodePropertyType_Boolean,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "condition_type",
										Value:       "boolean",
									},
								},
								{
									Key:         "route_date",
									Name:        "Route Date",
									Description: "The date value to match the route against",
									Type:        domain.NodePropertyType_Date,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "condition_type",
										Value:       "date",
									},
								},
								{
									Key:         "route_array",
									Name:        "Route Array",
									Description: "The array value to match the route against",
									Type:        domain.NodePropertyType_CodeEditor,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "condition_type",
										Value:       "array",
									},
								},
								{
									Key:         "route_object",
									Name:        "Route Object",
									Description: "The object value to match the route against",
									Type:        domain.NodePropertyType_CodeEditor,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "condition_type",
										Value:       "object",
									},
								},
								{
									Key:         "route_deep_equal",
									Name:        "Route Deep Equal",
									Description: "The deep equal value to match the route against. Any type of value can be used.",
									Type:        domain.NodePropertyType_String,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "condition_type",
										Value:       "deep_equal",
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
