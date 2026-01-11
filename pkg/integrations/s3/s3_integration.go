package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/rs/zerolog/log"
)

type S3IntegrationCreator struct {
	credentialGetter       domain.CredentialGetter[S3Credential]
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewS3IntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &S3IntegrationCreator{
		credentialGetter:       managers.NewExecutorCredentialGetter[S3Credential](deps.ExecutorCredentialManager),
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *S3IntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewS3Integration(ctx, S3IntegrationDependencies{
		WorkspaceID:            p.WorkspaceID,
		CredentialGetter:       c.credentialGetter,
		ParameterBinder:        c.binder,
		CredentialID:           p.CredentialID,
		ExecutorStorageManager: c.executorStorageManager,
	})
}

type S3Integration struct {
	workspaceID  string
	credentialID string

	s3Client               *s3.S3
	credentialGetter       domain.CredentialGetter[S3Credential]
	binder                 domain.IntegrationParameterBinder
	actionManager          *domain.IntegrationActionManager
	peekFuncs              map[domain.IntegrationPeekableType]domain.PeekFunc
	executorStorageManager domain.ExecutorStorageManager
}

type S3Credential struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Region          string `json:"region"`
}

type S3IntegrationDependencies struct {
	WorkspaceID  string
	CredentialID string

	CredentialGetter       domain.CredentialGetter[S3Credential]
	ParameterBinder        domain.IntegrationParameterBinder
	ExecutorStorageManager domain.ExecutorStorageManager
}

func NewS3Integration(ctx context.Context, deps S3IntegrationDependencies) (*S3Integration, error) {
	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(credential.Region),
		Credentials: credentials.NewStaticCredentials(credential.AccessKeyID, credential.SecretAccessKey, ""),
	})
	if err != nil {
		return nil, err
	}

	s3Client := s3.New(sess)

	integration := &S3Integration{
		workspaceID:            deps.WorkspaceID,
		credentialID:           deps.CredentialID,
		s3Client:               s3Client,
		credentialGetter:       deps.CredentialGetter,
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_UploadObject, integration.UploadObject).
		AddPerItemWithFile(IntegrationActionType_DownloadObject, integration.DownloadObject).
		AddPerItem(IntegrationActionType_DeleteObject, integration.DeleteObject).
		AddPerItem(IntegrationActionType_ListObjects, integration.ListObjects).
		AddPerItem(IntegrationActionType_CopyObject, integration.CopyObject).
		AddPerItem(IntegrationActionType_GetObjectInfo, integration.GetObjectInfo).
		AddPerItem(IntegrationActionType_CreateBucket, integration.CreateBucket).
		AddPerItem(IntegrationActionType_DeleteBucket, integration.DeleteBucket)

	peekFuncs := map[domain.IntegrationPeekableType]domain.PeekFunc{
		S3IntegrationPeekable_Buckets: integration.PeekBuckets,
		S3IntegrationPeekable_Objects: integration.PeekObjects,
		S3IntegrationPeekable_Regions: integration.PeekRegions,
	}

	integration.peekFuncs = peekFuncs
	integration.actionManager = actionManager

	return integration, nil
}

func (i *S3Integration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *S3Integration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found: %s", params.PeekableType)
	}

	return peekFunc(ctx, params)
}

type UploadObjectParams struct {
	Bucket       string          `json:"bucket"`
	Key          string          `json:"key"`
	Content      domain.FileItem `json:"content"`
	ContentType  string          `json:"content_type"`
	MetadataJSON string          `json:"metadata"`
}

func (i *S3Integration) UploadObject(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UploadObjectParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	uploader := s3manager.NewUploaderWithClient(i.s3Client)

	metadata := map[string]any{}

	if p.MetadataJSON != "" {
		err = json.Unmarshal([]byte(p.MetadataJSON), &metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	contentType := p.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	executionFile, err := i.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: i.workspaceID,
		UploadID:    p.Content.FileID,
	})
	if err != nil {
		return nil, err
	}

	if p.Key == "" {
		p.Key = p.Content.Name
	}

	awsMetadata := map[string]*string{}

	for k, v := range metadata {
		strVal := fmt.Sprintf("%v", v)
		awsMetadata[k] = aws.String(strVal)
	}

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(p.Bucket),
		Key:         aws.String(p.Key),
		Body:        executionFile.Reader,
		ContentType: aws.String(contentType),
		Metadata:    awsMetadata,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to upload object to S3")
		return nil, err
	}

	return result, nil
}

type DownloadObjectParams struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func (i *S3Integration) DownloadObject(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.ItemWithFile, error) {
	p := DownloadObjectParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.ItemWithFile{}, err
	}

	result, err := i.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(p.Key),
	})
	if err != nil {
		return domain.ItemWithFile{}, err
	}
	defer result.Body.Close()

	fileItem, err := i.executorStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  i.workspaceID,
		OriginalName: filepath.Base(p.Key),
		SizeInBytes:  *result.ContentLength,
		ContentType:  *result.ContentType,
		Reader:       result.Body,
		UploadedBy:   i.workspaceID,
	})
	if err != nil {
		return domain.ItemWithFile{}, err
	}

	return domain.ItemWithFile{
		Item:            item,
		UseFileFieldKey: "file_content",
		File:            fileItem,
	}, nil
}

