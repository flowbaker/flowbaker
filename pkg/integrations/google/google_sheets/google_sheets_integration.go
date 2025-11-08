package googlesheets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	IntegrationActionType_CreateSheet     domain.IntegrationActionType = "create_sheet"
	IntegrationActionType_CreateWorkSheet domain.IntegrationActionType = "create_work_sheet"
	IntegrationActionType_DeleteSheet     domain.IntegrationActionType = "delete_sheet"
	IntegrationActionType_AddColumn       domain.IntegrationActionType = "add_column"
	IntegrationActionType_AddData         domain.IntegrationActionType = "add_data"
	IntegrationActionType_CopySpreadSheet domain.IntegrationActionType = "copy_spread_sheet"
	IntegrationActionType_CopyWorkSheet   domain.IntegrationActionType = "copy_work_sheet"
	IntegrationActionType_DeleteWorkSheet domain.IntegrationActionType = "delete_work_sheet"
	IntegrationActionType_FindRows        domain.IntegrationActionType = "find_rows"
)

const (
	GoogleSheetsPeekable_Files   domain.IntegrationPeekableType = "files"
	GoogleSheetsPeekable_Sheets  domain.IntegrationPeekableType = "sheets"
	GoogleSheetsPeekable_Columns domain.IntegrationPeekableType = "columns"
)

type GoogleSheetsIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder           domain.IntegrationParameterBinder
}

func NewGoogleSheetsIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &GoogleSheetsIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

type GoogleSheetsIntegrationDependencies struct {
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialID     string
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
}

type GoogleSheetsIntegration struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder           domain.IntegrationParameterBinder
	sheetsService    *sheets.Service
	driveService     *drive.Service

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

func (c *GoogleSheetsIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewGoogleSheetsIntegration(ctx, GoogleSheetsIntegrationDependencies{
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
		CredentialGetter: c.credentialGetter,
	})
}

func NewGoogleSheetsIntegration(ctx context.Context, deps GoogleSheetsIntegrationDependencies) (*GoogleSheetsIntegration, error) {
	integration := &GoogleSheetsIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
	}

	tokens, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	token := &oauth2.Token{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Expiry:       tokens.Expiry,
	}

	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))

	if integration.sheetsService == nil {
		integration.sheetsService, err = sheets.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return nil, fmt.Errorf("failed to create sheets service: %w", err)
		}
	}

	if integration.driveService == nil {
		integration.driveService, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return nil, fmt.Errorf("failed to create drive service: %w", err)
		}
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(IntegrationActionType_CreateSheet, integration.CreateSheet).
		AddPerItem(IntegrationActionType_DeleteSheet, integration.DeleteSheet).
		AddPerItem(IntegrationActionType_AddColumn, integration.AddColumn).
		AddPerItem(IntegrationActionType_AddData, integration.AppendJSONDataToWorksheet).
		AddPerItem(IntegrationActionType_CreateWorkSheet, integration.CreateWorksheet).
		AddPerItem(IntegrationActionType_CopySpreadSheet, integration.CopySpreadsheet).
		AddPerItem(IntegrationActionType_CopyWorkSheet, integration.CopyWorksheet).
		AddPerItem(IntegrationActionType_DeleteWorkSheet, integration.DeleteWorksheet).
		AddPerItemMulti(IntegrationActionType_FindRows, integration.FindRows)

	integration.actionManager = actionManager

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		GoogleSheetsPeekable_Files:   integration.PeekFiles,
		GoogleSheetsPeekable_Sheets:  integration.PeekSheets,
		GoogleSheetsPeekable_Columns: integration.PeekColumns,
	}

	integration.peekFuncs = peekFuncs

	return integration, nil
}

func (g *GoogleSheetsIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	log.Info().Msg("Executing Google Sheets integration")

	return g.actionManager.Run(ctx, params.ActionType, params)
}

type CreateSheetParams struct {
	Title string `json:"title"`
}

type DeleteSheetParams struct {
	SpreadsheetID string `json:"spreadsheet_id"`
}

type AddColumnParams struct {
	SpreadsheetID string `json:"spreadsheet_id"`
	WorksheetID   string `json:"worksheet_id"`
	Content       string `json:"content"`
	Index         int    `json:"index"`
}

