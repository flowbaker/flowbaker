package s3

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	IntegrationActionType_UploadObject   = "upload_object"
	IntegrationActionType_DownloadObject = "download_object"
	IntegrationActionType_DeleteObject   = "delete_object"
	IntegrationActionType_ListObjects    = "list_objects"
	IntegrationActionType_CopyObject     = "copy_object"
	IntegrationActionType_GetObjectInfo  = "get_object_info"
	IntegrationActionType_CreateBucket   = "create_bucket"
	IntegrationActionType_DeleteBucket   = "delete_bucket"
)

const (
	S3IntegrationPeekable_Buckets  = "buckets"
	S3IntegrationPeekable_Objects  = "objects"
	S3IntegrationPeekable_Prefixes = "prefixes"
	S3IntegrationPeekable_Regions  = "regions"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:          domain.IntegrationType_AwsS3,
		Name:        "Amazon S3",
		Description: "Use Amazon S3 integration to manage objects in buckets, including upload, download, and deletion operations.",
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "access_key_id",
				Name:        "AWS Access Key ID",
				Description: "Your AWS access key ID",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "secret_access_key",
				Name:        "AWS Secret Access Key",
				Description: "Your AWS secret access key",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:          "region",
				Name:         "AWS Region",
				Description:  "The AWS region where your S3 buckets are located",
				Required:     true,
				Type:         domain.NodePropertyType_String,
				Peekable:     true,
				PeekableType: S3IntegrationPeekable_Regions,
			},
		},
		Actions: []domain.IntegrationAction{
			{
				ID:          "upload_object",
				Name:        "Upload Object",
				ActionType:  IntegrationActionType_UploadObject,
				Description: "Upload a file or data to an S3 bucket",
				Properties: []domain.NodeProperty{
					{
						Key:          "bucket",
						Name:         "Bucket",
						Description:  "The name of the S3 bucket",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:         "key",
						Name:        "Object Key",
						Description: "The key (path) where the object will be stored",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "content",
						Name:        "Content",
						Description: "The content to upload (can be file data or text)",
						Required:    true,
						Type:        domain.NodePropertyType_File,
						ValidDraggableTypes: []string{
							"file",
						},
					},
					{
						Key:         "content_type",
						Name:        "Content Type",
						Description: "The MIME type of the content",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "metadata",
						Name:        "Metadata",
						Description: "Custom metadata to attach to the object",
						Required:    false,
						Type:        domain.NodePropertyType_CodeEditor,
					},
				},
			},
			{
				ID:          "download_object",
				Name:        "Download Object",
				ActionType:  IntegrationActionType_DownloadObject,
				Description: "Download an object from an S3 bucket",
				Properties: []domain.NodeProperty{
					{
						Key:          "bucket",
						Name:         "Bucket",
						Description:  "The name of the S3 bucket",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:                      "key",
						Name:                     "Object Key",
						Description:              "The key (path) of the object to download",
						Required:                 true,
						Type:                     domain.NodePropertyType_String,
						Peekable:                 true,
						PeekableType:             S3IntegrationPeekable_Objects,
						Dependent:                []string{"bucket"},
						PeekablePaginationType:   domain.PeekablePaginationType_PageToken,

						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "bucket",
								ValueKey:    "bucket",
							},
						},
					},
				},
			},
			{
				ID:          "delete_object",
				Name:        "Delete Object",
				ActionType:  IntegrationActionType_DeleteObject,
				Description: "Delete an object from an S3 bucket",
				Properties: []domain.NodeProperty{
					{
						Key:          "bucket",
						Name:         "Bucket",
						Description:  "The name of the S3 bucket",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:                      "key",
						Name:                     "Object Key",
						Description:              "The key (path) of the object to delete",
						Required:                 true,
						Type:                     domain.NodePropertyType_String,
						Peekable:                 true,
						PeekableType:             S3IntegrationPeekable_Objects,
						Dependent:                []string{"bucket"},
						PeekablePaginationType:   domain.PeekablePaginationType_PageToken,

						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "bucket",
								ValueKey:    "bucket",
							},
						},
					},
				},
			},
			{
				ID:          "list_objects",
				Name:        "List Objects",
				ActionType:  IntegrationActionType_ListObjects,
				Description: "List objects in an S3 bucket with optional prefix filtering",
				Properties: []domain.NodeProperty{
					{
						Key:          "bucket",
						Name:         "Bucket",
						Description:  "The name of the S3 bucket",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:          "prefix",
						Name:         "Prefix",
						Description:              "Filter objects by prefix (folder path)",
						Required:                 false,
						Type:                     domain.NodePropertyType_String,
						Peekable:                 true,
						PeekableType:             S3IntegrationPeekable_Prefixes,
						Dependent:                []string{"bucket"},
						PeekablePaginationType:   domain.PeekablePaginationType_PageToken,

						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "bucket",
								ValueKey:    "bucket",
							},
						},
					},
					{
						Key:         "max_keys",
						Name:        "Maximum Keys",
						Description: "Maximum number of objects to return",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "copy_object",
				Name:        "Copy Object",
				ActionType:  IntegrationActionType_CopyObject,
				Description: "Copy an object within or between S3 buckets",
				Properties: []domain.NodeProperty{
					{
						Key:          "source_bucket",
						Name:         "Source Bucket",
						Description:  "The name of the source S3 bucket",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:          "source_key",
						Name:         "Source Object Key",
						Description:              "The key (path) of the source object",
						Required:                 true,
						Type:                     domain.NodePropertyType_String,
						Peekable:                 true,
						PeekableType:             S3IntegrationPeekable_Objects,
						Dependent:                []string{"source_bucket"},
						PeekablePaginationType:   domain.PeekablePaginationType_PageToken,

						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "source_bucket",
								ValueKey:    "bucket",
							},
						},
					},
					{
						Key:          "destination_bucket",
						Name:         "Destination Bucket",
						Description:  "The name of the destination S3 bucket",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:         "destination_key",
						Name:        "Destination Object Key",
						Description: "The key (path) for the copied object",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_object_info",
				Name:        "Get Object Info",
				ActionType:  IntegrationActionType_GetObjectInfo,
				Description: "Get metadata and information about an S3 object",
				Properties: []domain.NodeProperty{
					{
						Key:          "bucket",
						Name:         "Bucket",
						Description:  "The name of the S3 bucket",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:          "key",
						Name:         "Object Key",
						Description:              "The key (path) of the object",
						Required:                 true,
						Type:                     domain.NodePropertyType_String,
						Peekable:                 true,
						PeekableType:             S3IntegrationPeekable_Objects,
						Dependent:                []string{"bucket"},
						PeekablePaginationType:   domain.PeekablePaginationType_PageToken,

						PeekableDependentProperties: []domain.PeekableDependentProperty{
							{
								PropertyKey: "bucket",
								ValueKey:    "bucket",
							},
						},
					},
				},
			},
			{
				ID:          "create_bucket",
				Name:        "Create Bucket",
				ActionType:  IntegrationActionType_CreateBucket,
				Description: "Create a new S3 bucket",
				Properties: []domain.NodeProperty{
					{
						Key:         "bucket",
						Name:        "Bucket Name",
						Description: "The name for the new bucket",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "acl",
						Name:        "Bucket ACL",
						Description: "The canned ACL to apply to the bucket",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{
								Label: "Private",
								Value: "private",
							},
							{
								Label: "Public Read",
								Value: "public-read",
							},
							{
								Label: "Public Read Write",
								Value: "public-read-write",
							},
							{
								Label: "Authenticated Read",
								Value: "authenticated-read",
							},
						},
					},
				},
			},
			{
				ID:          "delete_bucket",
				Name:        "Delete Bucket",
				ActionType:  IntegrationActionType_DeleteBucket,
				Description: "Delete an empty S3 bucket",
				Properties: []domain.NodeProperty{
					{
						Key:          "bucket",
						Name:         "Bucket",
						Description:  "The name of the bucket to delete",
						Required:     true,
						Type:         domain.NodePropertyType_String,
						Peekable:     true,
						PeekableType: S3IntegrationPeekable_Buckets,
					},
					{
						Key:         "force",
						Name:        "Force Delete",
						Description: "If true, delete all objects in the bucket before deleting the bucket",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
				},
			},
		},
	}
)
