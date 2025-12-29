package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	// String operations
	RedisIntegrationActionType_Get    domain.IntegrationActionType = "get"
	RedisIntegrationActionType_Set    domain.IntegrationActionType = "set"
	RedisIntegrationActionType_Del    domain.IntegrationActionType = "del"
	RedisIntegrationActionType_Exists domain.IntegrationActionType = "exists"
	RedisIntegrationActionType_Incr   domain.IntegrationActionType = "incr"
	RedisIntegrationActionType_Decr   domain.IntegrationActionType = "decr"
	RedisIntegrationActionType_Append domain.IntegrationActionType = "append"
	RedisIntegrationActionType_Strlen domain.IntegrationActionType = "strlen"

	// Hash operations
	RedisIntegrationActionType_HGet    domain.IntegrationActionType = "hget"
	RedisIntegrationActionType_HSet    domain.IntegrationActionType = "hset"
	RedisIntegrationActionType_HDel    domain.IntegrationActionType = "hdel"
	RedisIntegrationActionType_HExists domain.IntegrationActionType = "hexists"
	RedisIntegrationActionType_HGetAll domain.IntegrationActionType = "hgetall"
	RedisIntegrationActionType_HKeys   domain.IntegrationActionType = "hkeys"
	RedisIntegrationActionType_HVals   domain.IntegrationActionType = "hvals"
	RedisIntegrationActionType_HLen    domain.IntegrationActionType = "hlen"

	// List operations
	RedisIntegrationActionType_LPush  domain.IntegrationActionType = "lpush"
	RedisIntegrationActionType_RPush  domain.IntegrationActionType = "rpush"
	RedisIntegrationActionType_LPop   domain.IntegrationActionType = "lpop"
	RedisIntegrationActionType_RPop   domain.IntegrationActionType = "rpop"
	RedisIntegrationActionType_LLen   domain.IntegrationActionType = "llen"
	RedisIntegrationActionType_LRange domain.IntegrationActionType = "lrange"
	RedisIntegrationActionType_LIndex domain.IntegrationActionType = "lindex"
	RedisIntegrationActionType_LSet   domain.IntegrationActionType = "lset"

	// Set operations
	RedisIntegrationActionType_SAdd      domain.IntegrationActionType = "sadd"
	RedisIntegrationActionType_SRem      domain.IntegrationActionType = "srem"
	RedisIntegrationActionType_SMembers  domain.IntegrationActionType = "smembers"
	RedisIntegrationActionType_SIsMember domain.IntegrationActionType = "sismember"
	RedisIntegrationActionType_SCard     domain.IntegrationActionType = "scard"
	RedisIntegrationActionType_SPop      domain.IntegrationActionType = "spop"

	// Sorted Set operations
	RedisIntegrationActionType_ZAdd             domain.IntegrationActionType = "zadd"
	RedisIntegrationActionType_ZRem             domain.IntegrationActionType = "zrem"
	RedisIntegrationActionType_ZRange           domain.IntegrationActionType = "zrange"
	RedisIntegrationActionType_ZRevRange        domain.IntegrationActionType = "zrevrange"
	RedisIntegrationActionType_ZRangeByScore    domain.IntegrationActionType = "zrangebyscore"
	RedisIntegrationActionType_ZRevRangeByScore domain.IntegrationActionType = "zrevrangebyscore"
	RedisIntegrationActionType_ZCard            domain.IntegrationActionType = "zcard"
	RedisIntegrationActionType_ZScore           domain.IntegrationActionType = "zscore"
	RedisIntegrationActionType_ZRank            domain.IntegrationActionType = "zrank"

	// Key management
	RedisIntegrationActionType_Keys    domain.IntegrationActionType = "keys"
	RedisIntegrationActionType_Expire  domain.IntegrationActionType = "expire"
	RedisIntegrationActionType_TTL     domain.IntegrationActionType = "ttl"
	RedisIntegrationActionType_Type    domain.IntegrationActionType = "type"
	RedisIntegrationActionType_Rename  domain.IntegrationActionType = "rename"
	RedisIntegrationActionType_Persist domain.IntegrationActionType = "persist"

	// AI Agent Memory
	RedisIntegrationActionType_UseMemory domain.IntegrationActionType = "redis_agent_use_memory"

	// Peekable types
	RedisIntegrationPeekable_Keys      domain.IntegrationPeekableType = "keys"
	RedisIntegrationPeekable_Databases domain.IntegrationPeekableType = "databases"
)

