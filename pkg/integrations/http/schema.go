package http

import "github.com/flowbaker/flowbaker/pkg/domain"

const (
	IntegrationType_HTTP         domain.IntegrationType       = "http"
	IntegrationActionType_Post   domain.IntegrationActionType = domain.IntegrationActionType(HTTPMethod_Post)
	IntegrationActionType_Get    domain.IntegrationActionType = domain.IntegrationActionType(HTTPMethod_Get)
	IntegrationActionType_Put    domain.IntegrationActionType = domain.IntegrationActionType(HTTPMethod_Put)
	IntegrationActionType_Delete domain.IntegrationActionType = domain.IntegrationActionType(HTTPMethod_Delete)
	IntegrationActionType_Patch  domain.IntegrationActionType = domain.IntegrationActionType(HTTPMethod_Patch)
)

var (
	Schema = schema

	schema = domain.Integration{
		ID:                   IntegrationType_HTTP,
		Name:                 "HTTP",
		Description:          "Simple HTTP POST request.",
		IsCredentialOptional: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "auth_type",
				Name:        "Auth Type",
				Description: "Whether to send no credential, use the vault entry as generic auth (basic/bearer/etc.), or as a pre-defined OAuth/API credential",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				Options: []domain.NodePropertyOption{
					{
						Label: "No credential",
						Value: string(HTTPAuthType_NoCredential),
					},
					{
						Label: "Generic (basic, bearer, header, query)",
						Value: string(HTTPAuthType_Generic),
					},
					{
						Label: "Pre-defined (OAuth or default vault payload)",
						Value: string(HTTPAuthType_PreDefined),
					},
				},
			},
			{
				Key:         "generic_auth_type",
				Name:        "Generic authentication method",
				Description: "Used when credential usage is generic",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "auth_type",
					Value:       string(HTTPAuthType_Generic),
				},
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
						Label: "JSON (Custom JSON Parameters)",
						Value: HTTPGenericAuthType_JSON,
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
				Key:         "custom_json_payload",
				Name:        "Custom JSON Payload",
				Description: "JSON payload that will be merged into request body without removing existing keys",
				Type:        domain.NodePropertyType_CodeEditor,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "generic_auth_type",
					Value:       HTTPGenericAuthType_JSON,
				},
			},
		},
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
							{
								Label: "Multi-Part Form Data",
								Value: string(HTTPBodyType_MultiPartFormData),
							},
							{
								Label: "File (Application/Octet-Stream)",
								Value: string(HTTPBodyType_Application_OctetStream),
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
					// not implemented yet currently. but should be implemented in the future.
					// {
					// 	Key:         "body",
					// 	Name:        "Body",
					// 	Description: "Request body (JSON)",
					// 	Type:        domain.NodePropertyType_CodeEditor,
					// 	DependsOn: &domain.DependsOn{
					// 		PropertyKey: "body_type",
					// 		Value:       string(HTTPBodyType_JSON),
					// 	},
					// },
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
									Key:         "values",
									Name:        "Values",
									Description: "Query parameter values",
									Type:        domain.NodePropertyType_Array,
									ArrayOpts: &domain.ArrayPropertyOptions{
										MinItems: 0,
										MaxItems: 100,
										ItemType: domain.NodePropertyType_Map,
										ItemProperties: []domain.NodeProperty{
											{
												Key:         "value",
												Name:        "Value",
												Description: "Query parameter value",
												Type:        domain.NodePropertyType_String,
											},
										},
									},
									Required: true,
								},
							},
						},
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       string(HTTPBodyType_URLEncodedFormData),
						},
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "Request body (Multi-Part Form Data)",
						Type:        domain.NodePropertyType_Array,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       string(HTTPBodyType_MultiPartFormData),
						},
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 0,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{

								{
									Key:         "name",
									Name:        "Name",
									Description: "Form data file name (without extension)",
									Type:        domain.NodePropertyType_String,
									Required:    true,
								},
								{
									Key:         "file",
									Name:        "File",
									Description: "Multi-Part form data file",
									Type:        domain.NodePropertyType_File,
									Required:    true,
								},
							},
						},
					},
					{
						Key:         "body",
						Name:        "Body",
						Description: "Request body (File (Application/Octet-Stream))",
						Type:        domain.NodePropertyType_File,
						DependsOn: &domain.DependsOn{
							PropertyKey: "body_type",
							Value:       string(HTTPBodyType_Application_OctetStream),
						},
					},
				},
			},
		},
	}
)
