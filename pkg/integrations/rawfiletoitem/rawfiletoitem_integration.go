package rawfiletoitem

import (
	"context"
	"fmt"
	"io"

	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/flowbaker/flowbaker/pkg/integrations/rawfiletoitem/parsers"
)

type RawFileToItemIntegrationCreator struct {
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewRawFileToItemIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &RawFileToItemIntegrationCreator{
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *RawFileToItemIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewRawFileToItemIntegration(RawFileToItemIntegrationDependencies{
		ParameterBinder:        c.binder,
		ExecutorStorageManager: c.executorStorageManager,
		WorkspaceID:            p.WorkspaceID,
	})
}

type RawFileToItemIntegration struct {
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
	workspaceID            string
	actionManager          *domain.IntegrationActionManager
	parserRegistry         *parsers.ParserRegistry
}

type RawFileToItemIntegrationDependencies struct {
	ParameterBinder        domain.IntegrationParameterBinder
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

func NewRawFileToItemIntegration(deps RawFileToItemIntegrationDependencies) (*RawFileToItemIntegration, error) {
	integration := &RawFileToItemIntegration{
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
		workspaceID:            deps.WorkspaceID,
		parserRegistry:         parsers.NewDefaultRegistry(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemMulti(IntegrationActionType_ConvertRawFileToItem, integration.ConvertRawFileToItem)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *RawFileToItemIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type ConvertRawFileToItemParams struct {
	File   domain.FileItem `json:"file"`
	Format string          `json:"format"`
}

func (i *RawFileToItemIntegration) ConvertRawFileToItem(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := ConvertRawFileToItemParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if p.File.FileID == "" {
		return nil, fmt.Errorf("file is required")
	}

	// Validate format if specified
	format := parsers.FileFormat(p.Format)
	if format == "" {
		format = parsers.FileFormatAuto
	}

	if !parsers.IsValidFormat(format) {
		return nil, fmt.Errorf("unsupported file format: %s. Supported formats: json, ndjson, csv, tsv, xlsx, xml, yaml", p.Format)
	}

	executionFile, err := i.executorStorageManager.GetExecutionFile(ctx, domain.GetExecutionFileParams{
		WorkspaceID: i.workspaceID,
		UploadID:    p.File.FileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer executionFile.Reader.Close()

	if executionFile.SizeInBytes > 100*1024*1024 {
		return nil, fmt.Errorf("file is too large to parse (max 100MB)")
	}

	fileContent, err := io.ReadAll(executionFile.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	items, err := i.parserRegistry.Parse(fileContent, p.File.ContentType, p.File.Name, format)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file (format: %s, content-type: %s, filename: %s): %w",
			format, p.File.ContentType, p.File.Name, err)
	}

	return items, nil
}