type RedisCredential struct {
	Host          string `json:"host"`
	Port          string `json:"port"`
	Password      string `json:"password"`
	Database      string `json:"database"`
	Username      string `json:"username"`
	TLS           bool   `json:"tls"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`
	TLSServerName string `json:"tls_server_name,omitempty"`
}

type RedisIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[RedisCredential]
	binder           domain.IntegrationParameterBinder
}

func NewRedisIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &RedisIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[RedisCredential](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

func (c *RedisIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewRedisIntegration(ctx, RedisIntegrationDependencies{
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
	})
}

type RedisIntegration struct {
	credentialID     string
	credentialGetter domain.CredentialGetter[RedisCredential]
	binder           domain.IntegrationParameterBinder
	client           *redis.Client
	actionManager    *domain.IntegrationActionManager

	peekFuncs map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type RedisIntegrationDependencies struct {
	CredentialID     string
	CredentialGetter domain.CredentialGetter[RedisCredential]
	ParameterBinder  domain.IntegrationParameterBinder
}

func NewRedisIntegration(ctx context.Context, deps RedisIntegrationDependencies) (*RedisIntegration, error) {
	integration := &RedisIntegration{
		credentialID:     deps.CredentialID,
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(RedisIntegrationActionType_Get, integration.Get).
		AddPerItem(RedisIntegrationActionType_Set, integration.Set).
		AddPerItem(RedisIntegrationActionType_Del, integration.Del).
		AddPerItem(RedisIntegrationActionType_Exists, integration.Exists).
		AddPerItem(RedisIntegrationActionType_Incr, integration.Incr).
		AddPerItem(RedisIntegrationActionType_Decr, integration.Decr).
		AddPerItem(RedisIntegrationActionType_Append, integration.Append).
		AddPerItem(RedisIntegrationActionType_Strlen, integration.Strlen).
		AddPerItem(RedisIntegrationActionType_HGet, integration.HGet).
		AddPerItem(RedisIntegrationActionType_HSet, integration.HSet).
		AddPerItem(RedisIntegrationActionType_HDel, integration.HDel).
		AddPerItem(RedisIntegrationActionType_HExists, integration.HExists).
		AddPerItem(RedisIntegrationActionType_HGetAll, integration.HGetAll).
		AddPerItem(RedisIntegrationActionType_HKeys, integration.HKeys).
		AddPerItem(RedisIntegrationActionType_HVals, integration.HVals).
		AddPerItem(RedisIntegrationActionType_HLen, integration.HLen).
		AddPerItem(RedisIntegrationActionType_LPush, integration.LPush).
		AddPerItem(RedisIntegrationActionType_RPush, integration.RPush).
		AddPerItem(RedisIntegrationActionType_LPop, integration.LPop).
		AddPerItem(RedisIntegrationActionType_RPop, integration.RPop).
		AddPerItem(RedisIntegrationActionType_LLen, integration.LLen).
		AddPerItem(RedisIntegrationActionType_LRange, integration.LRange).
		AddPerItem(RedisIntegrationActionType_LIndex, integration.LIndex).
		AddPerItem(RedisIntegrationActionType_LSet, integration.LSet).
		AddPerItem(RedisIntegrationActionType_SAdd, integration.SAdd).
		AddPerItem(RedisIntegrationActionType_SRem, integration.SRem).
		AddPerItem(RedisIntegrationActionType_SMembers, integration.SMembers).
		AddPerItem(RedisIntegrationActionType_SIsMember, integration.SIsMember).
		AddPerItem(RedisIntegrationActionType_SCard, integration.SCard).
		AddPerItem(RedisIntegrationActionType_SPop, integration.SPop).
		AddPerItem(RedisIntegrationActionType_ZAdd, integration.ZAdd).
		AddPerItem(RedisIntegrationActionType_ZRem, integration.ZRem).
		AddPerItem(RedisIntegrationActionType_ZRange, integration.ZRange).
		AddPerItem(RedisIntegrationActionType_ZRevRange, integration.ZRevRange).
		AddPerItem(RedisIntegrationActionType_ZRangeByScore, integration.ZRangeByScore).
		AddPerItem(RedisIntegrationActionType_ZRevRangeByScore, integration.ZRevRangeByScore).
		AddPerItem(RedisIntegrationActionType_ZCard, integration.ZCard).
		AddPerItem(RedisIntegrationActionType_ZScore, integration.ZScore).
		AddPerItem(RedisIntegrationActionType_ZRank, integration.ZRank).
		AddPerItem(RedisIntegrationActionType_Keys, integration.Keys).
		AddPerItem(RedisIntegrationActionType_Expire, integration.Expire).
		AddPerItem(RedisIntegrationActionType_TTL, integration.TTL).
		AddPerItem(RedisIntegrationActionType_Type, integration.Type).
		AddPerItem(RedisIntegrationActionType_Rename, integration.Rename).
		AddPerItem(RedisIntegrationActionType_Persist, integration.Persist)

	integration.actionManager = actionManager

	// Initialize peek function map
	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error){}

	integration.peekFuncs = peekFuncs

	// Initialize Redis client
	if integration.client == nil {
		credential, err := integration.credentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get Redis credential: %w", err)
		}

		db, err := strconv.Atoi(credential.Database)
		if err != nil {
			return nil, fmt.Errorf("failed to convert database to int: %w", err)
		}

		port, err := strconv.Atoi(credential.Port)
		if err != nil {
			return nil, fmt.Errorf("failed to convert port to int: %w", err)
		}

		client := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", credential.Host, port),
			Password: credential.Password,
			DB:       db,
			Username: credential.Username,
			TLSConfig: func() *tls.Config {
				if credential.TLS {
					serverName := credential.Host
					if credential.TLSServerName != "" {
						serverName = credential.TLSServerName
					}
					return &tls.Config{
						ServerName:         serverName,
						InsecureSkipVerify: credential.TLSSkipVerify,
					}
				}
				return nil
			}(),
		})

		integration.client = client
	}

	return integration, nil
}