// Delete Object
type DeleteObjectParams struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func (i *S3Integration) DeleteObject(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteObjectParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	result, err := i.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(p.Key),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// List Objects
type ListObjectsParams struct {
	Bucket                   string   `json:"bucket"`
	Prefix                   string   `json:"prefix"`
	MaxKeys                  int64    `json:"max_keys"`
	Delimiter                string   `json:"delimiter"`
	FetchOwner               bool     `json:"fetch_owner"`
	ContinuationToken        string   `json:"continuation_token"`
	RequestPayer             string   `json:"request_payer"`
	EncodingType             string   `json:"encoding_type"`
	ExpectedBucketOwner      string   `json:"expected_bucket_owner"`
	OptionalObjectAttributes []string `json:"optional_object_attributes"`
	StartAfter               string   `json:"start_after"`
}

func (i *S3Integration) ListObjects(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ListObjectsParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	input := &s3.ListObjectsV2Input{}

	if p.Bucket != "" {
		input.Bucket = aws.String(p.Bucket)
	}

	if p.Prefix != "" {
		input.Prefix = aws.String(p.Prefix)
	}

	if p.MaxKeys > 0 {
		input.MaxKeys = aws.Int64(p.MaxKeys)
	}

	if p.Delimiter != "" {
		input.Delimiter = aws.String(p.Delimiter)
	}

	if p.FetchOwner {
		input.FetchOwner = aws.Bool(p.FetchOwner)
	}

	if p.ContinuationToken != "" {
		input.ContinuationToken = aws.String(p.ContinuationToken)
	}

	if p.StartAfter != "" {
		input.StartAfter = aws.String(p.StartAfter)
	}

	if p.EncodingType != "" {
		input.EncodingType = aws.String(p.EncodingType)
	}

	if p.ExpectedBucketOwner != "" {
		input.ExpectedBucketOwner = aws.String(p.ExpectedBucketOwner)
	}

	if p.RequestPayer != "" {
		input.RequestPayer = aws.String(p.RequestPayer)
	}

	listResult, err := i.s3Client.ListObjectsV2WithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	return listResult, nil
}

// Copy Object
type CopyObjectParams struct {
	SourceBucket      string `json:"source_bucket"`
	SourceKey         string `json:"source_key"`
	DestinationBucket string `json:"destination_bucket"`
	DestinationKey    string `json:"destination_key"`
}

func (i *S3Integration) CopyObject(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CopyObjectParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	sourcePath := fmt.Sprintf("%s/%s", p.SourceBucket, p.SourceKey)

	result, err := i.s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(p.DestinationBucket),
		Key:        aws.String(p.DestinationKey),
		CopySource: aws.String(sourcePath),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Get Object Info
type GetObjectInfoParams struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func (i *S3Integration) GetObjectInfo(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetObjectInfoParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	headResult, err := i.s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(p.Bucket),
		Key:    aws.String(p.Key),
	})
	if err != nil {
		return nil, err
	}

	return headResult, nil
}

// Create Bucket
type CreateBucketParams struct {
	Bucket string `json:"bucket"`
	ACL    string `json:"acl"`
}

func (i *S3Integration) CreateBucket(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateBucketParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	createBucketInput := &s3.CreateBucketInput{
		Bucket: aws.String(p.Bucket),
	}

	if p.ACL != "" {
		createBucketInput.ACL = aws.String(p.ACL)
	}

	result, err := i.s3Client.CreateBucket(createBucketInput)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Delete Bucket
type DeleteBucketParams struct {
	Bucket string `json:"bucket"`
}

func (i *S3Integration) DeleteBucket(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteBucketParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	result, err := i.s3Client.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(p.Bucket),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Peek functions
func (i *S3Integration) PeekBuckets(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	result, err := i.s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return domain.PeekResult{}, err
	}

	var items []domain.PeekResultItem
	for _, bucket := range result.Buckets {
		items = append(items, domain.PeekResultItem{
			Key:     *bucket.Name,
			Value:   *bucket.Name,
			Content: *bucket.Name,
		})
	}

	return domain.PeekResult{
		Result: items,
	}, nil
}

type PeekObjectsParams struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
}

func (i *S3Integration) PeekObjects(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	var p PeekObjectsParams
	if err := json.Unmarshal(params.PayloadJSON, &p); err != nil {
		return domain.PeekResult{}, err
	}

	limit := params.GetLimitWithMax(20, 1000)
	pageToken := params.Pagination.Cursor

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(p.Bucket),
		Prefix:    aws.String(p.Prefix),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int64(int64(limit)),
	}

	if pageToken != "" {
		input.ContinuationToken = aws.String(pageToken)
	}

	result, err := i.s3Client.ListObjectsV2(input)
	if err != nil {
		return domain.PeekResult{}, err
	}

	var items []domain.PeekResultItem
	for _, obj := range result.Contents {
		items = append(items, domain.PeekResultItem{
			Key:     *obj.Key,
			Value:   *obj.Key,
			Content: *obj.Key,
		})
	}

	peekResult := domain.PeekResult{
		Result: items,
	}

	if result.NextContinuationToken != nil {
		peekResult.Pagination.NextCursor = *result.NextContinuationToken
	}

	if result.IsTruncated != nil {
		peekResult.Pagination.HasMore = *result.IsTruncated
	}

	return peekResult, nil
}

func (i *S3Integration) PeekRegions(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	// List of AWS regions
	regions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"af-south-1", "ap-east-1", "ap-south-1", "ap-northeast-1",
		"ap-northeast-2", "ap-northeast-3", "ap-southeast-1", "ap-southeast-2",
		"ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2",
		"eu-west-3", "eu-south-1", "eu-north-1", "me-south-1",
		"sa-east-1",
	}

	var items []domain.PeekResultItem
	for _, region := range regions {
		items = append(items, domain.PeekResultItem{
			Key:     region,
			Value:   region,
			Content: region,
		})
	}

	return domain.PeekResult{
		Result: items,
	}, nil
}
