package transform

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_Append          domain.IntegrationActionType = "append"
	IntegrationActionType_InnerJoin       domain.IntegrationActionType = "inner_join"
	IntegrationActionType_OuterJoin       domain.IntegrationActionType = "outer_join"
	IntegrationActionType_LeftJoin        domain.IntegrationActionType = "left_join"
	IntegrationActionType_RightJoin       domain.IntegrationActionType = "right_join"
	IntegrationActionType_ExcludeMatching domain.IntegrationActionType = "exclude_matching"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Transform,
		Name:        "Transform",
		Description: "Transform node to merge multiple input streams into a single output stream",
		Actions: []domain.IntegrationAction{
			{
				ID:         string(IntegrationActionType_InnerJoin),
				Name:       "Merge Matching Items",
				ActionType: IntegrationActionType_InnerJoin,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Input 1"},
							{Type: domain.NodeHandleTypeDefault, Text: "Input 2"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties:  commonProperties,
				Description: "Merge items that match",
			},
			{
				ID:         string(IntegrationActionType_OuterJoin),
				Name:       "Merge and Keep Unmatched",
				ActionType: IntegrationActionType_OuterJoin,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Input 1"},
							{Type: domain.NodeHandleTypeDefault, Text: "Input 2"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties:  commonProperties,
				Description: "Merge items that match, and keep items that do not match",
			},
			{
				ID:         string(IntegrationActionType_LeftJoin),
				Name:       "Merge and Keep Left",
				ActionType: IntegrationActionType_LeftJoin,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Input 1"},
							{Type: domain.NodeHandleTypeDefault, Text: "Input 2"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties:  commonProperties,
				Description: "Merge items that match, and keep left items that do not match",
			},
			{
				ID:         string(IntegrationActionType_RightJoin),
				Name:       "Merge and Keep Right",
				ActionType: IntegrationActionType_RightJoin,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Input 1"},
							{Type: domain.NodeHandleTypeDefault, Text: "Input 2"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties:  commonProperties,
				Description: "Merge items that match, and keep right items that do not match",
			},
			{
				ID:         string(IntegrationActionType_ExcludeMatching),
				Name:       "Exclude Matching Items",
				ActionType: IntegrationActionType_ExcludeMatching,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Input 1"},
							{Type: domain.NodeHandleTypeDefault, Text: "Input 2"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Properties:  commonProperties,
				Description: "Exclude items that match, keeping only items that do not match",
			},
			{
				ID:         string(IntegrationActionType_Append),
				Name:       "Append All",
				ActionType: IntegrationActionType_Append,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				Properties: []domain.NodeProperty{},
				HandlesByContext: map[domain.ActionUsageContext]domain.ContextHandles{
					domain.UsageContextWorkflow: {
						Input: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Input 1"},
							{Type: domain.NodeHandleTypeDefault, Text: "Input 2"},
						},
						Output: []domain.NodeHandle{
							{Type: domain.NodeHandleTypeDefault, Text: "Output"},
						},
					},
				},
				Description: "Merges two input streams into a single output stream",
			},
		},
	}

	commonProperties = []domain.NodeProperty{
		{
			Key:         "criteria",
			Name:        "Criteria",
			Description: "The fields to match records on",
			Required:    true,
			Type:        domain.NodePropertyType_Array,
			ArrayOpts: &domain.ArrayPropertyOptions{
				ItemType: domain.NodePropertyType_Map,
				ItemProperties: []domain.NodeProperty{
					{
						Key:         "left_field_key",
						Name:        "Left Field Name",
						Description: "The field to match records on from the first input",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "right_field_key",
						Name:        "Right Field Name",
						Description: "The field to match records on from the second input",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
				MinItems: 1,
				MaxItems: 100,
			},
		},
		{
			Key:         "handle_collisions",
			Name:        "Handle Collisions",
			Description: "Whether to handle collisions",
			Type:        domain.NodePropertyType_Boolean,
		},
	}
)