func (i *RedisIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

func (i *RedisIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	log.Info().Msgf("Executing Redis integration peek: %s", params.PeekableType)

	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found: %s", params.PeekableType)
	}

	return peekFunc(ctx, params)
}

// String Operations

type GetParams struct {
	Key string `json:"key"`
}

type GetResult struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
	Found bool    `json:"found"`
}

func (i *RedisIntegration) Get(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p GetParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	result, err := i.client.Get(ctx, p.Key).Result()
	if err != nil {
		if err == redis.Nil {
			return GetResult{
				Key:   p.Key,
				Value: nil,
				Found: false,
			}, nil
		}

		return nil, fmt.Errorf("failed to get key %s: %w", p.Key, err)
	}

	return GetResult{
		Key:   p.Key,
		Value: &result,
		Found: true,
	}, nil

}

type SetParams struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Expiration int    `json:"expiration,omitempty"` // seconds
}

func (i *RedisIntegration) Set(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SetParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	var expiration time.Duration

	if p.Expiration > 0 {
		expiration = time.Duration(p.Expiration) * time.Second
	}

	err = i.client.Set(ctx, p.Key, p.Value, expiration).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to set key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":     p.Key,
		"value":   p.Value,
		"success": true,
	}, nil
}

type DelParams struct {
	Keys []string `json:"keys"`
}

func (i *RedisIntegration) Del(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p DelParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	deletedCount, err := i.client.Del(ctx, p.Keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to delete keys %v: %w", p.Keys, err)
	}

	return map[string]interface{}{
		"keys":          p.Keys,
		"deleted_count": deletedCount,
		"success":       true,
	}, nil
}

type ExistsParams struct {
	Keys []string `json:"keys"`
}

func (i *RedisIntegration) Exists(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ExistsParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	count, err := i.client.Exists(ctx, p.Keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check existence of keys %v: %w", p.Keys, err)
	}

	return map[string]any{
		"keys":         p.Keys,
		"exists_count": count,
		"all_exist":    count == int64(len(p.Keys)),
	}, nil
}

type IncrParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) Incr(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p IncrParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	newValue, err := i.client.Incr(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to increment key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":       p.Key,
		"new_value": newValue,
		"success":   true,
	}, nil
}

type DecrParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) Decr(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p DecrParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	newValue, err := i.client.Decr(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to decrement key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":       p.Key,
		"new_value": newValue,
		"success":   true,
	}, nil
}

