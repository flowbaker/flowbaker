package youtube

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	IntegrationActionType_UploadVideo    domain.IntegrationActionType = "upload_video"
	IntegrationActionType_GetVideoBrief  domain.IntegrationActionType = "get_video_brief"
	IntegrationActionType_GetVideoBriefs domain.IntegrationActionType = "get_video_briefs"
	IntegrationActionType_DeleteVideo    domain.IntegrationActionType = "delete_video"
)

var (
	Schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_Youtube,
		Name:        "Youtube",
		Description: "Youtube integration for uploading videos, getting video briefs, and deleting videos",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:               "oauth_provider",
				Name:              "OAuth Provider",
				Description:       "The OAuth Provider of Youtube",
				Required:          false,
				Type:              domain.NodePropertyType_OAuth,
				OAuthType:         domain.OAuthTypeGoogle,
				IsCustomOAuthable: true,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "upload_video",
				Name:        "Upload Video",
				Description: "Upload a video to Youtube",
				ActionType:  IntegrationActionType_UploadVideo,
				Properties: []domain.NodeProperty{
					{
						Key:         "title",
						Name:        "Title",
						Description: "The title of the video",
						Type:        domain.NodePropertyType_String,
						Required:    true,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "The description of the video",
						Type:        domain.NodePropertyType_String,
						Required:    true,
					},
					{
						Key:         "video",
						Name:        "Video File",
						Description: "The video file to upload",
						Type:        domain.NodePropertyType_File,
						Required:    true,
					},
					{
						Key:         "tags",
						Name:        "Tags",
						Description: "The tags of the video",
						Type:        domain.NodePropertyType_TagInput,
						Required:    true,
					},
					{
						Key:         "privacy_status",
						Name:        "Privacy Status",
						Description: "The privacy status of the video",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						Options: []domain.NodePropertyOption{
							{
								Label: "Public",
								Value: "public",
							},
							{
								Label: "Unlisted",
								Value: "unlisted",
							},
							{
								Label: "Private",
								Value: "private",
							},
						},
					},
				},
			},
			{
				ID:          "get_video_brief",
				Name:        "Get Video Brief",
				Description: "Get a video brief from Youtube",
				ActionType:  IntegrationActionType_GetVideoBrief,
				Properties: []domain.NodeProperty{
					{
						Key:         "video_id",
						Name:        "Video ID",
						Description: "The ID of the video to get brief",
						Type:        domain.NodePropertyType_String,
						Required:    true,
					},
				},
			},
			{
				ID:          "get_video_briefs",
				Name:        "Get Video Briefs",
				Description: "Get video briefs from Youtube with advanced filtering and sorting",
				ActionType:  IntegrationActionType_GetVideoBriefs,
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The number of videos to get brief (1-50)",
						Type:        domain.NodePropertyType_Integer,
						Required:    true,
					},
					{
						Key:         "channelId",
						Name:        "Channel ID",
						Description: "The ID of a youtube channel, for example: 'UCk8GzjMOrta8yxDcKfylJYA'",
						Type:        domain.NodePropertyType_String,
						Required:    false,
					},
					{
						Key:         "query",
						Name:        "Search Query",
						Description: "The query to get video briefs, for example: 'how to make a video'",
						Type:        domain.NodePropertyType_String,
						Required:    false,
					},
					{
						Key:         "order",
						Name:        "Sort Order",
						Description: "How to sort the search results",
						Type:        domain.NodePropertyType_String,
						Required:    false,
						Options: []domain.NodePropertyOption{
							{
								Label: "Relevance",
								Value: "relevance",
							},
							{
								Label: "Upload Date",
								Value: "date",
							},
							{
								Label: "View Count",
								Value: "viewCount",
							},
							{
								Label: "Rating",
								Value: "rating",
							},
							{
								Label: "Title",
								Value: "title",
							},
						},
					},
					{
						Key:         "type",
						Name:        "Content Type",
						Description: "Types of content to search for",
						Type:        domain.NodePropertyType_String,
						Required:    false,
						Options: []domain.NodePropertyOption{
							{
								Label: "All Types",
								Value: "video,channel,playlist",
							},
							{
								Label: "Videos Only",
								Value: "video",
							},
							{
								Label: "Channels Only",
								Value: "channel",
							},
							{
								Label: "Playlists Only",
								Value: "playlist",
							},
						},
					},
					{
						Key:         "videoDuration",
						Name:        "Video Duration",
						Description: "Filter videos by duration (only applies when type includes video)",
						Type:        domain.NodePropertyType_String,
						Required:    false,
						Options: []domain.NodePropertyOption{
							{
								Label: "Any Duration",
								Value: "any",
							},
							{
								Label: "Short (< 4 minutes)",
								Value: "short",
							},
							{
								Label: "Medium (4-20 minutes)",
								Value: "medium",
							},
							{
								Label: "Long (> 20 minutes)",
								Value: "long",
							},
						},
					},
					{
						Key:         "publishedAfter",
						Name:        "Published After",
						Description: "Only include content published after this date (RFC 3339 format: 2023-01-01T00:00:00Z)",
						Type:        domain.NodePropertyType_String,
						Required:    false,
					},
					{
						Key:         "publishedBefore",
						Name:        "Published Before",
						Description: "Only include content published before this date (RFC 3339 format: 2023-12-31T23:59:59Z)",
						Type:        domain.NodePropertyType_String,
						Required:    false,
					},
					{
						Key:         "regionCode",
						Name:        "Region Code",
						Description: "ISO 3166-1 alpha-2 country code for regional filtering (e.g., US, GB, DE)",
						Type:        domain.NodePropertyType_String,
						Required:    false,
					},
					{
						Key:         "relevanceLanguage",
						Name:        "Language",
						Description: "ISO 639-1 language code for language-relevant results (e.g., en, es, fr)",
						Type:        domain.NodePropertyType_String,
						Required:    false,
					},
				},
			},
			{
				ID:          "delete_video",
				Name:        "Delete Video",
				Description: "Delete a video from Youtube",
				ActionType:  IntegrationActionType_DeleteVideo,
				Properties: []domain.NodeProperty{
					{
						Key:         "video_id",
						Name:        "Video ID",
						Description: "The ID of the video to delete",
						Type:        domain.NodePropertyType_String,
						Required:    true,
					},
				},
			},
		},
	}
)
