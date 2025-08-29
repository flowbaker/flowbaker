package postgresql

import (
	"context"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"

	"github.com/jackc/pgx/v5"
)

const (
	IntegrationActionType_ExecuteQuery domain.IntegrationActionType = "execute_query"
	IntegrationActionType_Delete       domain.IntegrationActionType = "delete"
	IntegrationActionType_Insert       domain.IntegrationActionType = "insert"
	IntegrationActionType_Select       domain.IntegrationActionType = "select"
	IntegrationActionType_Upsert       domain.IntegrationActionType = "upsert"
	IntegrationActionType_Update       domain.IntegrationActionType = "update"
)

const (
	PostgreSQLPeekable_Tables  domain.IntegrationPeekableType = "tables"
	PostgreSQLPeekable_Columns domain.IntegrationPeekableType = "columns"
	PostgreSQLPeekable_Schemas domain.IntegrationPeekableType = "schemas"
)

type PostgreSQLIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[PostgreSQLCredential]
	binder           domain.IntegrationParameterBinder
}

func NewPostgreSQLIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &PostgreSQLIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[PostgreSQLCredential](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

type PostgreSQLCredential struct {
	URI string `json:"uri"`
}

func (c *PostgreSQLIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewPostgreSQLIntegration(ctx, PostgreSQLIntegrationDependencies{
		CredentialID:     p.CredentialID,
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
	})
}

type PostgreSQLIntegration struct {
	credentialGetter domain.CredentialGetter[PostgreSQLCredential]
	binder           domain.IntegrationParameterBinder
	conn             *pgx.Conn
	actionManager    *domain.IntegrationActionManager
}

type PostgreSQLIntegrationDependencies struct {
	CredentialID     string
	CredentialGetter domain.CredentialGetter[PostgreSQLCredential]
	ParameterBinder  domain.IntegrationParameterBinder
}

func NewPostgreSQLIntegration(ctx context.Context, deps PostgreSQLIntegrationDependencies) (*PostgreSQLIntegration, error) {
	integration := &PostgreSQLIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
		actionManager:    domain.NewIntegrationActionManager(),
	}

	integration.actionManager.AddPerItemMulti(IntegrationActionType_ExecuteQuery, integration.ExecuteQuery)

	credential, err := integration.credentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if integration.conn == nil {
		if err != nil {
			return nil, err
		}

		conn, err := pgx.Connect(ctx, credential.URI)
		if err != nil {
			return nil, err
		}

		integration.conn = conn
	}

	return integration, nil
}

func (i *PostgreSQLIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type Query struct {
	Content          string `json:"content"`
	Query_Parameters []any  `json:"query_parameters"`
}

type PostgreSQLParams struct {
	Query Query `json:"query"`
}

func (i *PostgreSQLIntegration) ExecuteQuery(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := PostgreSQLParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	rows, err := i.conn.Query(ctx, p.Query.Content, p.Query.Query_Parameters...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, value := range values {
			row[rows.FieldDescriptions()[i].Name] = value
		}

		results = append(results, row)
	}

	items := make([]domain.Item, len(results))

	for _, result := range results {
		items = append(items, result)
	}

	return items, nil
}