type AppendParams struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (i *RedisIntegration) Append(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p AppendParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	newLength, err := i.client.Append(ctx, p.Key, p.Value).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to append to key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":        p.Key,
		"new_length": newLength,
		"success":    true,
	}, nil
}

type StrlenParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) Strlen(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p StrlenParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	length, err := i.client.StrLen(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get string length for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":    p.Key,
		"length": length,
	}, nil
}

// Hash Operations

type HGetParams struct {
	Key   string `json:"key"`
	Field string `json:"field"`
}

func (i *RedisIntegration) HGet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HGetParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	result, err := i.client.HGet(ctx, p.Key, p.Field).Result()
	if err != nil {
		if err == redis.Nil {
			return map[string]any{
				"key":   p.Key,
				"field": p.Field,
				"value": nil,
				"found": false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get hash field %s from key %s: %w", p.Field, p.Key, err)
	}

	return map[string]any{
		"key":   p.Key,
		"field": p.Field,
		"value": result,
		"found": true,
	}, nil
}

type HSetParams struct {
	Key        string `json:"key"`
	FieldsJSON string `json:"fields"`
}

func (i *RedisIntegration) HSet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HSetParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	fields := map[string]any{}

	err = json.Unmarshal([]byte(p.FieldsJSON), &fields)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal fields: %w", err)
	}

	fieldsSet, err := i.client.HSet(ctx, p.Key, fields).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to set hash fields for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":        p.Key,
		"fields_set": fieldsSet,
		"success":    true,
	}, nil
}

type HDelParams struct {
	Key    string   `json:"key"`
	Fields []string `json:"fields"`
}

func (i *RedisIntegration) HDel(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HDelParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	deletedCount, err := i.client.HDel(ctx, p.Key, p.Fields...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to delete hash fields %v from key %s: %w", p.Fields, p.Key, err)
	}

	return map[string]any{
		"key":           p.Key,
		"fields":        p.Fields,
		"deleted_count": deletedCount,
		"success":       true,
	}, nil
}

type HExistsParams struct {
	Key   string `json:"key"`
	Field string `json:"field"`
}

func (i *RedisIntegration) HExists(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HExistsParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	exists, err := i.client.HExists(ctx, p.Key, p.Field).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check existence of hash field %s in key %s: %w", p.Field, p.Key, err)
	}

	return map[string]any{
		"key":    p.Key,
		"field":  p.Field,
		"exists": exists,
	}, nil
}

type HGetAllParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) HGetAll(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HGetAllParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	result, err := i.client.HGetAll(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get all hash fields for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":    p.Key,
		"fields": result,
	}, nil
}

type HKeysParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) HKeys(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HKeysParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	keys, err := i.client.HKeys(ctx, p.Key).Result()
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to get hash keys for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":  p.Key,
		"keys": keys,
	}, nil
}

type HValsParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) HVals(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HValsParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	values, err := i.client.HVals(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get hash values for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":    p.Key,
		"values": values,
	}, nil
}

type HLenParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) HLen(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p HLenParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	length, err := i.client.HLen(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get hash length for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":    p.Key,
		"length": length,
	}, nil
}

type LPushParams struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func (i *RedisIntegration) LPush(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p LPushParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Convert []string to []interface{} for Redis client
	values := make([]interface{}, len(p.Values))
	for i, v := range p.Values {
		values[i] = v
	}

	newLength, err := i.client.LPush(ctx, p.Key, values...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to left push to list %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":        p.Key,
		"values":     p.Values,
		"new_length": newLength,
		"success":    true,
	}, nil
}

type RPushParams struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func (i *RedisIntegration) RPush(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p RPushParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Convert []string to []interface{} for Redis client
	values := make([]interface{}, len(p.Values))
	for i, v := range p.Values {
		values[i] = v
	}

	newLength, err := i.client.RPush(ctx, p.Key, values...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to right push to list %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":        p.Key,
		"values":     p.Values,
		"new_length": newLength,
		"success":    true,
	}, nil
}

type LPopParams struct {
	Key   string `json:"key"`
	Count int    `json:"count,omitempty"` // Default 1
}

