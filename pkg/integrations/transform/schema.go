package transform

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_MergeStreams     domain.IntegrationActionType = "merge_streams"
	IntegrationActionType_InnerJoin        domain.IntegrationActionType = "inner_join"
	IntegrationActionType_OuterJoin        domain.IntegrationActionType = "outer_join"
	IntegrationActionType_LeftJoin         domain.IntegrationActionType = "left_join"
	IntegrationActionType_RightJoin        domain.IntegrationActionType = "right_join"
	IntegrationActionType_ReverseInnerJoin domain.IntegrationActionType = "reverse_inner_join"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Transform,
		Name:        "Transform",
		Description: "Transform node to merge multiple input streams into a single output stream",
		Actions: []domain.IntegrationAction{
			{
				ID:         "merge_streams",
				Name:       "Append All",
				ActionType: IntegrationActionType_MergeStreams,
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
				Description: "Merges two input streams into a single output stream",
			},
			{
				ID:         "inner_join",
				Name:       "Only Matching Items",
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
				Properties: []domain.NodeProperty{
					{
						Key:         "join_criteria",
						Name:        "Join Criteria",
						Description: "The fields to match records on",
						Required:    true,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
				Description: "Items that match, merged into a single item",
			},
			{
				ID:         "outer_join",
				Name:       "All Items (Keep Unmatched Too)",
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
				Properties: []domain.NodeProperty{
					{
						Key:         "join_criteria",
						Name:        "Join Criteria",
						Description: "The fields to match records on",
						Required:    true,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
				Description: "Items that match, merged into a single item. Plus items that do not match, added as separate items",
			},
			{
				ID:         "left_join",
				Name:       "Keep All from Input 1",
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
				Properties: []domain.NodeProperty{
					{
						Key:         "join_criteria",
						Name:        "Join Criteria",
						Description: "The fields to match records on",
						Required:    true,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
				Description: "All items from input 1, with matching items from input 2 merged in",
			},
			{
				ID:         "right_join",
				Name:       "Keep All from Input 2",
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
				Properties: []domain.NodeProperty{
					{
						Key:         "join_criteria",
						Name:        "Join Criteria",
						Description: "The fields to match records on",
						Required:    true,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
				Description: "All items from input 2, with matching items from input 1 merged in",
			},
			{
				ID:         "reverse_inner_join",
				Name:       "Only Non-Matching Items",
				ActionType: IntegrationActionType_ReverseInnerJoin,
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
				Properties: []domain.NodeProperty{
					{
						Key:         "join_criteria",
						Name:        "Join Criteria",
						Description: "The fields to match records on",
						Required:    true,
						Type:        domain.NodePropertyType_TagInput,
					},
				},
				Description: "Items that don't match, kept as separate items",
			},
		},
	}
)