func (g *GoogleSheetsIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := g.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

func (g *GoogleSheetsIntegration) PeekFiles(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	limit := p.GetLimitWithMax(20, 100)
	pageToken := p.Pagination.Cursor

	query := "mimeType='application/vnd.google-apps.spreadsheet'"
	listCall := g.driveService.Files.List().
		Q(query).
		Fields("nextPageToken, files(id, name)").
		PageSize(int64(limit)).
		Context(ctx)

	if pageToken != "" {
		listCall = listCall.PageToken(pageToken)
	}

	filesList, err := listCall.Do()
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list spreadsheets: %w", err)
	}

	var results []domain.PeekResultItem
	for _, file := range filesList.Files {
		results = append(results, domain.PeekResultItem{
			Key:     file.Name, // Use file name for display and dependencies
			Value:   file.Id,   // Use file ID for backend processing
			Content: file.Name, // Use file name for display
		})
	}

	hasMore := filesList.NextPageToken != ""

	result := domain.PeekResult{
		Result: results,
		Pagination: domain.PaginationMetadata{
			NextCursor: filesList.NextPageToken,
			HasMore:    hasMore,
		},
	}

	return result, nil
}

func (g *GoogleSheetsIntegration) PeekSheets(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	// Parse PayloadJSON to get spreadsheet_id
	var payload map[string]interface{}
	if len(p.PayloadJSON) > 0 {
		if err := json.Unmarshal(p.PayloadJSON, &payload); err != nil {
			return domain.PeekResult{}, fmt.Errorf("failed to parse payload JSON: %w", err)
		}
	}

	// Try to get spreadsheet_id from different sources
	var spreadsheetID string

	// First try from payload
	if id, ok := payload["spreadsheet_id"]; ok {
		if idStr, ok := id.(string); ok && idStr != "" {
			spreadsheetID = idStr
		}
	}

	// If not found in payload, try params.Path
	if spreadsheetID == "" && p.Path != "" {
		spreadsheetID = p.Path
	}

	// If still not found, return empty result (no sheets available)
	if spreadsheetID == "" {
		return domain.PeekResult{
			Result: []domain.PeekResultItem{},
		}, nil
	}

	spreadsheet, err := g.sheetsService.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	var sheets []domain.PeekResultItem
	for _, sheet := range spreadsheet.Sheets {
		sheetID := strconv.Itoa(int(sheet.Properties.SheetId))
		sheets = append(sheets, domain.PeekResultItem{
			Key:     sheet.Properties.Title, // Use sheet title for display and dependencies
			Value:   sheetID,                // Use sheet ID for backend processing
			Content: sheet.Properties.Title, // Use sheet title for display
		})
	}

	return domain.PeekResult{
		Result: sheets,
	}, nil
}

func (g *GoogleSheetsIntegration) PeekColumns(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	// Parse PayloadJSON to get spreadsheet_id and worksheet_id
	var payload map[string]interface{}
	if len(p.PayloadJSON) > 0 {
		if err := json.Unmarshal(p.PayloadJSON, &payload); err != nil {
			return domain.PeekResult{}, fmt.Errorf("failed to parse payload JSON: %w", err)
		}
	}

	// Try to get spreadsheet_id and worksheet_id from payload
	var spreadsheetID, worksheetID string

	if id, ok := payload["spreadsheet_id"]; ok {
		if idStr, ok := id.(string); ok && idStr != "" {
			spreadsheetID = idStr
		}
	}

	if id, ok := payload["worksheet_id"]; ok {
		if idStr, ok := id.(string); ok && idStr != "" {
			worksheetID = idStr
		}
	}

	// If either required field is missing, return empty result
	if spreadsheetID == "" || worksheetID == "" {
		return domain.PeekResult{
			Result: []domain.PeekResultItem{},
		}, nil
	}

	spreadsheet, err := g.sheetsService.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	var sheetTitle string
	// Check if worksheetID is a title (string) or ID (numeric)
	if targetSheetID, err := strconv.Atoi(worksheetID); err == nil {
		// It's a numeric ID, find by sheet ID
		for _, sheet := range spreadsheet.Sheets {
			if int(sheet.Properties.SheetId) == targetSheetID {
				sheetTitle = sheet.Properties.Title
				break
			}
		}
	} else {
		// It's a title, find by sheet title
		for _, sheet := range spreadsheet.Sheets {
			if sheet.Properties.Title == worksheetID {
				sheetTitle = sheet.Properties.Title
				break
			}
		}
	}

	if sheetTitle == "" {
		return domain.PeekResult{}, fmt.Errorf("worksheet with id/title %s not found", worksheetID)
	}

	headerRange := fmt.Sprintf("%s!1:1", sheetTitle)
	headerResp, err := g.sheetsService.Spreadsheets.Values.Get(spreadsheetID, headerRange).Context(ctx).Do()
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get header row: %w", err)
	}
	if len(headerResp.Values) == 0 || len(headerResp.Values[0]) == 0 {
		return domain.PeekResult{}, fmt.Errorf("no header row found in worksheet %s", sheetTitle)
	}

	header := headerResp.Values[0]
	var columns []domain.PeekResultItem
	for _, cell := range header {
		cellStr := fmt.Sprintf("%v", cell)
		columns = append(columns, domain.PeekResultItem{
			Key:     cellStr,
			Value:   cellStr,
			Content: cellStr,
		})
	}

	return domain.PeekResult{
		Result: columns,
	}, nil
}