func (i *RedisIntegration) LPop(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p LPopParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	if p.Count <= 0 {
		p.Count = 1
	}

	var result interface{}
	var redisErr error

	if p.Count == 1 {
		result, redisErr = i.client.LPop(ctx, p.Key).Result()
	} else {
		result, redisErr = i.client.LPopCount(ctx, p.Key, p.Count).Result()
	}

	if redisErr != nil {
		if redisErr == redis.Nil {
			return map[string]interface{}{
				"key":    p.Key,
				"values": nil,
				"found":  false,
			}, nil
		}

		return nil, fmt.Errorf("failed to left pop from list %s: %w", p.Key, redisErr)
	}

	return map[string]interface{}{
		"key":    p.Key,
		"values": result,
		"found":  true,
	}, nil

}

type RPopParams struct {
	Key   string `json:"key"`
	Count int    `json:"count,omitempty"` // Default 1
}

func (i *RedisIntegration) RPop(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p RPopParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Count <= 0 {
		p.Count = 1
	}

	var result interface{}
	var redisErr error

	if p.Count == 1 {
		result, redisErr = i.client.RPop(ctx, p.Key).Result()
	} else {
		result, redisErr = i.client.RPopCount(ctx, p.Key, p.Count).Result()
	}

	if redisErr != nil {
		if redisErr == redis.Nil {
			return map[string]interface{}{
				"key":    p.Key,
				"values": nil,
				"found":  false,
			}, nil
		}

		return nil, fmt.Errorf("failed to right pop from list %s: %w", p.Key, redisErr)
	}

	return map[string]interface{}{
		"key":    p.Key,
		"values": result,
		"found":  true,
	}, nil
}

type LLenParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) LLen(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p LLenParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	length, err := i.client.LLen(ctx, p.Key).Result()
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to get list length for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":    p.Key,
		"length": length,
	}, nil
}

type LRangeParams struct {
	Key   string `json:"key"`
	Start int64  `json:"start"`
	Stop  int64  `json:"stop"`
}

func (i *RedisIntegration) LRange(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p LRangeParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	values, err := i.client.LRange(ctx, p.Key, p.Start, p.Stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get list range for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":    p.Key,
		"start":  p.Start,
		"stop":   p.Stop,
		"values": values,
	}, nil
}

type LIndexParams struct {
	Key   string `json:"key"`
	Index int64  `json:"index"`
}

func (i *RedisIntegration) LIndex(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p LIndexParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	value, err := i.client.LIndex(ctx, p.Key, p.Index).Result()
	if err != nil {
		if err == redis.Nil {
			return map[string]interface{}{
				"key":   p.Key,
				"index": p.Index,
				"value": nil,
				"found": false,
			}, nil
		}

		return nil, fmt.Errorf("failed to get list index %d for key %s: %w", p.Index, p.Key, err)
	}

	return map[string]interface{}{
		"key":   p.Key,
		"index": p.Index,
		"value": value,
		"found": true,
	}, nil
}

type LSetParams struct {
	Key   string `json:"key"`
	Index int64  `json:"index"`
	Value string `json:"value"`
}

func (i *RedisIntegration) LSet(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p LSetParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	err = i.client.LSet(ctx, p.Key, p.Index, p.Value).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to set list index %d for key %s: %w", p.Index, p.Key, err)
	}

	return map[string]interface{}{
		"key":     p.Key,
		"index":   p.Index,
		"value":   p.Value,
		"success": true,
	}, nil
}

// Set Operations

type SAddParams struct {
	Key     string   `json:"key"`
	Members []string `json:"members"`
}

func (i *RedisIntegration) SAdd(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SAddParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Convert []string to []interface{} for Redis client
	members := make([]interface{}, len(p.Members))
	for i, m := range p.Members {
		members[i] = m
	}

	addedCount, err := i.client.SAdd(ctx, p.Key, members...).Result()
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to add members to set %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":         p.Key,
		"members":     p.Members,
		"added_count": addedCount,
		"success":     true,
	}, nil
}

type SRemParams struct {
	Key     string   `json:"key"`
	Members []string `json:"members"`
}

func (i *RedisIntegration) SRem(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SRemParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Convert []string to []interface{} for Redis client
	members := make([]interface{}, len(p.Members))
	for i, m := range p.Members {
		members[i] = m
	}

	removedCount, err := i.client.SRem(ctx, p.Key, members...).Result()
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to remove members from set %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":           p.Key,
		"members":       p.Members,
		"removed_count": removedCount,
		"success":       true,
	}, nil
}

type SMembersParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) SMembers(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SMembersParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	members, err := i.client.SMembers(ctx, p.Key).Result()
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to get set members for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":     p.Key,
		"members": members,
	}, nil
}

type SIsMemberParams struct {
	Key    string `json:"key"`
	Member string `json:"member"`
}

func (i *RedisIntegration) SIsMember(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SIsMemberParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	isMember, err := i.client.SIsMember(ctx, p.Key, p.Member).Result()
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to check set membership for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":       p.Key,
		"member":    p.Member,
		"is_member": isMember,
	}, nil
}

type SCardParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) SCard(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SCardParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	cardinality, err := i.client.SCard(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get set cardinality for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":         p.Key,
		"cardinality": cardinality,
	}, nil
}

type SPopParams struct {
	Key   string `json:"key"`
	Count int64  `json:"count,omitempty"` // Default 1
}

func (i *RedisIntegration) SPop(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p SPopParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Count <= 0 {
		p.Count = 1
	}

	var result interface{}
	var redisErr error

	if p.Count == 1 {
		result, redisErr = i.client.SPop(ctx, p.Key).Result()
	} else {
		result, redisErr = i.client.SPopN(ctx, p.Key, p.Count).Result()
	}

	if redisErr != nil {
		if redisErr == redis.Nil {
			return map[string]interface{}{
				"key":     p.Key,
				"members": nil,
				"found":   true,
			}, nil
		}

		return nil, fmt.Errorf("failed to pop from set %s: %w", p.Key, redisErr)
	}

	return map[string]interface{}{
		"key":     p.Key,
		"members": result,
		"found":   true,
	}, nil
}

// Sorted Set Operations

type ZAddParams struct {
	Key     string    `json:"key"`
	Members []redis.Z `json:"members"`
}

type ZMember struct {
	Score  float64 `json:"score"`
	Member string  `json:"member"`
}

type ZAddParamsInput struct {
	Key     string    `json:"key"`
	Members []ZMember `json:"members"`
}

func (i *RedisIntegration) ZAdd(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZAddParamsInput

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Convert to redis.Z format
	members := make([]redis.Z, len(p.Members))
	for i, m := range p.Members {
		members[i] = redis.Z{
			Score:  m.Score,
			Member: m.Member,
		}
	}

	addedCount, err := i.client.ZAdd(ctx, p.Key, members...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to add members to sorted set %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":         p.Key,
		"members":     p.Members,
		"added_count": addedCount,
		"success":     true,
	}, nil
}

type ZRemParams struct {
	Key     string   `json:"key"`
	Members []string `json:"members"`
}

func (i *RedisIntegration) ZRem(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZRemParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Convert []string to []interface{} for Redis client
	members := make([]interface{}, len(p.Members))
	for i, m := range p.Members {
		members[i] = m
	}

	removedCount, err := i.client.ZRem(ctx, p.Key, members...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to remove members from sorted set %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":           p.Key,
		"members":       p.Members,
		"removed_count": removedCount,
		"success":       true,
	}, nil
}

type ZRangeParams struct {
	Key   string `json:"key"`
	Start int64  `json:"start"`
	Stop  int64  `json:"stop"`
}

func (i *RedisIntegration) ZRange(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZRangeParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	members, err := i.client.ZRange(ctx, p.Key, p.Start, p.Stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get sorted set range for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":     p.Key,
		"start":   p.Start,
		"stop":    p.Stop,
		"members": members,
	}, nil
}

type ZRevRangeParams struct {
	Key   string `json:"key"`
	Start int64  `json:"start"`
	Stop  int64  `json:"stop"`
}

func (i *RedisIntegration) ZRevRange(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZRevRangeParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	members, err := i.client.ZRevRange(ctx, p.Key, p.Start, p.Stop).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get sorted set reverse range for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":     p.Key,
		"start":   p.Start,
		"stop":    p.Stop,
		"members": members,
	}, nil
}

type ZRangeByScoreParams struct {
	Key string `json:"key"`
	Min string `json:"min"`
	Max string `json:"max"`
}

