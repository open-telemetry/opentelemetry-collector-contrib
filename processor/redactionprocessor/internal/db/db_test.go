// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCreateAttributes(t *testing.T) {
	tests := []struct {
		name       string
		attributes []string
		expected   map[string]bool
	}{
		{
			name:       "empty attributes",
			attributes: []string{},
			expected:   map[string]bool{},
		},
		{
			name:       "single attribute",
			attributes: []string{"db.statement"},
			expected:   map[string]bool{"db.statement": true},
		},
		{
			name:       "multiple attributes",
			attributes: []string{"db.statement", "db.query", "db.command"},
			expected:   map[string]bool{"db.statement": true, "db.query": true, "db.command": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createAttributes(tt.attributes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewObfuscator(t *testing.T) {
	tests := []struct {
		name                    string
		config                  DBSanitizerConfig
		expectedObfuscatorCount int
		expectedProcessSpecific bool
	}{
		{
			name:                    "all disabled",
			config:                  DBSanitizerConfig{},
			expectedObfuscatorCount: 0,
			expectedProcessSpecific: false,
		},
		{
			name: "sql enabled",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			expectedObfuscatorCount: 1,
			expectedProcessSpecific: true,
		},
		{
			name: "multiple enabled",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				RedisConfig: RedisConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				MemcachedConfig: MemcachedConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				ValkeyConfig: ValkeyConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				MongoConfig: MongoConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				OpenSearchConfig: OpenSearchConfig{
					Enabled: true,
				},
				ESConfig: ESConfig{
					Enabled: true,
				},
			},
			expectedObfuscatorCount: 7,
			expectedProcessSpecific: true,
		},
		{
			name: "valkey enabled",
			config: DBSanitizerConfig{
				ValkeyConfig: ValkeyConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			expectedObfuscatorCount: 1,
			expectedProcessSpecific: true,
		},
		{
			name: "mongo enabled",
			config: DBSanitizerConfig{
				MongoConfig: MongoConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			expectedObfuscatorCount: 1,
			expectedProcessSpecific: true,
		},
		{
			name: "opensearch enabled",
			config: DBSanitizerConfig{
				OpenSearchConfig: OpenSearchConfig{
					Enabled: true,
				},
			},
			expectedObfuscatorCount: 1,
			expectedProcessSpecific: false,
		},
		{
			name: "elasticsearch enabled",
			config: DBSanitizerConfig{
				ESConfig: ESConfig{
					Enabled: true,
				},
			},
			expectedObfuscatorCount: 1,
			expectedProcessSpecific: false,
		},
		{
			name: "enabled without attributes",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled: true,
				},
				RedisConfig: RedisConfig{
					Enabled: true,
				},
			},
			expectedObfuscatorCount: 2,
			expectedProcessSpecific: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewObfuscator(tt.config, zaptest.NewLogger(t))
			assert.Len(t, o.obfuscators, tt.expectedObfuscatorCount)
			assert.Equal(t, tt.expectedProcessSpecific, o.HasSpecificAttributes())
			assert.Equal(t, tt.expectedObfuscatorCount > 0, o.HasObfuscators())
		})
	}
}

func TestObfuscate(t *testing.T) {
	config := DBSanitizerConfig{
		SQLConfig: SQLConfig{
			Enabled:    true,
			Attributes: []string{"db.statement"},
		},
		RedisConfig: RedisConfig{
			Enabled:    true,
			Attributes: []string{"db.statement"},
		},
	}

	o := NewObfuscator(config, zaptest.NewLogger(t))

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "sql query",
			input:    "SELECT * FROM users WHERE id = 123",
			expected: "SELECT * FROM users WHERE id = ?",
			wantErr:  false,
		},
		{
			name:     "redis command",
			input:    "SET user:123 john",
			expected: "SET user:? ?",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.Obfuscate(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObfuscateWithSystem(t *testing.T) {
	config := DBSanitizerConfig{
		SQLConfig: SQLConfig{
			Enabled:    true,
			Attributes: []string{"db.statement"},
		},
		RedisConfig: RedisConfig{
			Enabled:    true,
			Attributes: []string{"db.statement"},
		},
		MemcachedConfig: MemcachedConfig{
			Enabled:    true,
			Attributes: []string{"db.statement"},
		},
		MongoConfig: MongoConfig{
			Enabled:    true,
			Attributes: []string{"db.statement"},
		},
		OpenSearchConfig: OpenSearchConfig{
			Enabled: true,
		},
		ESConfig: ESConfig{
			Enabled: true,
		},
	}

	o := NewObfuscator(config, zaptest.NewLogger(t))

	t.Run("sql system", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem("SELECT * FROM users WHERE id = 1", "mysql")
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", result)
	})

	t.Run("redis system", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem("SET user:1 top-secret", "redis")
		require.NoError(t, err)
		assert.Equal(t, "SET user:1 ?", result)
	})

	t.Run("memcached system", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem("set mykey 0 60 5\r\nsecret\r\n", "memcached")
		require.NoError(t, err)
		assert.Equal(t, "set mykey 0 60 5", result)
	})

	t.Run("mongodb system", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem(`{"find":"users","filter":{"name":"john"}}`, "mongodb")
		require.NoError(t, err)
		assert.Contains(t, result, `"find":"?"`)
	})

	t.Run("opensearch system", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem(`{"query":{"match":{"title":"secret"}}}`, "opensearch")
		require.NoError(t, err)
		assert.Contains(t, result, `"title":"?"`)
	})

	t.Run("elasticsearch system", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem(`{"query":{"match":{"title":"secret"}}}`, "elasticsearch")
		require.NoError(t, err)
		assert.Contains(t, result, `"title":"?"`)
	})

	t.Run("missing system leaves value unchanged", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem("SELECT * FROM users WHERE id = 1", "")
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = 1", result)
	})

	t.Run("unknown system leaves unchanged", func(t *testing.T) {
		result, err := o.ObfuscateWithSystem("SET key value", "unknown")
		require.NoError(t, err)
		assert.Equal(t, "SET key value", result)
	})
}

