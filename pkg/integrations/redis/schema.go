package redis

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                domain.IntegrationType_Redis,
		Name:              "Redis",
		Description:       "Use Redis integration to perform database operations including string, hash, list, set, sorted set operations and key management.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "host",
				Name:        "Host",
				Description: "Redis server hostname or IP address",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "port",
				Name:        "Port",
				Description: "Redis server port (default: 6379)",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "password",
				Name:        "Password",
				Description: "Redis server password (leave empty if no password)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "database",
				Name:        "Database",
				Description: "Redis database number (0-15, default: 0)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "username",
				Name:        "Username",
				Description: "Redis username (for Redis 6.0+ ACL)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "tls",
				Name:        "TLS",
				Description: "Enable TLS/SSL connection",
				Required:    false,
				Type:        domain.NodePropertyType_Boolean,
			},
			{
				Key:         "tls_skip_verify",
				Name:        "TLS Skip Verify",
				Description: "Skip TLS certificate verification (use only for development/testing)",
				Required:    false,
				Type:        domain.NodePropertyType_Boolean,
				DependsOn: &domain.DependsOn{
					PropertyKey: "tls",
					Value:       true,
				},
			},
			{
				Key:         "tls_server_name",
				Name:        "TLS Server Name",
				Description: "Custom server name for TLS certificate validation (optional, defaults to host)",
				Required:    false,
				Type:        domain.NodePropertyType_String,
				DependsOn: &domain.DependsOn{
					PropertyKey: "tls",
					Value:       true,
				},
			},
		},
		Actions: []domain.IntegrationAction{
			// String Operations
			{
				ID:          "get",
				Name:        "Get String Value",
				ActionType:  RedisIntegrationActionType_Get,
				Description: "Get the value of a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to get the value for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "set",
				Name:        "Set String Value",
				ActionType:  RedisIntegrationActionType_Set,
				Description: "Set the value of a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to set",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "value",
						Name:        "Value",
						Description: "The value to set",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "expiration",
						Name:        "Expiration (seconds)",
						Description: "Optional expiration time in seconds",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "del",
				Name:        "Delete Keys",
				ActionType:  RedisIntegrationActionType_Del,
				Description: "Delete one or more keys",
				Properties: []domain.NodeProperty{
					{
						Key:         "keys",
						Name:        "Keys",
						Description: "Array of keys to delete",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key to delete",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "exists",
				Name:        "Check Key Existence",
				ActionType:  RedisIntegrationActionType_Exists,
				Description: "Check if one or more keys exist",
				Properties: []domain.NodeProperty{
					{
						Key:         "keys",
						Name:        "Keys",
						Description: "Array of keys to check",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "key",
									Name:        "Key",
									Description: "The key to check",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "incr",
				Name:        "Increment",
				ActionType:  RedisIntegrationActionType_Incr,
				Description: "Increment the integer value of a key by one",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to increment",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "decr",
				Name:        "Decrement",
				ActionType:  RedisIntegrationActionType_Decr,
				Description: "Decrement the integer value of a key by one",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to decrement",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "append",
				Name:        "Append to String",
				ActionType:  RedisIntegrationActionType_Append,
				Description: "Append a value to a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to append to",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "value",
						Name:        "Value",
						Description: "The value to append",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "strlen",
				Name:        "String Length",
				ActionType:  RedisIntegrationActionType_Strlen,
				Description: "Get the length of the value stored in a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to get the length for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},

			// Hash Operations
			{
				ID:          "hget",
				Name:        "Get Hash Field",
				ActionType:  RedisIntegrationActionType_HGet,
				Description: "Get the value of a hash field",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "field",
						Name:        "Field",
						Description: "The field name",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "hset",
				Name:        "Set Hash Fields",
				ActionType:  RedisIntegrationActionType_HSet,
				Description: "Set the value of hash fields",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:          "fields",
						Name:         "Fields",
						Description:  "Object with field-value pairs",
						Required:     true,
						Type:         domain.NodePropertyType_CodeEditor,
						CodeLanguage: domain.CodeLanguageType_JSON,
					},
				},
			},
			{
				ID:          "hdel",
				Name:        "Delete Hash Fields",
				ActionType:  RedisIntegrationActionType_HDel,
				Description: "Delete one or more hash fields",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "fields",
						Name:        "Fields",
						Description: "Array of field names to delete",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "field",
									Name:        "Field",
									Description: "The field to delete",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "hexists",
				Name:        "Check Hash Field Existence",
				ActionType:  RedisIntegrationActionType_HExists,
				Description: "Check if a hash field exists",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "field",
						Name:        "Field",
						Description: "The field name to check",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "hgetall",
				Name:        "Get All Hash Fields",
				ActionType:  RedisIntegrationActionType_HGetAll,
				Description: "Get all fields and values in a hash",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "hkeys",
				Name:        "Get Hash Field Names",
				ActionType:  RedisIntegrationActionType_HKeys,
				Description: "Get all field names in a hash",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "hvals",
				Name:        "Get Hash Values",
				ActionType:  RedisIntegrationActionType_HVals,
				Description: "Get all values in a hash",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "hlen",
				Name:        "Get Hash Length",
				ActionType:  RedisIntegrationActionType_HLen,
				Description: "Get the number of fields in a hash",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The hash key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},

			// List Operations
			{
				ID:          "lpush",
				Name:        "Push to List (Left)",
				ActionType:  RedisIntegrationActionType_LPush,
				Description: "Push one or more values to the head of a list",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "values",
						Name:        "Values",
						Description: "Array of values to push",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value to push",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "rpush",
				Name:        "Push to List (Right)",
				ActionType:  RedisIntegrationActionType_RPush,
				Description: "Push one or more values to the tail of a list",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "values",
						Name:        "Values",
						Description: "Array of values to push",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "value",
									Name:        "Value",
									Description: "The value to push",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "lpop",
				Name:        "Pop from List (Left)",
				ActionType:  RedisIntegrationActionType_LPop,
				Description: "Remove and return elements from the head of a list",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "count",
						Name:        "Count",
						Description: "Number of elements to pop (default: 1)",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "rpop",
				Name:        "Pop from List (Right)",
				ActionType:  RedisIntegrationActionType_RPop,
				Description: "Remove and return elements from the tail of a list",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "count",
						Name:        "Count",
						Description: "Number of elements to pop (default: 1)",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "llen",
				Name:        "Get List Length",
				ActionType:  RedisIntegrationActionType_LLen,
				Description: "Get the length of a list",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "lrange",
				Name:        "Get List Range",
				ActionType:  RedisIntegrationActionType_LRange,
				Description: "Get a range of elements from a list",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "start",
						Name:        "Start Index",
						Description: "Start index (0-based, negative for end-relative)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "stop",
						Name:        "Stop Index",
						Description: "Stop index (inclusive, -1 for end)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "lindex",
				Name:        "Get List Element by Index",
				ActionType:  RedisIntegrationActionType_LIndex,
				Description: "Get an element from a list by its index",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "index",
						Name:        "Index",
						Description: "The index of the element (0-based, negative for end-relative)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "lset",
				Name:        "Set List Element by Index",
				ActionType:  RedisIntegrationActionType_LSet,
				Description: "Set the value of an element in a list by its index",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The list key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "index",
						Name:        "Index",
						Description: "The index of the element to set",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "value",
						Name:        "Value",
						Description: "The new value",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},

			// Set Operations
			{
				ID:          "sadd",
				Name:        "Add to Set",
				ActionType:  RedisIntegrationActionType_SAdd,
				Description: "Add one or more members to a set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "members",
						Name:        "Members",
						Description: "Array of members to add",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "member",
									Name:        "Member",
									Description: "The member to add",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "srem",
				Name:        "Remove from Set",
				ActionType:  RedisIntegrationActionType_SRem,
				Description: "Remove one or more members from a set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "members",
						Name:        "Members",
						Description: "Array of members to remove",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "member",
									Name:        "Member",
									Description: "The member to remove",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "smembers",
				Name:        "Get Set Members",
				ActionType:  RedisIntegrationActionType_SMembers,
				Description: "Get all members in a set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "sismember",
				Name:        "Check Set Membership",
				ActionType:  RedisIntegrationActionType_SIsMember,
				Description: "Check if a member exists in a set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "member",
						Name:        "Member",
						Description: "The member to check",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "scard",
				Name:        "Get Set Cardinality",
				ActionType:  RedisIntegrationActionType_SCard,
				Description: "Get the number of members in a set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "spop",
				Name:        "Pop from Set",
				ActionType:  RedisIntegrationActionType_SPop,
				Description: "Remove and return random members from a set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "count",
						Name:        "Count",
						Description: "Number of members to pop (default: 1)",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},

			// Sorted Set Operations
			{
				ID:          "zadd",
				Name:        "Add to Sorted Set",
				ActionType:  RedisIntegrationActionType_ZAdd,
				Description: "Add one or more members to a sorted set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "members",
						Name:        "Members",
						Description: "Array of objects with score and member fields",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_Map,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "member",
									Name:        "Member",
									Description: "The member to add",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
								{
									Key:         "score",
									Name:        "Score",
									Description: "The score of the member",
									Required:    true,
									Type:        domain.NodePropertyType_Number,
								},
							},
						},
					},
				},
			},
			{
				ID:          "zrem",
				Name:        "Remove from Sorted Set",
				ActionType:  RedisIntegrationActionType_ZRem,
				Description: "Remove one or more members from a sorted set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "members",
						Name:        "Members",
						Description: "Array of members to remove",
						Required:    true,
						Type:        domain.NodePropertyType_Array,
						ArrayOpts: &domain.ArrayPropertyOptions{
							MinItems: 1,
							MaxItems: 100,
							ItemType: domain.NodePropertyType_String,
							ItemProperties: []domain.NodeProperty{
								{
									Key:         "member",
									Name:        "Member",
									Description: "The member to remove",
									Required:    true,
									Type:        domain.NodePropertyType_String,
								},
							},
						},
					},
				},
			},
			{
				ID:          "zrange",
				Name:        "Get Sorted Set Range",
				ActionType:  RedisIntegrationActionType_ZRange,
				Description: "Get a range of members from a sorted set by index",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "start",
						Name:        "Start Index",
						Description: "Start index (0-based)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "stop",
						Name:        "Stop Index",
						Description: "Stop index (inclusive, -1 for end)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "zrevrange",
				Name:        "Get Sorted Set Range (Reverse)",
				ActionType:  RedisIntegrationActionType_ZRevRange,
				Description: "Get a range of members from a sorted set by index in reverse order",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "start",
						Name:        "Start Index",
						Description: "Start index (0-based)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
					{
						Key:         "stop",
						Name:        "Stop Index",
						Description: "Stop index (inclusive, -1 for end)",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "zrangebyscore",
				Name:        "Get Sorted Set Range by Score",
				ActionType:  RedisIntegrationActionType_ZRangeByScore,
				Description: "Get members from a sorted set by score range",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "min",
						Name:        "Min Score",
						Description: "Minimum score (use -inf for negative infinity)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "max",
						Name:        "Max Score",
						Description: "Maximum score (use +inf for positive infinity)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "zrevrangebyscore",
				Name:        "Get Sorted Set Range by Score (Reverse)",
				ActionType:  RedisIntegrationActionType_ZRevRangeByScore,
				Description: "Get members from a sorted set by score range in reverse order",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "min",
						Name:        "Min Score",
						Description: "Minimum score (use -inf for negative infinity)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "max",
						Name:        "Max Score",
						Description: "Maximum score (use +inf for positive infinity)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "zcard",
				Name:        "Get Sorted Set Cardinality",
				ActionType:  RedisIntegrationActionType_ZCard,
				Description: "Get the number of members in a sorted set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "zscore",
				Name:        "Get Sorted Set Member Score",
				ActionType:  RedisIntegrationActionType_ZScore,
				Description: "Get the score of a member in a sorted set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "member",
						Name:        "Member",
						Description: "The member to get the score for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "zrank",
				Name:        "Get Sorted Set Member Rank",
				ActionType:  RedisIntegrationActionType_ZRank,
				Description: "Get the rank of a member in a sorted set",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The sorted set key",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "member",
						Name:        "Member",
						Description: "The member to get the rank for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},

			// Key Management Operations
			{
				ID:          "keys",
				Name:        "Find Keys",
				ActionType:  RedisIntegrationActionType_Keys,
				Description: "Find all keys matching a pattern",
				Properties: []domain.NodeProperty{
					{
						Key:         "pattern",
						Name:        "Pattern",
						Description: "Pattern to match keys (use * for wildcard)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "expire",
				Name:        "Set Key Expiration",
				ActionType:  RedisIntegrationActionType_Expire,
				Description: "Set a timeout on a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to set expiration for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "expiration",
						Name:        "Expiration (seconds)",
						Description: "Expiration time in seconds",
						Required:    true,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
			{
				ID:          "ttl",
				Name:        "Get Key TTL",
				ActionType:  RedisIntegrationActionType_TTL,
				Description: "Get the time to live for a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to get TTL for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "type",
				Name:        "Get Key Type",
				ActionType:  RedisIntegrationActionType_Type,
				Description: "Get the type of a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to get type for",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "rename",
				Name:        "Rename Key",
				ActionType:  RedisIntegrationActionType_Rename,
				Description: "Rename a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "old_key",
						Name:        "Old Key",
						Description: "The current key name",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "new_key",
						Name:        "New Key",
						Description: "The new key name",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "persist",
				Name:        "Remove Key Expiration",
				ActionType:  RedisIntegrationActionType_Persist,
				Description: "Remove the expiration from a key",
				Properties: []domain.NodeProperty{
					{
						Key:         "key",
						Name:        "Key",
						Description: "The key to remove expiration from",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "redis_agent_memory",
				Name:        "Agent Conversation Memory",
				ActionType:  RedisIntegrationActionType_UseMemory,
				Description: "Use Conversation Memory to store and retrieve conversation history for AI agent",
				SupportedContexts: []domain.ActionUsageContext{
					domain.UsageContextMemoryProvider,
				},
				Properties: []domain.NodeProperty{
					{
						Key:         "key_prefix",
						Name:        "Key Prefix",
						Description: "The prefix for the keys",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "session_id",
						Name:        "Session ID",
						Description: "Unique identifier for the session",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "ttl",
						Name:        "TTL",
						Description: "The time to live for the conversation",
						Required:    false,
						Type:        domain.NodePropertyType_Integer,
					},
				},
			},
		},
	}
)
