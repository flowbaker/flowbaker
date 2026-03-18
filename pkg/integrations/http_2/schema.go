package http_2

import "github.com/flowbaker/flowbaker/pkg/domain"

const (
	IntegrationType_HTTP2      domain.IntegrationType       = "http_2"
	IntegrationActionType_Post domain.IntegrationActionType = "post"
)

var (
	Schema = schema

	schema = domain.Integration{
		ID:                   IntegrationType_HTTP2,
		Name:                 "HTTP 2",
		Description:          "Simple HTTP POST request.",
		IsCredentialOptional: true,
		Actions: []domain.IntegrationAction{
			{
				ID:          string(IntegrationActionType_Post),
				ActionType:  IntegrationActionType_Post,
				Name:        "POST Request",
				Description: "Make an HTTP POST request",
				Properties: []domain.NodeProperty{
					{
						Key:         "url",
						Name:        "URL",
						Description: "Request URL",
						Type:        domain.NodePropertyType_String,
						Required:    true,
					},
					{
						Key:         "headers",
						Name:        "Headers",
						Description: "Optional request headers",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 0,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "Header key",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "Header value",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
					},
					{
						Key:         "query_params",
						Name:        "Query Parameters",
						Description: "Optional request query parameters",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 0,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "Query parameter key",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "Query parameter value",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
					},
					{
						Key:         "body_type",
						Name:        "Body Type ",
						Description: "Request body type",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						Options: []domain.NodePropertyOption{
							{
								Label: "Text",
								Value: string(HTTPBodyType_Text),
							},
							{
								Label: "JSON",
								Value: string(HTTPBodyType_JSON),
							},
							{
								Label: "URL Encoded Form Data",
								Value: string(HTTPBodyType_URLEncodedFormData),
							},
						},
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "Request body (Text)",
						Type:        domain.NodePropertyType_Text,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       string(HTTPBodyType_Text),
						},
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "Request body (JSON)",
						Type:        domain.NodePropertyType_CodeEditor,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       string(HTTPBodyType_JSON),
						},
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "Request body (URL Encoded Form Data)",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 0,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{

								{
									Key:         "key",
									Name:        "Key",
									Description: "Query parameter key",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "Query parameter value",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       string(HTTPBodyType_URLEncodedFormData),
						},
					},
				},
			},
		},
	}
)
