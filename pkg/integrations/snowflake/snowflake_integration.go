package snowflake

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"regexp"
	"strings"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/snowflakedb/gosnowflake"
	"github.com/youmark/pkcs8"
)

var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateIdentifier checks that an identifier only contains safe characters
func validateIdentifier(id string) error {
	if !identifierRegex.MatchString(id) {
		return fmt.Errorf("invalid identifier %q: must start with letter/underscore and contain only alphanumeric/underscore", id)
	}
	return nil
}

// quoteIdentifier safely quotes an identifier for Snowflake SQL
func quoteIdentifier(id string) string {
	escaped := strings.ReplaceAll(id, `"`, `""`)
	return `"` + escaped + `"`
}

const (
	IntegrationActionType_ExecuteQuery domain.IntegrationActionType = "execute_query"
	IntegrationActionType_Insert       domain.IntegrationActionType = "insert"
	IntegrationActionType_Update       domain.IntegrationActionType = "update"
)

const (
	SnowflakePeekable_Databases domain.IntegrationPeekableType = "databases"
	SnowflakePeekable_Schemas   domain.IntegrationPeekableType = "schemas"
	SnowflakePeekable_Tables    domain.IntegrationPeekableType = "tables"
)

type SnowflakeCredential struct {
	Account              string `json:"account"`
	Username             string `json:"username"`
	AuthType             string `json:"auth_type"`
	Password             string `json:"password,omitempty"`
	PrivateKey           string `json:"private_key,omitempty"`
	PrivateKeyPassphrase string `json:"private_key_passphrase,omitempty"`
	Warehouse            string `json:"warehouse"`
	Database             string `json:"database,omitempty"`
	Schema               string `json:"schema,omitempty"`
	Role                 string `json:"role,omitempty"`
}

func (c *SnowflakeCredential) BuildDSN() (string, error) {
	cfg := &gosnowflake.Config{
		Account:   c.Account,
		User:      c.Username,
		Warehouse: c.Warehouse,
	}

	if c.Database != "" {
		cfg.Database = c.Database
	}

	if c.Schema != "" {
		cfg.Schema = c.Schema
	}

	if c.Role != "" {
		cfg.Role = c.Role
	}

	if c.AuthType == "key_pair" {
		privateKey, err := parsePrivateKey(c.PrivateKey, c.PrivateKeyPassphrase)
		if err != nil {
			return "", fmt.Errorf("failed to parse private key: %w", err)
		}
		cfg.Authenticator = gosnowflake.AuthTypeJwt
		cfg.PrivateKey = privateKey
	} else {
		cfg.Password = c.Password
	}

	dsn, err := gosnowflake.DSN(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to build DSN: %w", err)
	}

	return dsn, nil
}

func parsePrivateKey(privateKeyPEM string, passphrase string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	var parsedKey interface{}
	var err error

	switch block.Type {
	case "ENCRYPTED PRIVATE KEY":
		parsedKey, err = pkcs8.ParsePKCS8PrivateKey(block.Bytes, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt PKCS8 private key: %w", err)
		}
	case "PRIVATE KEY":
		parsedKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
	case "RSA PRIVATE KEY":
		if x509.IsEncryptedPEMBlock(block) {
			decryptedBytes, err := x509.DecryptPEMBlock(block, []byte(passphrase))
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt RSA private key: %w", err)
			}
			parsedKey, err = x509.ParsePKCS1PrivateKey(decryptedBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse decrypted RSA private key: %w", err)
			}
		} else {
			parsedKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}

	rsaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA type")
	}

	return rsaKey, nil
}

type SnowflakeIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[SnowflakeCredential]
	binder           domain.IntegrationParameterBinder
}

func NewSnowflakeIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &SnowflakeIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[SnowflakeCredential](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

func (c *SnowflakeIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewSnowflakeIntegration(ctx, SnowflakeIntegrationDependencies{
		CredentialID:     p.CredentialID,
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
	})
}

type SnowflakeIntegrationDependencies struct {
	CredentialID     string
	CredentialGetter domain.CredentialGetter[SnowflakeCredential]
	ParameterBinder  domain.IntegrationParameterBinder
}

type SnowflakeIntegration struct {
	credentialGetter domain.CredentialGetter[SnowflakeCredential]
	binder           domain.IntegrationParameterBinder
	db               *sql.DB
	actionManager    *domain.IntegrationActionManager
	peekFuncs        map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

func NewSnowflakeIntegration(ctx context.Context, deps SnowflakeIntegrationDependencies) (*SnowflakeIntegration, error) {
	integration := &SnowflakeIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
		actionManager:    domain.NewIntegrationActionManager(),
	}

	integration.actionManager.
		AddPerItemMulti(IntegrationActionType_ExecuteQuery, integration.ExecuteQuery).
		AddPerItem(IntegrationActionType_Insert, integration.Insert).
		AddPerItem(IntegrationActionType_Update, integration.Update)

	integration.peekFuncs = map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error){
		SnowflakePeekable_Databases: integration.PeekDatabases,
		SnowflakePeekable_Schemas:   integration.PeekSchemas,
		SnowflakePeekable_Tables:    integration.PeekTables,
	}

	credential, err := integration.credentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	dsn, err := credential.BuildDSN()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open snowflake connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping snowflake: %w", err)
	}

	integration.db = db

	return integration, nil
}

func (i *SnowflakeIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type ExecuteQueryParams struct {
	Query string `json:"query"`
}

func (i *SnowflakeIntegration) ExecuteQuery(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	var p ExecuteQueryParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	rows, err := i.db.QueryContext(ctx, p.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []domain.Item

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for idx := range columns {
			valuePtrs[idx] = &values[idx]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]any)
		for idx, col := range columns {
			val := values[idx]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

type InsertParams struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
	Data     string `json:"data"`
}

func (i *SnowflakeIntegration) Insert(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p InsertParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Validate identifiers to prevent SQL injection
	if err := validateIdentifier(p.Database); err != nil {
		return nil, fmt.Errorf("invalid database: %w", err)
	}
	if err := validateIdentifier(p.Schema); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	if err := validateIdentifier(p.Table); err != nil {
		return nil, fmt.Errorf("invalid table: %w", err)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(p.Data), &data); err != nil {
		return nil, fmt.Errorf("failed to parse data JSON: %w", err)
	}

	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]any, 0, len(data))

	for col, val := range data {
		columns = append(columns, quoteIdentifier(col))
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s.%s.%s (%s) VALUES (%s)",
		quoteIdentifier(p.Database),
		quoteIdentifier(p.Schema),
		quoteIdentifier(p.Table),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	result, err := i.db.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert data: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return map[string]any{
		"success":       true,
		"rows_affected": rowsAffected,
	}, nil
}

type UpdateCondition struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type UpdateParams struct {
	Database   string            `json:"database"`
	Schema     string            `json:"schema"`
	Table      string            `json:"table"`
	Data       string            `json:"data"`
	Conditions []UpdateCondition `json:"conditions"`
}

// validOperators defines the allowed SQL operators for WHERE conditions
var validOperators = map[string]bool{
	"=":           true,
	"!=":          true,
	">":           true,
	"<":           true,
	">=":          true,
	"<=":          true,
	"LIKE":        true,
	"NOT LIKE":    true,
	"IS NULL":     true,
	"IS NOT NULL": true,
}

func (i *SnowflakeIntegration) Update(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p UpdateParams

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Validate identifiers to prevent SQL injection
	if err := validateIdentifier(p.Database); err != nil {
		return nil, fmt.Errorf("invalid database: %w", err)
	}
	if err := validateIdentifier(p.Schema); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	if err := validateIdentifier(p.Table); err != nil {
		return nil, fmt.Errorf("invalid table: %w", err)
	}

	// Validate conditions
	if len(p.Conditions) == 0 {
		return nil, fmt.Errorf("at least one condition is required")
	}

	for idx, cond := range p.Conditions {
		if err := validateIdentifier(cond.Column); err != nil {
			return nil, fmt.Errorf("invalid column in condition %d: %w", idx+1, err)
		}
		if !validOperators[cond.Operator] {
			return nil, fmt.Errorf("invalid operator in condition %d: %q", idx+1, cond.Operator)
		}
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(p.Data), &data); err != nil {
		return nil, fmt.Errorf("failed to parse data JSON: %w", err)
	}

	setClauses := make([]string, 0, len(data))
	values := make([]any, 0, len(data)+len(p.Conditions))

	for col, val := range data {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteIdentifier(col)))
		values = append(values, val)
	}

	// Build WHERE clause from conditions
	whereClauses := make([]string, 0, len(p.Conditions))
	for _, cond := range p.Conditions {
		if cond.Operator == "IS NULL" || cond.Operator == "IS NOT NULL" {
			whereClauses = append(whereClauses, fmt.Sprintf("%s %s", quoteIdentifier(cond.Column), cond.Operator))
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("%s %s ?", quoteIdentifier(cond.Column), cond.Operator))
			values = append(values, cond.Value)
		}
	}

	query := fmt.Sprintf(
		"UPDATE %s.%s.%s SET %s WHERE %s",
		quoteIdentifier(p.Database),
		quoteIdentifier(p.Schema),
		quoteIdentifier(p.Table),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)

	result, err := i.db.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, fmt.Errorf("failed to update data: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return map[string]any{
		"success":       true,
		"rows_affected": rowsAffected,
	}, nil
}

