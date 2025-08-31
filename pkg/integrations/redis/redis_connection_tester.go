package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type RedisConnectionTester struct {
	credentialGetter domain.CredentialGetter[RedisCredential]
}

func NewRedisConnectionTester(deps domain.IntegrationDeps) domain.IntegrationConnectionTester {
	return &RedisConnectionTester{
		credentialGetter: managers.NewExecutorCredentialGetter[RedisCredential](deps.ExecutorCredentialManager),
	}
}

func (c *RedisConnectionTester) TestConnection(ctx context.Context, params domain.TestConnectionParams) (bool, error) {
	data, err := json.Marshal(params.Credential.DecryptedPayload)
	if err != nil {
		return false, err
	}

	var redisCredential RedisCredential

	if err := json.Unmarshal(data, &redisCredential); err != nil {
		return false, err
	}

	log.Info().Msgf("Testing connection to Redis at %s:%s with database %s, TLS %t, TLS Skip Verify %t, TLS Server Name %s", redisCredential.Host, redisCredential.Port, redisCredential.Database, redisCredential.TLS, redisCredential.TLSSkipVerify, redisCredential.TLSServerName)

	// Convert database string to int
	db, err := strconv.Atoi(redisCredential.Database)
	if err != nil {
		return false, fmt.Errorf("database is not a number")
	}

	// Convert port string to int
	port, err := strconv.Atoi(redisCredential.Port)
	if err != nil {
		return false, fmt.Errorf("port is not a number")
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisCredential.Host, port),
		Password: redisCredential.Password,
		DB:       db,
		Username: redisCredential.Username,
		TLSConfig: func() *tls.Config {
			if redisCredential.TLS {
				serverName := redisCredential.Host
				if redisCredential.TLSServerName != "" {
					serverName = redisCredential.TLSServerName
				}
				return &tls.Config{
					ServerName:         serverName,
					InsecureSkipVerify: redisCredential.TLSSkipVerify,
				}
			}
			return nil
		}(),
	})

	// Test the connection with a simple ping
	_, err = client.Ping(ctx).Result()
	if err != nil {
		return false, fmt.Errorf("failed to ping Redis: %w", err)
	}

	// Close the client
	err = client.Close()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to close Redis client during connection test")
	}

	return true, nil
}
