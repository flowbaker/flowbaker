package jwtintegration

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_JWT,
		Name:        "JSON Web Token",
		Description: "Use JWT integration to create and decode JWT tokens",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "jwt_secret",
				Name:        "JWT Secret Key",
				Description: "The secret key used to generate JWT signatures",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				Hidden:      true,
			},
			{
				Key:         "jwt_algorithm",
				Name:        "JWT Signature Algorithm",
				Description: "The algorithm used to sign and verify JWTs",
				Type:        domain.NodePropertyType_String,
				Required:    true,
				Hidden:      true,
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
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "create_token",
				ActionType:  ActionCreateToken,
				Name:        "Create Token",
				Description: "Create a JWT token",
				Properties: []domain.NodeProperty{
					{
						Key:         "claims",
						Name:        "Claims",
						Description: "The claims to include in the JWT token",
						Type:        domain.NodePropertyType_CodeEditor,
						Required:    true,
					},
					{
						Key:         "expire_seconds",
						Name:        "Expire Seconds",
						Description: "The number of seconds until the JWT token expires",
						Type:        domain.NodePropertyType_Integer,
						Required:    true,
					},
				},
			},
			{
				ID:          "decode_token",
				ActionType:  ActionDecodeToken,
				Name:        "Decode Token",
				Description: "Decode a JWT token",
				Properties: []domain.NodeProperty{
					{
						Key:         "token",
						Name:        "Token",
						Description: "The JWT token to decode",
						Type:        domain.NodePropertyType_String,
						Required:    true,
					},
				},
			},
		},
	}
)
