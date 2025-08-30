package condition

import (
	"github.com/flowbaker/flowbaker/internal/domain"
)

const (
	IntegrationActionType_IfStreams domain.IntegrationActionType = "if_streams"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Condition,
		Name:        "Condition",
		Description: "Condition node to evaluate a condition and return a boolean result",
		Actions: []domain.IntegrationAction{
			{
				ID:         "if_streams",
				Name:       "If Streams",
				ActionType: IntegrationActionType_IfStreams,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Position: domain.NodeHandlePositionTop, Text: "Input"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeSuccess, Text: "True"},
							{Type: domain.NodeHandleTypeDestructive, Text: "False"},
						},
					},
				},
				Description: "Evaluate a condition and return a boolean result",
				Properties: []domain.NodeProperty{
					{
						Key:         "relation_type",
						Name:        "Relation Type",
						Description: "The relation type to evaluate the condition",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "And", Value: "and"},
							{Label: "Or", Value: "or"},
						},
					},
					{
						Key:         "conditions",
						Name:        "Conditions",
						Description: "The conditions to evaluate",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 10,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "condition_type",
									Name:        "Condition Type",
									Description: "The condition type to evaluate",
									Required:    true,
									Type:        domain.NodePropertyType_String,
									MultipleOpts: []domain.MultipleNodePropertyOption{
										{
											Label: "String",
											Value: "string",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: "exists"},
												{Label: "Does Not Exist", Value: "does_not_exist"},
												{Label: "Is Empty", Value: "is_empty"},
												{Label: "Is Not Empty", Value: "is_not_empty"},
												{Label: "Equals", Value: "is_equal"},
												{Label: "Not Equals", Value: "is_not_equal"},
												{Label: "Contains", Value: "contains"},
												{Label: "Does Not Contain", Value: "does_not_contain"},
												{Label: "Starts With", Value: "starts_with"},
												{Label: "Ends With", Value: "ends_with"},
												{Label: "Does Not Start With", Value: "does_not_start_with"},
												{Label: "Does Not End With", Value: "does_not_end_with"},
												{Label: "Matches Regex", Value: "matches_regex"},
												{Label: "Does Not Match Regex", Value: "does_not_match_regex"},
											},
										},
										{
											Label: "Number",
											Value: "number",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: "exists"},
												{Label: "Does Not Exist", Value: "does_not_exist"},
												{Label: "Equals", Value: "is_equal"},
												{Label: "Not Equals", Value: "is_not_equal"},
												{Label: "Greater Than", Value: "is_greater_than"},
												{Label: "Less Than", Value: "is_less_than"},
												{Label: "Greater Than or Equal", Value: "is_greater_than_or_equal"},
												{Label: "Less Than or Equal", Value: "is_less_than_or_equal"},
											},
										},
										{
											Label: "Boolean",
											Value: "boolean",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: "exists"},
												{Label: "Does Not Exist", Value: "does_not_exist"},
												{Label: "Equals", Value: "is_equal"},
												{Label: "Not Equals", Value: "is_not_equal"},
												{Label: "Is True", Value: "is_true"},
												{Label: "Is False", Value: "is_false"},
											},
										},
										{
											Label: "Date",
											Value: "date",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: "exists"},
												{Label: "Does Not Exist", Value: "does_not_exist"},
												{Label: "Is Equal", Value: "is_equal"},
												{Label: "Is Not Equal", Value: "is_not_equal"},
												{Label: "Is After", Value: "is_after"},
												{Label: "Is Before", Value: "is_before"},
												{Label: "Is After or Equal", Value: "is_after_or_equal"},
												{Label: "Is Before or Equal", Value: "is_before_or_equal"},
											},
										},
										{
											Label: "Array",
											Value: "array",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: "exists"},
												{Label: "Does Not Exist", Value: "does_not_exist"},
												{Label: "Is Empty", Value: "is_empty"},
												{Label: "Is Not Empty", Value: "is_not_empty"},
												{Label: "Contains", Value: "contains"},
												{Label: "Does Not Contain", Value: "does_not_contain"},
												{Label: "Length Equals", Value: "length_equals"},
												{Label: "Length Greater Than", Value: "length_greater_than"},
												{Label: "Length Less Than", Value: "length_less_than"},
											},
										},
										{
											Label: "Object",
											Value: "object",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: "exists"},
												{Label: "Does Not Exist", Value: "does_not_exist"},
												{Label: "Has Key", Value: "has_key"},
												{Label: "Does Not Have Key", Value: "does_not_have_key"},
												{Label: "Key Equals", Value: "key_equals"},
												{Label: "Key Not Equals", Value: "key_not_equals"},
											},
										},
									},
								},
								{
									Key:         "condition1",
									Name:        "Condition 1",
									Description: "The first condition to evaluate",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
								{
									Key:         "condition2",
									Name:        "Condition 2",
									Description: "The second condition to evaluate",
									Required:    true,
									Type:        domain.NodePropertyType_String,
									HideIf: &domain.HideIf{
										PropertyKey: "condition_type",
										Values:      []any{"string.exists", "string.does_not_exist", "string.is_empty", "string.is_not_empty", "number.exists", "number.does_not_exist", "boolean.exists", "boolean.does_not_exist", "date.exists", "date.does_not_exist", "array.exists", "array.does_not_exist", "object.exists", "object.does_not_exist"},
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
