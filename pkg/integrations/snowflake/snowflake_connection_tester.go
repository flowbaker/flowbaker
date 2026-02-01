package snowflake

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/domain"
	_ "github.com/snowflakedb/gosnowflake"
)

type SnowflakeConnectionTester struct {
	credentialGetter domain.CredentialGetter[SnowflakeCredential]
}

func NewSnowflakeConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &SnowflakeConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[SnowflakeCredential](deps.ExecutorCredentialManager),
	}
}

func (c *SnowflakeConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	data, err := json.Marshal(params.Credential.DecryptedPayload)
	if err != nil {
		return false, err
	}

	var credential SnowflakeCredential

	if err := json.Unmarshal(data, &credential); err != nil {
		return false, err
	}

	dsn, err := credential.BuildDSN()
	if err != nil {
		return false, fmt.Errorf("failed to build DSN: %w", err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return false, fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return false, fmt.Errorf("failed to ping Snowflake: %w", err)
	}

	return true, nil
}
