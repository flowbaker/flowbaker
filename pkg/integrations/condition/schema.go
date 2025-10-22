package condition

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_IfStreams domain.IntegrationActionType = "if_streams"
	IntegrationActionType_Switch    domain.IntegrationActionType = "switch"
)

// String condition types
type ConditionTypeString string

const (
	ConditionTypeString_Exists            ConditionTypeString = "exists"
	ConditionTypeString_DoesNotExist      ConditionTypeString = "does_not_exist"
	ConditionTypeString_IsEmpty           ConditionTypeString = "is_empty"
	ConditionTypeString_IsNotEmpty        ConditionTypeString = "is_not_empty"
	ConditionTypeString_IsEqual           ConditionTypeString = "is_equal"
	ConditionTypeString_IsNotEqual        ConditionTypeString = "is_not_equal"
	ConditionTypeString_Contains          ConditionTypeString = "contains"
	ConditionTypeString_DoesNotContain    ConditionTypeString = "does_not_contain"
	ConditionTypeString_StartsWith        ConditionTypeString = "starts_with"
	ConditionTypeString_EndsWith          ConditionTypeString = "ends_with"
	ConditionTypeString_DoesNotStartWith  ConditionTypeString = "does_not_start_with"
	ConditionTypeString_DoesNotEndWith    ConditionTypeString = "does_not_end_with"
	ConditionTypeString_MatchesRegex      ConditionTypeString = "matches_regex"
	ConditionTypeString_DoesNotMatchRegex ConditionTypeString = "does_not_match_regex"
)

// Number condition types
type ConditionTypeNumber string

const (
	ConditionTypeNumber_Exists               ConditionTypeNumber = "exists"
	ConditionTypeNumber_DoesNotExist         ConditionTypeNumber = "does_not_exist"
	ConditionTypeNumber_IsEqual              ConditionTypeNumber = "is_equal"
	ConditionTypeNumber_IsNotEqual           ConditionTypeNumber = "is_not_equal"
	ConditionTypeNumber_IsGreaterThan        ConditionTypeNumber = "is_greater_than"
	ConditionTypeNumber_IsLessThan           ConditionTypeNumber = "is_less_than"
	ConditionTypeNumber_IsGreaterThanOrEqual ConditionTypeNumber = "is_greater_than_or_equal"
	ConditionTypeNumber_IsLessThanOrEqual    ConditionTypeNumber = "is_less_than_or_equal"
)

// Boolean condition types
type ConditionTypeBoolean string

const (
	ConditionTypeBoolean_Exists       ConditionTypeBoolean = "exists"
	ConditionTypeBoolean_DoesNotExist ConditionTypeBoolean = "does_not_exist"
	ConditionTypeBoolean_IsEqual      ConditionTypeBoolean = "is_equal"
	ConditionTypeBoolean_IsNotEqual   ConditionTypeBoolean = "is_not_equal"
	ConditionTypeBoolean_IsTrue       ConditionTypeBoolean = "is_true"
	ConditionTypeBoolean_IsFalse      ConditionTypeBoolean = "is_false"
)

// Date condition types
type ConditionTypeDate string

const (
	ConditionTypeDate_Exists          ConditionTypeDate = "exists"
	ConditionTypeDate_DoesNotExist    ConditionTypeDate = "does_not_exist"
	ConditionTypeDate_IsEqual         ConditionTypeDate = "is_equal"
	ConditionTypeDate_IsNotEqual      ConditionTypeDate = "is_not_equal"
	ConditionTypeDate_IsAfter         ConditionTypeDate = "is_after"
	ConditionTypeDate_IsBefore        ConditionTypeDate = "is_before"
	ConditionTypeDate_IsAfterOrEqual  ConditionTypeDate = "is_after_or_equal"
	ConditionTypeDate_IsBeforeOrEqual ConditionTypeDate = "is_before_or_equal"
)

// Array condition types
type ConditionTypeArray string