func TestNewObfuscatorWithNilLogger(t *testing.T) {
	config := DBSanitizerConfig{
		SQLConfig: SQLConfig{
			Enabled:    true,
			Attributes: []string{"db.statement"},
		},
	}
	o := NewObfuscator(config, nil)
	assert.NotNil(t, o)
	assert.NotNil(t, o.logger)
}

func TestObfuscateWithNoObfuscators(t *testing.T) {
	o := NewObfuscator(DBSanitizerConfig{}, zaptest.NewLogger(t))
	result, err := o.ObfuscateWithSystem("SELECT * FROM users WHERE id = 1", "mysql")
	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM users WHERE id = 1", result)
}

func TestCreateSystems(t *testing.T) {
	tests := []struct {
		name     string
		systems  []string
		expected map[string]bool
	}{
		{
			name:     "empty systems",
			systems:  []string{},
			expected: nil,
		},
		{
			name:     "single system",
			systems:  []string{"MySQL"},
			expected: map[string]bool{"mysql": true},
		},
		{
			name:     "multiple systems",
			systems:  []string{"MySQL", "PostgreSQL", "Redis"},
			expected: map[string]bool{"mysql": true, "postgresql": true, "redis": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createSystems(tt.systems)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObfuscateAttribute(t *testing.T) {
	tests := []struct {
		name     string
		config   DBSanitizerConfig
		value    string
		key      string
		dbSystem string
		expected string
		wantErr  bool
		fallback bool
	}{
		{
			name: "sql statement",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "SELECT * FROM users WHERE id = 123",
			key:      "db.statement",
			dbSystem: "mysql",
			expected: "SELECT * FROM users WHERE id = ?",
			wantErr:  false,
			fallback: false,
		},
		{
			name: "sql statement with mariadb system",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "UPDATE accounts SET balance = 10 WHERE id = 7",
			key:      "db.statement",
			dbSystem: "mariadb",
			expected: "UPDATE accounts SET balance = ? WHERE id = ?",
			wantErr:  false,
		},
		{
			name: "non-db attribute",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "SELECT * FROM users",
			key:      "other.field",
			dbSystem: "mysql",
			expected: "SELECT * FROM users",
			wantErr:  false,
		},
		{
			name: "missing db.system leaves attribute unchanged",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				RedisConfig: RedisConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "SET user:123 john",
			key:      "db.statement",
			dbSystem: "",
			expected: "SET user:123 john",
			wantErr:  false,
		},
		{
			name: "missing db.system with fallback obfuscates sequentially",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				RedisConfig: RedisConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "SET user:123 john",
			key:      "db.statement",
			dbSystem: "",
			expected: "SET user:? ?",
			wantErr:  false,
			fallback: true,
		},
		{
			name: "no specific attributes",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled: true,
				},
			},
			value:    "SELECT * FROM users",
			key:      "db.statement",
			dbSystem: "mysql",
			expected: "SELECT * FROM users",
			wantErr:  false,
		},
		{
			name: "redis system selects redis obfuscator",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				RedisConfig: RedisConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "SET user:123 john",
			key:      "db.statement",
			dbSystem: "redis",
			expected: "SET user:123 ?",
			wantErr:  false,
		},
		{
			name: "valkey system uses valkey obfuscator",
			config: DBSanitizerConfig{
				ValkeyConfig: ValkeyConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "SET key value",
			key:      "db.statement",
			dbSystem: "valkey",
			expected: "SET key ?",
			wantErr:  false,
		},
		{
			name: "mismatched system leaves attribute unchanged",
			config: DBSanitizerConfig{
				SQLConfig: SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				RedisConfig: RedisConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
			value:    "SET user:123 john",
			key:      "db.statement",
			dbSystem: "cassandra",
			expected: "SET user:123 john",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			if tt.fallback {
				cfg.AllowFallbackWithoutSystem = true
			}
			o := NewObfuscator(cfg, zaptest.NewLogger(t))
			o.DBSystem = tt.dbSystem
			result, err := o.ObfuscateAttribute(tt.value, tt.key)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