func (g *GoogleSheetsIntegration) getClient(ctx context.Context, oauthAccount domain.OAuthAccountWithSensitiveData) (*http.Client, error) {
	token := &oauth2.Token{
		AccessToken:  oauthAccount.SensitiveData.AccessToken,
		RefreshToken: oauthAccount.SensitiveData.RefreshToken,
	}
	ts := oauth2.StaticTokenSource(token)
	client := oauth2.NewClient(ctx, ts)
	return client, nil
}

func (g *GoogleSheetsIntegration) CreateSheet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	log.Info().Msg("Creating Google Sheet")

	var p CreateSheetParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: p.Title,
		},
	}

	createdSpreadsheet, err := g.sheetsService.Spreadsheets.Create(spreadsheet).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create spreadsheet: %w", err)
	}

	log.Info().Msgf("Created sheet with id %s and title %s", createdSpreadsheet.SpreadsheetId, p.Title)

	result := map[string]string{
		"spreadsheet_id": createdSpreadsheet.SpreadsheetId,
		"title":          p.Title,
	}

	return domain.Item(result), nil
}

func (g *GoogleSheetsIntegration) DeleteSheet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	log.Info().Msg("Deleting Google Sheet")

	var p DeleteSheetParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	if err := g.driveService.Files.Delete(p.SpreadsheetID).Context(ctx).Do(); err != nil {
		return nil, fmt.Errorf("failed to delete spreadsheet: %w. If you get permission errors, please re-authorize your Google account with the required Drive permissions", err)
	}

	log.Info().Msgf("Deleted sheet with id %s", p.SpreadsheetID)

	result := map[string]string{
		"spreadsheet_id": p.SpreadsheetID,
		"status":         "deleted",
	}

	return domain.Item(result), nil
}

func columnNumberToLetter(n int) string {
	if n < 1 {
		n = 1
	}
	var result string
	for n > 0 {
		n--
		remainder := n % 26
		result = string(rune('A'+remainder)) + result
		n = n / 26
	}
	return result
}