func (i *RedisIntegration) ZRangeByScore(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZRangeByScoreParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	members, err := i.client.ZRangeByScore(ctx, p.Key, &redis.ZRangeBy{
		Min: p.Min,
		Max: p.Max,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get sorted set range by score for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":     p.Key,
		"min":     p.Min,
		"max":     p.Max,
		"members": members,
	}, nil
}

type ZRevRangeByScoreParams struct {
	Key string `json:"key"`
	Min string `json:"min"`
	Max string `json:"max"`
}

func (i *RedisIntegration) ZRevRangeByScore(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZRevRangeByScoreParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	members, err := i.client.ZRevRangeByScore(ctx, p.Key, &redis.ZRangeBy{
		Min: p.Min,
		Max: p.Max,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get sorted set reverse range by score for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":     p.Key,
		"min":     p.Min,
		"max":     p.Max,
		"members": members,
	}, nil
}

type ZCardParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) ZCard(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZCardParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	cardinality, err := i.client.ZCard(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get sorted set cardinality for key %s: %w", p.Key, err)
	}

	return map[string]interface{}{
		"key":         p.Key,
		"cardinality": cardinality,
	}, nil
}

type ZScoreParams struct {
	Key    string `json:"key"`
	Member string `json:"member"`
}

func (i *RedisIntegration) ZScore(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZScoreParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	score, err := i.client.ZScore(ctx, p.Key, p.Member).Result()
	if err != nil {
		if err == redis.Nil {
			return map[string]any{
				"key":    p.Key,
				"member": p.Member,
				"score":  nil,
				"found":  true,
			}, nil
		}

		return nil, fmt.Errorf("failed to get score for member %s in sorted set %s: %w", p.Member, p.Key, err)
	}

	return map[string]any{
		"key":    p.Key,
		"member": p.Member,
		"score":  score,
		"found":  true,
	}, nil
}

type ZRankParams struct {
	Key    string `json:"key"`
	Member string `json:"member"`
}

func (i *RedisIntegration) ZRank(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ZRankParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	rank, err := i.client.ZRank(ctx, p.Key, p.Member).Result()
	if err != nil {
		if err == redis.Nil {
			return map[string]any{
				"key":    p.Key,
				"member": p.Member,
				"rank":   nil,
				"found":  false,
			}, nil
		}

		return nil, fmt.Errorf("failed to get rank for member %s in sorted set %s: %w", p.Member, p.Key, err)
	}

	return map[string]any{
		"key":    p.Key,
		"member": p.Member,
		"rank":   rank,
		"found":  true,
	}, nil
}

// Key Management Operations

type KeysParams struct {
	Pattern string `json:"pattern"`
}

func (i *RedisIntegration) Keys(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p KeysParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if p.Pattern == "" {
		p.Pattern = "*"
	}

	keys, err := i.client.Keys(ctx, p.Pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys with pattern %s: %w", p.Pattern, err)
	}

	return map[string]any{
		"pattern": p.Pattern,
		"keys":    keys,
		"count":   len(keys),
	}, nil
}

type ExpireParams struct {
	Key        string `json:"key"`
	Expiration int    `json:"expiration"` // seconds
}

func (i *RedisIntegration) Expire(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p ExpireParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	success, err := i.client.Expire(ctx, p.Key, time.Duration(p.Expiration)*time.Second).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to set expiration for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":        p.Key,
		"expiration": p.Expiration,
		"success":    success,
	}, nil
}

type TTLParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) TTL(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p TTLParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	ttl, err := i.client.TTL(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get TTL for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key": p.Key,
		"ttl": int64(ttl.Seconds()),
	}, nil
}

type TypeParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) Type(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p TypeParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	keyType, err := i.client.Type(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get type for key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":  p.Key,
		"type": keyType,
	}, nil
}

type RenameParams struct {
	OldKey string `json:"old_key"`
	NewKey string `json:"new_key"`
}

func (i *RedisIntegration) Rename(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p RenameParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	err = i.client.Rename(ctx, p.OldKey, p.NewKey).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to rename key %s to %s: %w", p.OldKey, p.NewKey, err)
	}

	return map[string]any{
		"old_key": p.OldKey,
		"new_key": p.NewKey,
		"success": true,
	}, nil
}

type PersistParams struct {
	Key string `json:"key"`
}

func (i *RedisIntegration) Persist(ctx context.Context, input domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	var p PersistParams

	err := i.binder.BindToStruct(ctx, item, &p, input.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	success, err := i.client.Persist(ctx, p.Key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to persist key %s: %w", p.Key, err)
	}

	return map[string]any{
		"key":     p.Key,
		"success": success,
	}, nil
}
