package snowflake

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema                    = schema
	schema domain.Integration = domain.Integration{
		ID:                domain.IntegrationType_Snowflake,
		Name:              "Snowflake",
		Description:       "Use Snowflake integration to perform data warehouse operations like executing queries, inserting data, and updating records.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "account",
				Name:        "Account",
				Description: "Snowflake account identifier",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "username",
				Name:        "Username",
				Description: "Snowflake username",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "auth_type",
				Name:        "Authentication Type",
				Description: "Choose authentication method",
				Required:    true,
				Type:        domain.NodePropertyType_String,
				Options: []domain.NodePropertyOption{
					{
						Label: "Password",
						Value: "password",
					},
					{
						Label: "Key Pair",
						Value: "key_pair",
					},
				},
			},
			{
				Key:         "password",
				Name:        "Password",
				Description: "Snowflake password",
				Required:    true,
				Type:        domain.NodePropertyType_String,
				IsSecret:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "auth_type",
					Value:       "password",
				},
			},
			{
				Key:         "private_key",
				Name:        "Private Key",
				Description: "RSA private key in PEM format",
				Required:    true,
				Type:        domain.NodePropertyType_Text,
				IsSecret:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "auth_type",
					Value:       "key_pair",
				},
			},
			{
				Key:         "private_key_passphrase",
				Name:        "Private Key Passphrase",
				Description: "Passphrase for encrypted private key (optional)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
				IsSecret:    true,
				DependsOn: &domain.DependsOn{
					PropertyKey: "auth_type",
					Value:       "key_pair",
				},
			},
			{
				Key:         "warehouse",
				Name:        "Warehouse",
				Description: "Snowflake warehouse name",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "database",
				Name:        "Database",
				Description: "Default database name (optional)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "schema",
				Name:        "Schema",
				Description: "Default schema name (optional)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "role",
				Name:        "Role",
				Description: "Snowflake role (optional)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "execute_query",
				Name:        "Execute Query",
				ActionType:  IntegrationActionType_ExecuteQuery,
				Description: "Execute a SQL query and return results",
				Properties: []domain.NodeProperty{
					{
						Key:              "query",
						Name:             "Query",
						Description:      "The SQL query to execute",
						Required:         true,
						Type:             domain.NodePropertyType_CodeEditor,
						ExpressionChoice: true,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_SQL,
							Dialect:   domain.PropertySyntaxDialectType_Snowflake,
						},
					},
				},
			},
			{
				ID:          "insert",
				Name:        "Insert",
				ActionType:  IntegrationActionType_Insert,
				Description: "Insert data into a Snowflake table",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     SnowflakePeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "schema",
						Name:         "Schema",
						Description:  "The schema name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SnowflakePeekable_Schemas,
						Dependent:    []string{"database"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "database",
								ValueKey:    "database",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:          "table",
						Name:         "Table",
						Description:  "The table name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SnowflakePeekable_Tables,
						Dependent:    []string{"database", "schema"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "database",
								ValueKey:    "database",
							},
							{
								PropertyKey: "schema",
								ValueKey:    "schema",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:         "data",
						Name:        "Data",
						Description: "The data to insert as JSON object",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
				},
			},
			{
				ID:          "update",
				Name:        "Update",
				ActionType:  IntegrationActionType_Update,
				Description: "Update data in a Snowflake table",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     SnowflakePeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "schema",
						Name:         "Schema",
						Description:  "The schema name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SnowflakePeekable_Schemas,
						Dependent:    []string{"database"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "database",
								ValueKey:    "database",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:          "table",
						Name:         "Table",
						Description:  "The table name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: SnowflakePeekable_Tables,
						Dependent:    []string{"database", "schema"},
						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "database",
								ValueKey:    "database",
							},
							{
								PropertyKey: "schema",
								ValueKey:    "schema",
							},
						},
						ExpressionChoice: true,
					},
					{
						Key:         "data",
						Name:        "Data",
						Description: "The data to update as JSON object",
						Required:    true,
						Type:        domain.NodePropertyType_CodeEditor,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
					{
						Key:         "conditions",
						Name:        "Conditions",
						Description: "WHERE clause conditions (combined with AND)",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 20,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:              "column",
									Name:             "Column",
									Description:      "Column name",
									Type:             domain.NodePropertyType_String,
									Required:         true,
									ExpressionChoice: true,
								},
								{
									Key:         "operator",
									Name:        "Operator",
									Description: "Comparison operator",
									Type:        domain.NodePropertyType_String,
									Required:    true,
									Options: []domain.NodePropertyOption{
										{Label: "Equals (=)", Value: "="},
										{Label: "Not Equals (!=)", Value: "!="},
										{Label: "Greater Than (>)", Value: ">"},
										{Label: "Less Than (<)", Value: "<"},
										{Label: "Greater or Equal (>=)", Value: ">="},
										{Label: "Less or Equal (<=)", Value: "<="},
										{Label: "LIKE", Value: "LIKE"},
										{Label: "NOT LIKE", Value: "NOT LIKE"},
										{Label: "IS NULL", Value: "IS NULL"},
										{Label: "IS NOT NULL", Value: "IS NOT NULL"},
									},
								},
								{
									Key:              "value",
									Name:             "Value",
									Description:      "Value to compare (ignored for IS NULL/IS NOT NULL)",
									Type:             domain.NodePropertyType_String,
									Required:         false,
									ExpressionChoice: true,
								},
							},
						},
					},
				},
			},
		},
	}
)