func (g *GoogleSheetsIntegration) AddColumn(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p AddColumnParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	worksheetId, err := strconv.Atoi(p.WorksheetID)
	if err != nil {
		return nil, fmt.Errorf("invalid worksheet id: %w", err)
	}

	if p.Index < 1 {
		p.Index = 1
	}

	startIndex := p.Index
	if startIndex < 1 {
		startIndex = 1
	}

	req := &sheets.Request{
		InsertDimension: &sheets.InsertDimensionRequest{
			Range: &sheets.DimensionRange{
				SheetId:    int64(worksheetId),
				Dimension:  "COLUMNS",
				StartIndex: int64(startIndex),
				EndIndex:   int64(startIndex) + 1,
			},
			InheritFromBefore: true,
		},
	}

	batchReq := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}

	_, batchErr := g.sheetsService.Spreadsheets.BatchUpdate(p.SpreadsheetID, batchReq).Context(ctx).Do()
	if batchErr != nil {
		return nil, fmt.Errorf("failed to add column: %w", batchErr)
	}

	ss, err := g.sheetsService.Spreadsheets.Get(p.SpreadsheetID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	var sheetTitle string
	for _, sheet := range ss.Sheets {
		if sheet.Properties.SheetId == int64(worksheetId) {
			sheetTitle = sheet.Properties.Title
			break
		}
	}
	if sheetTitle == "" {
		return nil, fmt.Errorf("sheet with id %d not found", worksheetId)
	}

	colLetter := columnNumberToLetter(p.Index)
	rangeStr := fmt.Sprintf("%s!%s1:%s1", sheetTitle, colLetter, colLetter)

	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{{p.Content}},
	}
	_, updateErr := g.sheetsService.Spreadsheets.Values.Update(p.SpreadsheetID, rangeStr, valueRange).
		ValueInputOption("RAW").Context(ctx).Do()
	if updateErr != nil {
		return nil, fmt.Errorf("failed to update column text: %w", updateErr)
	}

	result := map[string]string{
		"spreadsheet_id": p.SpreadsheetID,
		"status":         "column_added_and_text_updated",
		"index":          strconv.Itoa(p.Index),
		"worksheet_id":   p.WorksheetID,
		"content":        p.Content,
	}

	return domain.Item(result), nil
}

func flattenMap(d map[string]interface{}, prefix string, out map[string]string) {
	for key, value := range d {
		var fullKey string
		if prefix == "" {
			fullKey = key
		} else {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			flattenMap(v, fullKey, out)
		case []interface{}:
			var parts []string
			for _, item := range v {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
			out[fullKey] = strings.Join(parts, ", ")
		default:
			out[fullKey] = fmt.Sprintf("%v", v)
		}
	}
}

type AppendJSONDataParams struct {
	SpreadsheetID string `json:"spreadsheet_id"`
	WorksheetID   string `json:"worksheet_id"`
	Data          string `json:"data"`
}

func (g *GoogleSheetsIntegration) AppendJSONDataToWorksheet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p AppendJSONDataParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, fmt.Errorf("failed to bind params: %w", err)
	}

	// Get the worksheet name from the ID
	worksheetId, err := strconv.ParseInt(p.WorksheetID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid worksheet id: %w", err)
	}

	ss, err := g.sheetsService.Spreadsheets.Get(p.SpreadsheetID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	var sheetName string
	for _, sheet := range ss.Sheets {
		if sheet.Properties.SheetId == worksheetId {
			sheetName = sheet.Properties.Title
			break
		}
	}
	if sheetName == "" {
		return nil, fmt.Errorf("sheet with id %s not found", p.WorksheetID)
	}

	// Parse the JSON data
	var jsonData interface{}
	if err := json.Unmarshal([]byte(p.Data), &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Convert to array of maps
	var allItems []map[string]interface{}
	switch data := jsonData.(type) {
	case []interface{}:
		for _, item := range data {
			if m, ok := item.(map[string]interface{}); ok {
				allItems = append(allItems, m)
			}
		}
	case map[string]interface{}:
		allItems = append(allItems, data)
	default:
		return nil, fmt.Errorf("data must be a JSON object or array of objects")
	}

	if len(allItems) == 0 {
		return nil, fmt.Errorf("no valid data items found")
	}

	// Flatten the data
	allKeysMap := make(map[string]bool)
	flattenedItems := make([]map[string]string, len(allItems))
	for i, item := range allItems {
		flat := make(map[string]string)
		flattenMap(item, "", flat)
		flattenedItems[i] = flat
		for key := range flat {
			allKeysMap[key] = true
		}
	}

	var keys []string
	for k := range allKeysMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	headerRow := make([]interface{}, len(keys))
	for i, k := range keys {
		headerRow[i] = k
	}

	dataRows := make([][]interface{}, len(flattenedItems))
	for i, flat := range flattenedItems {
		row := make([]interface{}, len(keys))
		for j, key := range keys {
			if val, ok := flat[key]; ok {
				row[j] = val
			} else {
				row[j] = ""
			}
		}
		dataRows[i] = row
	}

	// Check if header exists
	readRange := fmt.Sprintf("%s!A1:Z1", sheetName)
	readResp, err := g.sheetsService.Spreadsheets.Values.Get(p.SpreadsheetID, readRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet header: %w", err)
	}

	appendRows := [][]interface{}{}
	if len(readResp.Values) == 0 || len(readResp.Values[0]) == 0 {
		appendRows = append(appendRows, headerRow)
	}
	appendRows = append(appendRows, dataRows...)

	appendRange := fmt.Sprintf("%s!A1", sheetName)
	valueRange := &sheets.ValueRange{
		Values: appendRows,
	}

	resp, err := g.sheetsService.Spreadsheets.Values.Append(p.SpreadsheetID, appendRange, valueRange).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to append data to sheet: %w", err)
	}

	result := map[string]interface{}{
		"spreadsheet_id": p.SpreadsheetID,
		"worksheet_id":   p.WorksheetID,
		"sheet_name":     sheetName,
		"rows_added":     len(dataRows),
		"response":       resp,
	}

	return domain.Item(result), nil
}

