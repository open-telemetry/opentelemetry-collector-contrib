// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			o := NewObfuscator(tt.config)
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

	o := NewObfuscator(config)

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

func TestObfuscateAttribute(t *testing.T) {
	tests := []struct {
		name     string
		config   DBSanitizerConfig
		value    string
		key      string
		expected string
		wantErr  bool
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
			expected: "SELECT * FROM users WHERE id = ?",
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
			expected: "SELECT * FROM users",
			wantErr:  false,
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
			expected: "SELECT * FROM users",
			wantErr:  false,
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
			expected: "SELECT * FROM users",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewObfuscator(tt.config)
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
