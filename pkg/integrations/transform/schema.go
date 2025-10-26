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
	IntegrationActionType_MergeByOrder    domain.IntegrationActionType = "merge_by_order"
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
				Properties:  commonProperties[:1],
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
			{
				ID:         string(IntegrationActionType_MergeByOrder),
				Name:       "Merge by Order",
				ActionType: IntegrationActionType_MergeByOrder,
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextWorkflow,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "unmatched_items",
						Name:        "Unmatched Items Behavior",
						Description: "How to handle items when input lengths don't match",
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Truncate", Value: "truncate", Description: "Stop at shorter length, ignore extra items"},
							{Label: "Keep All", Value: "keep_all", Description: "Keep unmatched items from both inputs"},
						},
					},
					{
						Key:         "handle_collisions",
						Name:        "Handle Collisions",
						Description: "Whether to handle collisions",
						Type:        domain.NodePropertyType_Boolean,
					},
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
				Description: "Merges items from two inputs by their order (index position)",
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
						Key:                 "left_field_key",
						Name:                "Left Field Name",
						Description:         "The field to match records on from the first input",
						Required:            true,
						Type:                domain.NodePropertyType_String,
						DragAndDropBehavior: domain.DragAndDropBehavior_BasicPath,
					},
					{
						Key:                 "right_field_key",
						Name:                "Right Field Name",
						Description:         "The field to match records on from the second input",
						Required:            true,
						Type:                domain.NodePropertyType_String,
						DragAndDropBehavior: domain.DragAndDropBehavior_BasicPath,
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