type CreateWorksheetParams struct {
	SpreadsheetID string `json:"spreadsheet_id"`
	Title         string `json:"title"`
}

func (g *GoogleSheetsIntegration) CreateWorksheet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	log.Info().Msg("Creating worksheet")

	var p CreateWorksheetParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	req := &sheets.Request{
		AddSheet: &sheets.AddSheetRequest{
			Properties: &sheets.SheetProperties{
				Title: p.Title,
			},
		},
	}

	batchReq := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}
	resp, err := g.sheetsService.Spreadsheets.BatchUpdate(p.SpreadsheetID, batchReq).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create worksheet: %w", err)
	}

	result := map[string]interface{}{
		"spreadsheet_id": p.SpreadsheetID,
		"new_sheet_id":   resp.Replies[0].AddSheet.Properties.SheetId,
		"title":          p.Title,
		"item":           item,
	}

	return domain.Item(result), nil
}

type CopySpreadsheetParams struct {
	SpreadsheetID string `json:"spreadsheet_id"`
	Title         string `json:"title"`
}

func (g *GoogleSheetsIntegration) CopySpreadsheet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	log.Info().Msg("Creating worksheet")

	var p CopySpreadsheetParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	file := &drive.File{
		Name: p.Title,
	}
	copiedFile, err := g.driveService.Files.Copy(p.SpreadsheetID, file).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to copy spreadsheet: %w", err)
	}

	spreadsheet, err := g.sheetsService.Spreadsheets.Get(copiedFile.Id).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get copied spreadsheet: %w", err)
	}

	result := map[string]string{
		"copied_spreadsheet_id": spreadsheet.SpreadsheetId,
		"title":                 spreadsheet.Properties.Title,
	}

	return domain.Item(result), nil
}

type CopyWorksheetParams struct {
	SourceSpreadsheetID      string `json:"spreadsheet_id"`
	SourceSheetID            string `json:"worksheet_id"`
	DestinationSpreadsheetID string `json:"dest_spreadsheet_id"`
	Title                    string `json:"title"`
}

func (g *GoogleSheetsIntegration) CopyWorksheet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	log.Info().Msg("Copying worksheet")

	var p CopyWorksheetParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	sheetID, err := strconv.ParseInt(p.SourceSheetID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid source sheet id: %w", err)
	}

	copyReq := &sheets.CopySheetToAnotherSpreadsheetRequest{
		DestinationSpreadsheetId: p.DestinationSpreadsheetID,
	}

	copiedSheet, err := g.sheetsService.Spreadsheets.Sheets.CopyTo(p.SourceSpreadsheetID, sheetID, copyReq).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to copy worksheet: %w", err)
	}

	newSheetID := copiedSheet.SheetId

	if p.Title != "" {
		renameReq := &sheets.Request{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{
					SheetId: newSheetID,
					Title:   p.Title,
				},
				Fields: "title",
			},
		}
		batchReq := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{renameReq},
		}
		_, err = g.sheetsService.Spreadsheets.BatchUpdate(p.DestinationSpreadsheetID, batchReq).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to rename copied worksheet: %w", err)
		}
	}

	result := map[string]string{
		"spreadsheet_id":      p.SourceSpreadsheetID,
		"dest_spreadsheet_id": p.DestinationSpreadsheetID,
		"worksheet_id":        p.SourceSheetID,
		"new_worksheet_id":    strconv.FormatInt(newSheetID, 10),
		"title":               p.Title,
	}

	return domain.Item(result), nil
}