const (
	ConditionTypeArray_Exists            ConditionTypeArray = "exists"
	ConditionTypeArray_DoesNotExist      ConditionTypeArray = "does_not_exist"
	ConditionTypeArray_IsEmpty           ConditionTypeArray = "is_empty"
	ConditionTypeArray_IsNotEmpty        ConditionTypeArray = "is_not_empty"
	ConditionTypeArray_Contains          ConditionTypeArray = "contains"
	ConditionTypeArray_DoesNotContain    ConditionTypeArray = "does_not_contain"
	ConditionTypeArray_LengthEquals      ConditionTypeArray = "length_equals"
	ConditionTypeArray_LengthGreaterThan ConditionTypeArray = "length_greater_than"
	ConditionTypeArray_LengthLessThan    ConditionTypeArray = "length_less_than"
)

// Object condition types
type ConditionTypeObject string

const (
	ConditionTypeObject_Exists         ConditionTypeObject = "exists"
	ConditionTypeObject_DoesNotExist   ConditionTypeObject = "does_not_exist"
	ConditionTypeObject_HasKey         ConditionTypeObject = "has_key"
	ConditionTypeObject_DoesNotHaveKey ConditionTypeObject = "does_not_have_key"
	ConditionTypeObject_KeyEquals      ConditionTypeObject = "key_equals"
	ConditionTypeObject_KeyNotEquals   ConditionTypeObject = "key_not_equals"
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
									Key:         "value1",
									Name:        "Value",
									Description: "The first value to evaluate",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
								{
									Key:         "value2",
									Name:        "Value",
									Description: "The second value to evaluate",
									Required:    true,
									Type:        domain.NodePropertyType_String,
									HideIf: &domain.HideIf{
										PropertyKey: "condition_type",
										Values: []any{
											"string." + string(ConditionTypeString_Exists),
											"string." + string(ConditionTypeString_DoesNotExist),
											"string." + string(ConditionTypeString_IsEmpty),
											"string." + string(ConditionTypeString_IsNotEmpty),
											"number." + string(ConditionTypeNumber_Exists),
											"number." + string(ConditionTypeNumber_DoesNotExist),
											"boolean." + string(ConditionTypeBoolean_Exists),
											"boolean." + string(ConditionTypeBoolean_DoesNotExist),
											"date." + string(ConditionTypeDate_Exists),
											"date." + string(ConditionTypeDate_DoesNotExist),
											"array." + string(ConditionTypeArray_Exists),
											"array." + string(ConditionTypeArray_DoesNotExist),
											"object." + string(ConditionTypeObject_Exists),
											"object." + string(ConditionTypeObject_DoesNotExist),
										},
									},
								},
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
												{Label: "Exists", Value: string(ConditionTypeString_Exists)},
												{Label: "Does Not Exist", Value: string(ConditionTypeString_DoesNotExist)},
												{Label: "Is Empty", Value: string(ConditionTypeString_IsEmpty)},
												{Label: "Is Not Empty", Value: string(ConditionTypeString_IsNotEmpty)},
												{Label: "Equals", Value: string(ConditionTypeString_IsEqual)},
												{Label: "Not Equals", Value: string(ConditionTypeString_IsNotEqual)},
												{Label: "Contains", Value: string(ConditionTypeString_Contains)},
												{Label: "Does Not Contain", Value: string(ConditionTypeString_DoesNotContain)},
												{Label: "Starts With", Value: string(ConditionTypeString_StartsWith)},
												{Label: "Ends With", Value: string(ConditionTypeString_EndsWith)},
												{Label: "Does Not Start With", Value: string(ConditionTypeString_DoesNotStartWith)},
												{Label: "Does Not End With", Value: string(ConditionTypeString_DoesNotEndWith)},
												{Label: "Matches Regex", Value: string(ConditionTypeString_MatchesRegex)},
												{Label: "Does Not Match Regex", Value: string(ConditionTypeString_DoesNotMatchRegex)},
											},
										},
										{
											Label: "Number",
											Value: "number",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: string(ConditionTypeNumber_Exists)},
												{Label: "Does Not Exist", Value: string(ConditionTypeNumber_DoesNotExist)},
												{Label: "Equals", Value: string(ConditionTypeNumber_IsEqual)},
												{Label: "Not Equals", Value: string(ConditionTypeNumber_IsNotEqual)},
												{Label: "Greater Than", Value: string(ConditionTypeNumber_IsGreaterThan)},
												{Label: "Less Than", Value: string(ConditionTypeNumber_IsLessThan)},
												{Label: "Greater Than or Equal", Value: string(ConditionTypeNumber_IsGreaterThanOrEqual)},
												{Label: "Less Than or Equal", Value: string(ConditionTypeNumber_IsLessThanOrEqual)},
											},
										},
										{
											Label: "Boolean",
											Value: "boolean",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: string(ConditionTypeBoolean_Exists)},
												{Label: "Does Not Exist", Value: string(ConditionTypeBoolean_DoesNotExist)},
												{Label: "Equals", Value: string(ConditionTypeBoolean_IsEqual)},
												{Label: "Not Equals", Value: string(ConditionTypeBoolean_IsNotEqual)},
												{Label: "Is True", Value: string(ConditionTypeBoolean_IsTrue)},
												{Label: "Is False", Value: string(ConditionTypeBoolean_IsFalse)},
											},
										},
										{
											Label: "Date",
											Value: "date",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: string(ConditionTypeDate_Exists)},
												{Label: "Does Not Exist", Value: string(ConditionTypeDate_DoesNotExist)},
												{Label: "Is Equal", Value: string(ConditionTypeDate_IsEqual)},
												{Label: "Is Not Equal", Value: string(ConditionTypeDate_IsNotEqual)},
												{Label: "Is After", Value: string(ConditionTypeDate_IsAfter)},
												{Label: "Is Before", Value: string(ConditionTypeDate_IsBefore)},
												{Label: "Is After or Equal", Value: string(ConditionTypeDate_IsAfterOrEqual)},
												{Label: "Is Before or Equal", Value: string(ConditionTypeDate_IsBeforeOrEqual)},
											},
										},
										{
											Label: "Array",
											Value: "array",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: string(ConditionTypeArray_Exists)},
												{Label: "Does Not Exist", Value: string(ConditionTypeArray_DoesNotExist)},
												{Label: "Is Empty", Value: string(ConditionTypeArray_IsEmpty)},
												{Label: "Is Not Empty", Value: string(ConditionTypeArray_IsNotEmpty)},
												{Label: "Contains", Value: string(ConditionTypeArray_Contains)},
												{Label: "Does Not Contain", Value: string(ConditionTypeArray_DoesNotContain)},
												{Label: "Length Equals", Value: string(ConditionTypeArray_LengthEquals)},
												{Label: "Length Greater Than", Value: string(ConditionTypeArray_LengthGreaterThan)},
												{Label: "Length Less Than", Value: string(ConditionTypeArray_LengthLessThan)},
											},
										},
										{
											Label: "Object",
											Value: "object",
											SubNodeProperties: []domain.NodePropertyOption{
												{Label: "Exists", Value: string(ConditionTypeObject_Exists)},
												{Label: "Does Not Exist", Value: string(ConditionTypeObject_DoesNotExist)},
												{Label: "Has Key", Value: string(ConditionTypeObject_HasKey)},
												{Label: "Does Not Have Key", Value: string(ConditionTypeObject_DoesNotHaveKey)},
												{Label: "Key Equals", Value: string(ConditionTypeObject_KeyEquals)},
												{Label: "Key Not Equals", Value: string(ConditionTypeObject_KeyNotEquals)},
											},
										},
									},
								},
							},
						},
					},
				},
			},

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
						Key:         "value_type",
						Name:        "Value Type",
						Description: "The type of value to evaluate",
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
						Key:         "value_string",
						Name:        "Value",
						Description: "The string value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						DependsOn: &domain.DependsOn{
							PropertyKey: "value_type",
							Value:       "string",
						},
					},
					{
						Key:         "value_number",
						Name:        "Value",
						Description: "The number value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_Number,
						DependsOn: &domain.DependsOn{
							PropertyKey: "value_type",
							Value:       "number",
						},
					},
					{
						Key:         "value_boolean",
						Name:        "Value",
						Description: "The boolean value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_Boolean,
						DependsOn: &domain.DependsOn{
							PropertyKey: "value_type",
							Value:       "boolean",
						},
					},
					{
						Key:         "value_date",
						Name:        "Value",
						Description: "The date value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_Date,
						DependsOn: &domain.DependsOn{
							PropertyKey: "value_type",
							Value:       "date",
						},
					},
					{
						Key:         "value_array",
						Name:        "Value",
						Description: "The array value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
						DependsOn: &domain.DependsOn{
							PropertyKey: "value_type",
							Value:       "array",
						},
					},
					{
						Key:         "value_object",
						Name:        "Value",
						Description: "The object value to match the condition against",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
						DependsOn: &domain.DependsOn{
							PropertyKey: "value_type",
							Value:       "object",
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
									Name:        "Key",
									Description: "The name of the route",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},

								{
									Key:         "value_string",
									Name:        "Value",
									Description: "The type of value to execute if the condition is met",
									Type:        domain.NodePropertyType_Text,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "string",
									},
								},
								{
									Key:         "value_string_comparison",
									Name:        "Value Comparison",
									Description: "The comparison type to use for the string value",
									Required:    true,
									Type:        domain.NodePropertyType_String,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "string",
									},
									Options: []domain.NodePropertyOption{
										{Label: "Equals", Value: string(ConditionTypeString_IsEqual)},
										{Label: "Not Equals", Value: string(ConditionTypeString_IsNotEqual)},
										{Label: "Contains", Value: string(ConditionTypeString_Contains)},
										{Label: "Does Not Contain", Value: string(ConditionTypeString_DoesNotContain)},
									},
								},
								{
									Key:         "value_number",
									Name:        "Value",
									Description: "The number value to match the condition against",
									Type:        domain.NodePropertyType_Number,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "number",
									},
								},
								{
									Key:         "value_number_comparison",
									Name:        "Value Comparison",
									Description: "The comparison type to use for the number value",
									Required:    true,
									Type:        domain.NodePropertyType_String,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "number",
									},
									Options: []domain.NodePropertyOption{
										{Label: "Equals", Value: ConditionTypeNumber_IsEqual},
										{Label: "Not Equals", Value: ConditionTypeNumber_IsNotEqual},
										{Label: "Greater Than", Value: ConditionTypeNumber_IsGreaterThan},
										{Label: "Less Than", Value: ConditionTypeNumber_IsLessThan},
									},
								},
								{
									Key:         "value_boolean",
									Name:        "Value",
									Description: "The boolean value to match the condition against",
									Type:        domain.NodePropertyType_Boolean,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "boolean",
									},
								},
								{
									Key:         "value_date",
									Name:        "Value",
									Description: "The date value to match the condition against",
									Type:        domain.NodePropertyType_Date,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "date",
									},
								},
								{
									Key:         "value_array",
									Name:        "Value",
									Description: "The array value to match the condition against",
									Type:        domain.NodePropertyType_CodeEditor,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "array",
									},
								},
								{
									Key:         "value_object",
									Name:        "Value",
									Description: "The object value to match the condition against",
									Type:        domain.NodePropertyType_CodeEditor,
									Required:    true,
									DependsOn: &domain.DependsOn{
										PropertyKey: "value_type",
										Value:       "object",
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
