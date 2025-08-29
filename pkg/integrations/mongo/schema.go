package mongodb

import (
	"flowbaker/internal/domain"
)

var (
	Schema                    = schema
	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_MongoDB,
		Name:        "MongoDB",
		Description: "Use MongoDB integration to perform database operations like inserting, finding, updating, and deleting documents.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "uri",
				Name:        "MongoDB URI",
				Description: "The connection string URI for your MongoDB deployment",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:         "insert_one",
				Name:       "Insert One",
				ActionType: IntegrationActionType_InsertOne,

				Description: "Insert a single document into a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "document",
						Name:        "Document",
						Description: "The document to insert",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
				},
			},
			{
				ID:         "insert_many",
				Name:       "Insert Many",
				ActionType: IntegrationActionType_InsertMany,

				Description: "Insert multiple documents into a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "documents",
						Name:        "Documents",
						Description: "The documents to insert",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "document",
									Name:        "Document",
									Description: "The document to insert",
									Required:    true,
									Type:        domain.NodePropertyType_Object,
									SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
										Extension: domain.PropertySyntaxExtensionType_JSON,
									},
								},
							},
						},
					},
				},
			},
			{
				ID:          "find_one",
				Name:        "Find One",
				ActionType:  IntegrationActionType_FindOne,
				Description: "Find a single document in a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "filter",
						Name:        "Filter",
						Description: "The filter criteria",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
				},
			},
			{
				ID:          "find_many",
				Name:        "Find Many",
				ActionType:  IntegrationActionType_FindMany,
				Description: "Find multiple documents in a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "filter",
						Name:        "Filter",
						Description: "The filter criteria",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "The maximum number of documents to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "skip",
						Name:        "Skip",
						Description: "The number of documents to skip",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "update_one",
				Name:        "Update One",
				ActionType:  IntegrationActionType_UpdateOne,
				Description: "Update a single document in a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "filter",
						Name:        "Filter",
						Description: "The filter criteria",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
					{
						Key:         "update",
						Name:        "Update",
						Description: "The update operations to apply",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
				},
			},
			{
				ID:          "update_many",
				Name:        "Update Many",
				ActionType:  IntegrationActionType_UpdateMany,
				Description: "Update multiple documents in a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "filter",
						Name:        "Filter",
						Description: "The filter criteria",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
					{
						Key:         "update",
						Name:        "Update",
						Description: "The update operations to apply",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
				},
			},
			{
				ID:          "delete_one",
				Name:        "Delete One",
				ActionType:  IntegrationActionType_DeleteOne,
				Description: "Delete a single document from a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "filter",
						Name:        "Filter",
						Description: "The filter criteria",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
				},
			},
			{
				ID:          "delete_many",
				Name:        "Delete Many",
				ActionType:  IntegrationActionType_DeleteMany,
				Description: "Delete multiple documents from a collection",
				Properties: []domain.NodeProperty{
					{
						Key:              "database",
						Name:             "Database",
						Description:      "The database name",
						Required:         true,
						Type:             domain.NodePropertyType_String,
						Peekable:         true,
						PeekableType:     MongoDBIntegrationPeekable_Databases,
						ExpressionChoice: true,
					},
					{
						Key:          "collection",
						Name:         "Collection",
						Description:  "The collection name",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: MongoDBIntegrationPeekable_Collections,
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
						Key:         "filter",
						Name:        "Filter",
						Description: "The filter criteria",
						Required:    true,
						Type:        domain.NodePropertyType_Object,
						SyntaxHighlightingOpts: domain.SyntaxHighlightingOpts{
							Extension: domain.PropertySyntaxExtensionType_JSON,
						},
					},
				},
			},
			{
				ID:          "store_memory",
				Name:        "Store Memory",
				ActionType:  "store_memory",
				Description: "Store data in MongoDB for AI agent memory",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextMemoryProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "database",
						Name:        "Database",
						Description: "The database name",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "collection",
						Name:        "Collection",
						Description: "The collection name",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "key",
						Name:        "Memory Key",
						Description: "Unique identifier for the memory",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "value",
						Name:        "Memory Value",
						Description: "Data to store in memory",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "retrieve_memory",
				Name:        "Retrieve Memory",
				ActionType:  "retrieve_memory",
				Description: "Retrieve data from MongoDB memory",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextMemoryProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "database",
						Name:        "Database",
						Description: "The database name",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "collection",
						Name:        "Collection",
						Description: "The collection name",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "key",
						Name:        "Memory Key",
						Description: "Unique identifier for the memory to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
	}
)
