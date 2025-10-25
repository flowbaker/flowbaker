package brightdata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
)

const (
	baseURL         = "https://api.brightdata.com/datasets/v3"
	defaultTimeout  = 60 * time.Second
	pollingInterval = 5 * time.Second
)

type BrightDataCredential struct {
	APIToken string `json:"api_token"`
}

type BrightDataIntegrationCreator struct {
	credentialGetter       domain.CredentialGetter[BrightDataCredential]
	binder                 domain.IntegrationParameterBinder
	executorStorageManager domain.ExecutorStorageManager
}

func NewBrightDataIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &BrightDataIntegrationCreator{
		credentialGetter:       managers.NewExecutorCredentialGetter[BrightDataCredential](deps.ExecutorCredentialManager),
		binder:                 deps.ParameterBinder,
		executorStorageManager: deps.ExecutorStorageManager,
	}
}

func (c *BrightDataIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewBrightDataIntegration(BrightDataIntegrationDependencies{
		CredentialGetter:       c.credentialGetter,
		ParameterBinder:        c.binder,
		CredentialID:           p.CredentialID,
		WorkspaceID:            p.WorkspaceID,
		ExecutorStorageManager: c.executorStorageManager,
	})
}

type BrightDataIntegration struct {
	credentialGetter       domain.CredentialGetter[BrightDataCredential]
	binder                 domain.IntegrationParameterBinder
	credentialID           string
	workspaceID            string
	executorStorageManager domain.ExecutorStorageManager
	actionManager          *domain.IntegrationActionManager
}

type BrightDataIntegrationDependencies struct {
	CredentialGetter       domain.CredentialGetter[BrightDataCredential]
	ParameterBinder        domain.IntegrationParameterBinder
	CredentialID           string
	WorkspaceID            string
	ExecutorStorageManager domain.ExecutorStorageManager
}

func NewBrightDataIntegration(deps BrightDataIntegrationDependencies) (*BrightDataIntegration, error) {
	integration := &BrightDataIntegration{
		credentialGetter:       deps.CredentialGetter,
		binder:                 deps.ParameterBinder,
		credentialID:           deps.CredentialID,
		workspaceID:            deps.WorkspaceID,
		executorStorageManager: deps.ExecutorStorageManager,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_Scrape, integration.Scrape)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *BrightDataIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type ScrapeParams struct {
	DatasetID             string `json:"dataset_id"`
	BodyType              string `json:"body_type"`
	JSONBody              string `json:"json_body"`
	TextBody              string `json:"text_body"`
	PollingTimeoutSeconds *int   `json:"polling_timeout_seconds"`
}

type TriggerResponse struct {
	SnapshotID string `json:"snapshot_id"`
}

type ProgressResponse struct {
	Status string `json:"status"`
}

func (i *BrightDataIntegration) Scrape(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ScrapeParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to bind parameters: %w", err)
	}

	if p.DatasetID == "" {
		return nil, fmt.Errorf("dataset_id is required")
	}

	credential, err := i.credentialGetter.GetDecryptedCredential(ctx, i.credentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	snapshotID, err := i.triggerScrapingJob(ctx, credential.APIToken, p)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger scraping job: %w", err)
	}

	timeout := defaultTimeout
	if p.PollingTimeoutSeconds != nil && *p.PollingTimeoutSeconds > 0 {
		timeout = time.Duration(*p.PollingTimeoutSeconds) * time.Second
	}

	ready, err := i.waitForCompletion(ctx, credential.APIToken, snapshotID, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed while waiting for completion: %w", err)
	}

	if !ready {
		return nil, fmt.Errorf("scraping job did not complete within timeout period")
	}

	fileItem, err := i.downloadSnapshot(ctx, credential.APIToken, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}

	response := map[string]interface{}{
		"snapshot_id": snapshotID,
		"file":        fileItem,
	}

	return response, nil
}

func (i *BrightDataIntegration) triggerScrapingJob(ctx context.Context, apiToken string, p ScrapeParams) (string, error) {
	url := fmt.Sprintf("%s/trigger?dataset_id=%s", baseURL, p.DatasetID)

	var bodyReader io.Reader

	if p.BodyType == "json" {
		bodyReader = bytes.NewReader([]byte(p.JSONBody))
	} else if p.BodyType == "text" && p.TextBody != "" {
		bodyReader = strings.NewReader(p.TextBody)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)

	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("trigger request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var triggerResp TriggerResponse

	if err := json.Unmarshal(body, &triggerResp); err != nil {
		return "", fmt.Errorf("failed to parse trigger response: %w", err)
	}

	if triggerResp.SnapshotID == "" {
		return "", fmt.Errorf("snapshot_id not found in response: %s", string(body))
	}

	return triggerResp.SnapshotID, nil
}

func (i *BrightDataIntegration) waitForCompletion(ctx context.Context, apiToken, snapshotID string, timeout time.Duration) (bool, error) {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timeoutTimer.C:
			return false, nil
		case <-ticker.C:
			attempts++

			status, err := i.checkProgress(ctx, apiToken, snapshotID)
			if err != nil {
				return false, fmt.Errorf("failed to check progress (attempt %d): %w", attempts, err)
			}

			if status == "ready" {
				return true, nil
			}
		}
	}
}

func (i *BrightDataIntegration) checkProgress(ctx context.Context, apiToken, snapshotID string) (string, error) {
	url := fmt.Sprintf("%s/progress/%s", baseURL, snapshotID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("progress request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var progressResp ProgressResponse
	if err := json.Unmarshal(body, &progressResp); err != nil {
		return "", fmt.Errorf("failed to parse progress response: %w", err)
	}

	return progressResp.Status, nil
}

func (i *BrightDataIntegration) downloadSnapshot(ctx context.Context, apiToken, snapshotID string) (domain.FileItem, error) {
	url := fmt.Sprintf("%s/snapshot/%s", baseURL, snapshotID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return domain.FileItem{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return domain.FileItem{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return domain.FileItem{}, fmt.Errorf("failed to read response body: %w", err)
		}

		return domain.FileItem{}, fmt.Errorf("snapshot request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.FileItem{}, fmt.Errorf("failed to read response body: %w", err)
	}

	fileName := resp.Header.Get("Content-Disposition")
	if fileName == "" {
		fileName = fmt.Sprintf("brightdata-snapshot-%s.ndjson", snapshotID)
	} else {
		fileName = strings.TrimPrefix(fileName, "attachment; filename=")
		fileName = strings.Trim(fileName, "\"")
	}

	contentType := resp.Header.Get("Content-Type")

	if contentType == "" {
		contentType = "application/x-ndjson"
	}

	fileItem, err := i.executorStorageManager.PutExecutionFile(ctx, domain.PutExecutionFileParams{
		WorkspaceID:  i.workspaceID,
		UploadedBy:   i.workspaceID,
		OriginalName: fileName,
		SizeInBytes:  int64(len(body)),
		ContentType:  contentType,
		Reader:       io.NopCloser(bytes.NewReader(body)),
	})
	if err != nil {
		return domain.FileItem{}, fmt.Errorf("failed to save file to storage: %w", err)
	}

	return fileItem, nil
}
