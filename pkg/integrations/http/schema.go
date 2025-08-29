package http

import (
	"flowbaker/internal/domain"
)

const (
	IntegrationActionType_Get    domain.IntegrationActionType = "get"
	IntegrationActionType_Post   domain.IntegrationActionType = "post"
	IntegrationActionType_Put    domain.IntegrationActionType = "put"
	IntegrationActionType_Patch  domain.IntegrationActionType = "patch"
	IntegrationActionType_Delete domain.IntegrationActionType = "delete"
)

type HTTPGenericAuthType string

const (
	HTTPGenericAuthType_Basic  HTTPGenericAuthType = "basic_auth"
	HTTPGenericAuthType_Bearer HTTPGenericAuthType = "bearer_auth"
	HTTPGenericAuthType_Query  HTTPGenericAuthType = "query_auth"
	HTTPGenericAuthType_Header HTTPGenericAuthType = "header_auth"
	HTTPGenericAuthType_Custom HTTPGenericAuthType = "custom_auth"
	HTTPGenericAuthType_Body   HTTPGenericAuthType = "body_auth"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_HTTP,
		Name:        "HTTP",
		Description: "Make HTTP requests to any API endpoint.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "generic_auth_type",
				Name:        "Authentication Method",
				Description: "Choose how to authenticate incoming webhook requests",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				Options: []domain.NodePropertyOption{
					{
						Label: "Basic Authentication",
						Value: HTTPGenericAuthType_Basic,
					},
					{
						Label: "Bearer Authentication",
						Value: HTTPGenericAuthType_Bearer,
					},
					{
						Label: "Query Authentication",
						Value: HTTPGenericAuthType_Query,
					},
					{
						Label: "Header Authentication",
						Value: HTTPGenericAuthType_Header,
					},
					{
						Label: "JSON Body Authentication",
						Value: HTTPGenericAuthType_Body,
					},
				},
			},
			{
				Key:         "username",
				Name:        "Username",
				Description: "The username used to authenticate with the API",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Basic,
				},
			},
			{
				Key:         "password",
				Name:        "Password",
				Description: "The password used to authenticate with the API",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Basic,
				},
			},
			{
				Key:         "bearer_token",
				Name:        "Bearer Token",
				Description: "The bearer token used to authenticate with the API",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Bearer,
				},
			},
			{
				Key:         "query_auth_key",
				Name:        "Query Parameter Key",
				Description: "The key for the authentication query parameter",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Query,
				},
			},
			{
				Key:         "query_auth_value",
				Name:        "Query Parameter Value",
				Description: "The value for the authentication query parameter",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Query,
				},
			},
			{
				Key:         "header_auth_key",
				Name:        "Header Key",
				Description: "The key for the authentication header",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Header,
				},
			},
			{
				Key:         "header_auth_value",
				Name:        "Header Value",
				Description: "The value for the authentication header",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Header,
				},
			},
			{
				Key:         "body_auth",
				Name:        "Body Authentication",
				Description: "Authentication in request body",
				Type:        domain.NodePropertyType_Object,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_Body,
				},
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "http_get",
				Name:        "GET Request",
				ActionType:  IntegrationActionType_Get,
				Description: "Make an HTTP GET request",
				Properties: []domain.NodeProperty{
					{
						Key:         "url",
						Name:        "URL",
						Description: "The URL to send the request to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "query_params",
						Name:        "Query Params",
						Description: "The query params to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "headers",
						Name:        "Headers",
						Description: "The headers to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
				},
			},
			{
				ID:          "http_post",
				Name:        "POST Request",
				ActionType:  IntegrationActionType_Post,
				Description: "Make an HTTP POST request",
				Properties: []domain.NodeProperty{
					{
						Key:         "url",
						Name:        "URL",
						Description: "The URL to send the request to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "query_params",
						Name:        "Query Params",
						Description: "The query params to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "headers",
						Name:        "Headers",
						Description: "The headers to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "body_type",
						Name:        "Body Type",
						Description: "The type of body to send with the request",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						Options: []domain.NodePropertyOption{
							{
								Label: "JSON",
								Value: "json",
							},
							{
								Label: "Text",
								Value: "text",
							},
							{
								Label: "URL Encoded Form Data",
								Value: "urlencoded_form_data",
							},
							{
								Label: "Multi-Part Form Data",
								Value: "multipart_form_data",
							},
							{
								Label: "File (Octet Stream)",
								Value: "file",
							},
						},
					},
					{
						Key:         "json_body",
						Name:        "JSON Body",
						Description: "The JSON body to send with the request",
						Type:        domain.NodePropertyType_Object,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "json",
						},
						Required: true,
					},
					{
						Key:         "text_body",
						Name:        "Text Body",
						Description: "The text body to send with the request",
						Type:        domain.NodePropertyType_Text,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "text",
						},
						Required: true,
					},
					{
						Key:         "urlencoded_form_data_body",
						Name:        "URL Encoded Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "urlencoded_form_data",
						},
						Required: true,
					},
					{
						Key:         "multipart_form_data_body",
						Name:        "Multi-Part Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_File,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "multipart_form_data",
						},
						Required: true,
					},
					{
						Key:         "file_body",
						Name:        "File Body",
						Description: "The file body to send with the request",
						Type:        domain.NodePropertyType_File,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "file",
						},
						Required: true,
					},
				},
			},
			{
				ID:          "http_put",
				Name:        "PUT Request",
				ActionType:  IntegrationActionType_Put,
				Description: "Make an HTTP PUT request",
				Properties: []domain.NodeProperty{
					{
						Key:         "url",
						Name:        "URL",
						Description: "The URL to send the request to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "query_params",
						Name:        "Query Params",
						Description: "The query params to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "headers",
						Name:        "Headers",
						Description: "The headers to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "body_type",
						Name:        "Body Type",
						Description: "The type of body to send with the request",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						Options: []domain.NodePropertyOption{
							{
								Label: "JSON",
								Value: "json",
							},
							{
								Label: "Text",
								Value: "text",
							},
							{
								Label: "URL Encoded Form Data",
								Value: "urlencoded_form_data",
							},
							{
								Label: "Multi-Part Form Data",
								Value: "multipart_form_data",
							},
							{
								Label: "File (Octet Stream)",
								Value: "file",
							},
						},
					},
					{
						Key:         "json_body",
						Name:        "JSON Body",
						Description: "The JSON body to send with the request",
						Type:        domain.NodePropertyType_Object,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "json",
						},
						Required: true,
					},
					{
						Key:         "text_body",
						Name:        "Text Body",
						Description: "The text body to send with the request",
						Type:        domain.NodePropertyType_Text,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "text",
						},
						Required: true,
					},
					{
						Key:         "urlencoded_form_data_body",
						Name:        "URL Encoded Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "urlencoded_form_data",
						},
						Required: true,
					},
					{
						Key:         "multipart_form_data_body",
						Name:        "Multi-Part Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_File,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "multipart_form_data",
						},
						Required: true,
					},
					{
						Key:         "file_body",
						Name:        "File Body",
						Description: "The file body to send with the request",
						Type:        domain.NodePropertyType_File,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "file",
						},
						Required: true,
					},
				},
			},
			{
				ID:          "http_patch",
				Name:        "PATCH Request",
				ActionType:  IntegrationActionType_Patch,
				Description: "Make an HTTP PATCH request",
				Properties: []domain.NodeProperty{
					{
						Key:         "url",
						Name:        "URL",
						Description: "The URL to send the request to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "query_params",
						Name:        "Query Params",
						Description: "The query params to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "headers",
						Name:        "Headers",
						Description: "The headers to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "body_type",
						Name:        "Body Type",
						Description: "The type of body to send with the request",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						Options: []domain.NodePropertyOption{
							{
								Label: "JSON",
								Value: "json",
							},
							{
								Label: "Text",
								Value: "text",
							},
							{
								Label: "URL Encoded Form Data",
								Value: "urlencoded_form_data",
							},
							{
								Label: "Multi-Part Form Data",
								Value: "multipart_form_data",
							},
							{
								Label: "File (Octet Stream)",
								Value: "file",
							},
						},
					},
					{
						Key:         "json_body",
						Name:        "JSON Body",
						Description: "The JSON body to send with the request",
						Type:        domain.NodePropertyType_Object,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "json",
						},
						Required: true,
					},
					{
						Key:         "text_body",
						Name:        "Text Body",
						Description: "The text body to send with the request",
						Type:        domain.NodePropertyType_Text,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "text",
						},
						Required: true,
					},
					{
						Key:         "urlencoded_form_data_body",
						Name:        "URL Encoded Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "urlencoded_form_data",
						},
						Required: true,
					},
					{
						Key:         "multipart_form_data_body",
						Name:        "Multi-Part Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_File,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "multipart_form_data",
						},
						Required: true,
					},
					{
						Key:         "file_body",
						Name:        "File Body",
						Description: "The file body to send with the request",
						Type:        domain.NodePropertyType_File,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "file",
						},
						Required: true,
					},
				},
			},
			{
				ID:          "http_delete",
				Name:        "DELETE Request",
				ActionType:  IntegrationActionType_Delete,
				Description: "Make an HTTP DELETE request",
				Properties: []domain.NodeProperty{
					{
						Key:         "url",
						Name:        "URL",
						Description: "The URL to send the request to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "query_params",
						Name:        "Query Params",
						Description: "The query params to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "headers",
						Name:        "Headers",
						Description: "The headers to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						Required: true,
					},
					{
						Key:         "body_type",
						Name:        "Body Type",
						Description: "The type of body to send with the request",
						Type:        domain.NodePropertyType_String,
						Required:    true,
						Options: []domain.NodePropertyOption{
							{
								Label: "JSON",
								Value: "json",
							},
							{
								Label: "Text",
								Value: "text",
							},
							{
								Label: "URL Encoded Form Data",
								Value: "urlencoded_form_data",
							},
							{
								Label: "Multi-Part Form Data",
								Value: "multipart_form_data",
							},
							{
								Label: "File (Octet Stream)",
								Value: "file",
							},
						},
					},
					{
						Key:         "json_body",
						Name:        "JSON Body",
						Description: "The JSON body to send with the request",
						Type:        domain.NodePropertyType_Object,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "json",
						},
						Required: true,
					},
					{
						Key:         "text_body",
						Name:        "Text Body",
						Description: "The text body to send with the request",
						Type:        domain.NodePropertyType_Text,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "text",
						},
						Required: true,
					},
					{
						Key:         "urlencoded_form_data_body",
						Name:        "URL Encoded Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "urlencoded_form_data",
						},
						Required: true,
					},
					{
						Key:         "multipart_form_data_body",
						Name:        "Multi-Part Form Data Body",
						Description: "The form data body to send with the request",
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key for the query parameter",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value for the header",
									Type:        domain.NodePropertyType_File,
									Required:    true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "multipart_form_data",
						},
						Required: true,
					},
					{
						Key:         "file_body",
						Name:        "File Body",
						Description: "The file body to send with the request",
						Type:        domain.NodePropertyType_File,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       "file",
						},
						Required: true,
					},
				},
			},
		},
	}
)