func (i *SnowflakeIntegration) Peek(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[p.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peekable type not found: %s", p.PeekableType)
	}

	return peekFunc(ctx, p)
}

func (i *SnowflakeIntegration) PeekDatabases(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	rows, err := i.db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return domain.PeekResult{}, err
	}

	nameIdx := -1
	for idx, col := range columns {
		if col == "name" {
			nameIdx = idx
			break
		}
	}

	var results []domain.PeekResultItem

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for idx := range columns {
			valuePtrs[idx] = &values[idx]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return domain.PeekResult{}, err
		}

		var dbName string
		if nameIdx >= 0 && values[nameIdx] != nil {
			dbName = fmt.Sprintf("%v", values[nameIdx])
		}

		if dbName != "" {
			results = append(results, domain.PeekResultItem{
				Key:     dbName,
				Value:   dbName,
				Content: dbName,
			})
		}
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekSchemasParams struct {
	Database string `json:"database"`
}

func (i *SnowflakeIntegration) PeekSchemas(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	var params PeekSchemasParams

	if err := json.Unmarshal(p.PayloadJSON, &params); err != nil {
		return domain.PeekResult{}, err
	}

	if err := validateIdentifier(params.Database); err != nil {
		return domain.PeekResult{}, fmt.Errorf("invalid database: %w", err)
	}

	query := fmt.Sprintf("SHOW SCHEMAS IN DATABASE %s", quoteIdentifier(params.Database))
	rows, err := i.db.QueryContext(ctx, query)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return domain.PeekResult{}, err
	}

	nameIdx := -1
	for idx, col := range columns {
		if col == "name" {
			nameIdx = idx
			break
		}
	}

	var results []domain.PeekResultItem

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for idx := range columns {
			valuePtrs[idx] = &values[idx]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return domain.PeekResult{}, err
		}

		var schemaName string
		if nameIdx >= 0 && values[nameIdx] != nil {
			schemaName = fmt.Sprintf("%v", values[nameIdx])
		}

		if schemaName != "" {
			results = append(results, domain.PeekResultItem{
				Key:     schemaName,
				Value:   schemaName,
				Content: schemaName,
			})
		}
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

type PeekTablesParams struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
}

func (i *SnowflakeIntegration) PeekTables(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error) {
	var params PeekTablesParams

	if err := json.Unmarshal(p.PayloadJSON, &params); err != nil {
		return domain.PeekResult{}, err
	}

	if err := validateIdentifier(params.Database); err != nil {
		return domain.PeekResult{}, fmt.Errorf("invalid database: %w", err)
	}
	if err := validateIdentifier(params.Schema); err != nil {
		return domain.PeekResult{}, fmt.Errorf("invalid schema: %w", err)
	}

	query := fmt.Sprintf("SHOW TABLES IN %s.%s", quoteIdentifier(params.Database), quoteIdentifier(params.Schema))
	rows, err := i.db.QueryContext(ctx, query)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return domain.PeekResult{}, err
	}

	nameIdx := -1
	for idx, col := range columns {
		if col == "name" {
			nameIdx = idx
			break
		}
	}

	var results []domain.PeekResultItem

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for idx := range columns {
			valuePtrs[idx] = &values[idx]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return domain.PeekResult{}, err
		}

		var tableName string
		if nameIdx >= 0 && values[nameIdx] != nil {
			tableName = fmt.Sprintf("%v", values[nameIdx])
		}

		if tableName != "" {
			results = append(results, domain.PeekResultItem{
				Key:     tableName,
				Value:   tableName,
				Content: tableName,
			})
		}
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}
