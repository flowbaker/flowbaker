package base64

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_EncodeToBase64   domain.IntegrationActionType = "encode_to_base64"
	IntegrationActionType_DecodeFromBase64 domain.IntegrationActionType = "decode_from_base64"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                   domain.IntegrationType_Base64,
		Name:                 "Base64",
		Description:          "Encode text to Base64 or decode Base64 to text",
		IsCredentialOptional: true,
		Actions: []domain.IntegrationAction{
			{
				ID:         string(IntegrationActionType_EncodeToBase64),
				Name:       "Encode to Base64",
				ActionType: IntegrationActionType_EncodeToBase64,
				Properties: []domain.NodeProperty{
					{
						Key:         "text",
						Name:        "Text",
						Description: "The text to encode to Base64",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
				Description: "Encodes the provided text to Base64 format",
			},
			{
				ID:         string(IntegrationActionType_DecodeFromBase64),
				Name:       "Decode from Base64",
				ActionType: IntegrationActionType_DecodeFromBase64,
				Properties: []domain.NodeProperty{
					{
						Key:         "encoded_text",
						Name:        "Base64 Text",
						Description: "The Base64 encoded text to decode",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
				Description: "Decodes the provided Base64 text to plain text",
			},
		},
	}
)
