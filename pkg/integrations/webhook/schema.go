package webhook

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationTriggerType_HttpRequestReceived domain.IntegrationTriggerEventType = "http_request_received"

	RespondType_Instant      string = "instant"
	RespondType_SendResponse string = "send_response"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Webhook,
		Name:                 "Webhook",
		Description:          "Receive webhook requests from any API endpoint.",
		IsCredentialOptional: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "webhook_auth_type",
				Name:        "Authentication Method",
				Description: "Choose how to authenticate incoming webhook requests",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				Options: []domain.NodePropertyOption{
					{
						Label: "JWT (JSON Web Token)",
						Value: "jwt",
					},
					{
						Label: "Basic Authentication",
						Value: "basic_auth",
					},
				},
			},
			{
				Key:         "jwt_secret",
				Name:        "JWT Secret Key",
				Description: "The secret key used to verify JWT signatures",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				Hidden:      true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "webhook_auth_type",
					Value:       "jwt",
				},
			},
			{
				Key:         "jwt_algorithm",
				Name:        "JWT Signature Algorithm",
				Description: "The algorithm used to sign and verify JWTs",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				Hidden:      true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "webhook_auth_type",
					Value:       "jwt",
				},
				Options: []domain.NodePropertyOption{
					{
						Label: "HS256 (HMAC with SHA-256)",
						Value: "HS256",
					},
					{
						Label: "HS384 (HMAC with SHA-384)",
						Value: "HS384",
					},
					{
						Label: "HS512 (HMAC with SHA-512)",
						Value: "HS512",
					},
					{
						Label: "RS256 (RSA with SHA-256)",
						Value: "RS256",
					},
					{
						Label: "RS384 (RSA with SHA-384)",
						Value: "RS384",
					},
					{
						Label: "RS512 (RSA with SHA-512)",
						Value: "RS512",
					},
					{
						Label: "ES256 (ECDSA with SHA-256)",
						Value: "ES256",
					},
					{
						Label: "ES384 (ECDSA with SHA-384)",
						Value: "ES384",
					},
					{
						Label: "ES512 (ECDSA with SHA-512)",
						Value: "ES512",
					},
				},
			},
			{
				Key:         "basic_auth_username",
				Name:        "Basic Authentication Username",
				Description: "The username used to authenticate with the API",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "webhook_auth_type",
					Value:       "basic_auth",
				},
			},
			{
				Key:         "basic_auth_password",
				Name:        "Basic Authentication Password",
				Description: "The password used to authenticate with the API",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "webhook_auth_type",
					Value:       "basic_auth",
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "http_request_received",
				Name:        "HTTP Request Received",
				EventType:   IntegrationTriggerType_HttpRequestReceived,
				Description: "Triggered when an HTTP request is received",
				OutputHandles: []domain.NodeHandle{
					{
						Type: domain.NodeHandleTypeDefault,
					},
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "path",
						Name:        "Path",
						Description: "The path of the request",
						Required:    true,
						Type:        domain.NodePropertyType_Endpoint,
						EndpointPropertyOpts: &domain.EndpointPropertyOptions{
							AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
						},
					},
					{
						Key:         "respond_type",
						Name:        "Respond Options",
						Description: "Choose how to respond to the request",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{
								Label: "Return Response Immediately",
								Value: RespondType_Instant,
							},
							{
								Label: "Use Send Response Node",
								Value: RespondType_SendResponse,
							},
						},
					},
				},
			},
		},
	}
)
