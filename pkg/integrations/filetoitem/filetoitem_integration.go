package filetoitem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/flowbaker/flowbaker/pkg/domain"
)

type FileToItemIntegrationCreator struct {
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewFileToItemIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &FileToItemIntegrationCreator{
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *FileToItemIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewFileToItemIntegration(FileToItemIntegrationDependencies{
		ParameterBinder:        c.binder,
		ExecutorStorageManager: c.executorStorageManager,
		WorkspaceID:            p.WorkspaceID,
	})
}

type FileToItemIntegration struct {
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
	workspaceID            string
	actionManager          *domain.IntegrationActionManager
	fileParser             *FileParser
}

type FileToItemIntegrationDependencies struct {
	ParameterBinder        domain.IntegrationParameterBinder
	ExecutorStorageManager domain.ExecutorStorageManager
	WorkspaceID            string
}

func NewFileToItemIntegration(deps FileToItemIntegrationDependencies) (*FileToItemIntegration, error) {
	integration := &FileToItemIntegration{
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
		workspaceID:            deps.WorkspaceID,
		fileParser:             NewFileParser(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItemMulti(IntegrationActionType_ConvertFileToItem, integration.ConvertFileToItem)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *FileToItemIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type ConvertFileToItemParams struct {
	File domain.FileItem `json:"file"`
}

func (i *FileToItemIntegration) ConvertFileToItem(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := ConvertFileToItemParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if p.File.FileID == "" {
		return nil, fmt.Errorf("file is required")
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

	if i.fileParser.IsNDJSON(fileContent, p.File.ContentType, p.File.Name) {
		return i.fileParser.ParseNDJSON(fileContent)
	}

	items, err := i.fileParser.ParseJSON(fileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file as JSON (content-type: %s, filename: %s): %w",
			p.File.ContentType, p.File.Name, err)
	}

	return items, nil
}

type FileParser struct {
	ndjsonContentTypes []string
	ndjsonExtensions   []string
	ndjsonContentHints []string
}

func NewFileParser() *FileParser {
	return &FileParser{
		ndjsonContentTypes: []string{"ndjson", "x-ndjson"},
		ndjsonExtensions:   []string{".ndjson", ".jsonl"},
		ndjsonContentHints: []string{"\n{"},
	}
}

func (fp *FileParser) IsNDJSON(content []byte, contentType, fileName string) bool {
	contentTypeLower := strings.ToLower(contentType)
	fileNameLower := strings.ToLower(fileName)

	for _, ct := range fp.ndjsonContentTypes {
		if strings.Contains(contentTypeLower, ct) {
			return true
		}
	}

	for _, ext := range fp.ndjsonExtensions {
		if strings.HasSuffix(fileNameLower, ext) {
			return true
		}
	}

	contentStr := string(content)
	for _, hint := range fp.ndjsonContentHints {
		if strings.Contains(contentStr, hint) {
			return true
		}
	}

	return false
}

func (fp *FileParser) ParseNDJSON(content []byte) ([]domain.Item, error) {
	var items []domain.Item
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var item interface{}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("failed to parse NDJSON line %d: %w", lineNum+1, err)
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("NDJSON file is empty")
	}

	return items, nil
}

func (fp *FileParser) ParseJSON(content []byte) ([]domain.Item, error) {
	var result interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, err
	}

	if arr, ok := result.([]interface{}); ok {
		items := make([]domain.Item, len(arr))
		for i, item := range arr {
			items[i] = item
		}
		return items, nil
	}

	return []domain.Item{result}, nil
}