type DeleteWorksheetParams struct {
	SpreadsheetID string `json:"spreadsheet_id"`
	WorksheetID   string `json:"worksheet_id"`
}

func (g *GoogleSheetsIntegration) DeleteWorksheet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	log.Info().Msg("Deleting worksheet")

	var p DeleteWorksheetParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	sheetID, err := strconv.Atoi(p.WorksheetID)
	if err != nil {
		return nil, fmt.Errorf("invalid worksheet id: %w", err)
	}

	req := &sheets.Request{
		DeleteSheet: &sheets.DeleteSheetRequest{
			SheetId: int64(sheetID),
		},
	}

	batchReq := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}

	_, err = g.sheetsService.Spreadsheets.BatchUpdate(p.SpreadsheetID, batchReq).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to delete worksheet: %w", err)
	}

	log.Info().Msgf("Deleted worksheet with id %s from spreadsheet %s", p.WorksheetID, p.SpreadsheetID)

	result := map[string]string{
		"spreadsheet_id": p.SpreadsheetID,
		"worksheet_id":   p.WorksheetID,
		"status":         "deleted",
	}

	return domain.Item(result), nil
}

type SearchWorksheetParams struct {
	SpreadsheetID string `json:"spreadsheet_id"`
	WorksheetID   string `json:"worksheet_id"`
	LookupColumn  string `json:"column_id"`
	LookupValue   string `json:"value"`
}

func (g *GoogleSheetsIntegration) FindRows(ctx context.Context, input domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var p SearchWorksheetParams
	if err := g.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings); err != nil {
		return nil, err
	}

	ss, err := g.sheetsService.Spreadsheets.Get(p.SpreadsheetID).Context(ctx).Do()
	if err != nil {
		log.Error().Err(err).Str("spreadsheet_id", p.SpreadsheetID).Msg("DEBUG: FindRows: Failed to get spreadsheet metadata")
		return nil, fmt.Errorf("failed to get spreadsheet metadata: %w", err)
	}

	sheetIDInt, err := strconv.ParseInt(p.WorksheetID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid worksheet id: %w", err)
	}

	var sheetTitle string
	for _, sheet := range ss.Sheets {
		if sheet.Properties.SheetId == sheetIDInt {
			sheetTitle = sheet.Properties.Title
			break
		}
	}
	if sheetTitle == "" {
		return nil, fmt.Errorf("sheet with id %s not found", p.WorksheetID)
	}

	headerRange := fmt.Sprintf("%s!1:1", sheetTitle)
	headerResp, err := g.sheetsService.Spreadsheets.Values.Get(p.SpreadsheetID, headerRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get header row: %w", err)
	}
	if len(headerResp.Values) == 0 || len(headerResp.Values[0]) == 0 {
		return nil, fmt.Errorf("no header found in sheet %s", sheetTitle)
	}
	header := headerResp.Values[0]
	numCols := len(header)
	lastColLetter := columnNumberToLetter(numCols)

	dataRange := fmt.Sprintf("%s!A1:%s", sheetTitle, lastColLetter)
	dataResp, err := g.sheetsService.Spreadsheets.Values.Get(p.SpreadsheetID, dataRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get sheet data: %w", err)
	}
	if len(dataResp.Values) == 0 {
		return []domain.Item{}, nil
	}

	header = dataResp.Values[0]
	var colIndex int = -1
	for i, cell := range header {
		if fmt.Sprintf("%v", cell) == p.LookupColumn {
			colIndex = i
			break
		}
	}
	if colIndex == -1 {
		return nil, fmt.Errorf("lookup column '%s' not found in header", p.LookupColumn)
	}

	var items []domain.Item
	for _, row := range dataResp.Values[1:] {
		if colIndex < len(row) && fmt.Sprintf("%v", row[colIndex]) == p.LookupValue {
			result := map[string]interface{}{
				"spreadsheet_id": p.SpreadsheetID,
				"worksheet_id":   p.WorksheetID,
				"sheet_title":    sheetTitle,
				"lookup_column":  p.LookupColumn,
				"lookup_value":   p.LookupValue,
				"matching_row":   row,
			}
			items = append(items, domain.Item(result))
		}
	}

	return items, nil
}
