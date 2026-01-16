// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestSQLObfuscator(t *testing.T) {
	o := &sqlObfuscator{
		dbAttributes: dbAttributes{
			attributes: map[string]bool{
				"db.statement": true,
			},
		},
		obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				ReplaceDigits: true,
			},
			Redis: obfuscate.RedisConfig{
				Enabled: true,
			},
			Memcached: obfuscate.MemcachedConfig{
				Enabled:     true,
				KeepCommand: true,
			},
			ES: obfuscate.JSONConfig{
				Enabled: true,
			},
			OpenSearch: obfuscate.JSONConfig{
				Enabled: true,
			},
		}),
	}

	tests := []struct {
		name          string
		input         string
		expected      string
		shouldError   bool
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "simple select",
			input:         "SELECT * FROM users WHERE id = 123",
			expected:      "SELECT * FROM users WHERE id = ?",
			shouldError:   false,
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "insert statement",
			input:         "INSERT INTO users (name, age) VALUES ('John', 30)",
			expected:      "INSERT INTO users ( name, age ) VALUES ( ? )",
			shouldError:   false,
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "non-db attribute",
			input:         "SELECT * FROM users",
			expected:      "SELECT * FROM users",
			shouldError:   false,
			attributeKey:  "other.field",
			shouldProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.ObfuscateAttribute(tt.input, tt.attributeKey)
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.shouldProcess, o.ShouldProcessAttribute(tt.attributeKey))
		})
	}
}

func TestRedisObfuscator(t *testing.T) {
	o := &redisObfuscator{
		dbAttributes: dbAttributes{
			attributes: map[string]bool{
				"db.statement": true,
			},
		},
		obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				ReplaceDigits: true,
			},
			Redis: obfuscate.RedisConfig{
				Enabled: true,
			},
			Memcached: obfuscate.MemcachedConfig{
				Enabled:     true,
				KeepCommand: true,
			},
			ES: obfuscate.JSONConfig{
				Enabled: true,
			},
			OpenSearch: obfuscate.JSONConfig{
				Enabled: true,
			},
		}),
	}

	tests := []struct {
		name          string
		input         string
		expected      string
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "set command",
			input:         "SET user:123 john",
			expected:      "SET user:123 ?",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "get command",
			input:         "GET user:123",
			expected:      "GET user:123",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "non-db attribute",
			input:         "SET key value",
			expected:      "SET key value",
			attributeKey:  "other.field",
			shouldProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.ObfuscateAttribute(tt.input, tt.attributeKey)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.shouldProcess, o.ShouldProcessAttribute(tt.attributeKey))
		})
	}
}

func TestMemcachedObfuscator(t *testing.T) {
	o := &memcachedObfuscator{
		dbAttributes: dbAttributes{
			attributes: map[string]bool{
				"db.statement": true,
			},
		},
		obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				ReplaceDigits: true,
			},
			Redis: obfuscate.RedisConfig{
				Enabled: true,
			},
			Memcached: obfuscate.MemcachedConfig{
				Enabled:     true,
				KeepCommand: true,
			},
			ES: obfuscate.JSONConfig{
				Enabled: true,
			},
			OpenSearch: obfuscate.JSONConfig{
				Enabled: true,
			},
		}),
	}

	tests := []struct {
		name          string
		input         string
		expected      string
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "set command",
			input:         "set mykey 0 60 5",
			expected:      "set mykey 0 60 5",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "get command",
			input:         "get mykey",
			expected:      "get mykey",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "non-db attribute",
			input:         "set key 0 60 5",
			expected:      "set key 0 60 5",
			attributeKey:  "other.field",
			shouldProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.ObfuscateAttribute(tt.input, tt.attributeKey)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.shouldProcess, o.ShouldProcessAttribute(tt.attributeKey))
		})
	}
}

func TestMongoObfuscator(t *testing.T) {
	o := &mongoObfuscator{
		dbAttributes: dbAttributes{
			attributes: map[string]bool{
				"db.statement": true,
			},
		},
		obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				ReplaceDigits: true,
			},
			Redis: obfuscate.RedisConfig{
				Enabled: true,
			},
			Memcached: obfuscate.MemcachedConfig{
				Enabled:     true,
				KeepCommand: true,
			},
			ES: obfuscate.JSONConfig{
				Enabled: true,
			},
			OpenSearch: obfuscate.JSONConfig{
				Enabled: true,
			},
		}),
		logger: zaptest.NewLogger(t),
	}

	tests := []struct {
		name          string
		input         string
		expected      string
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "find command",
			input:         `{"find": "users", "filter": {"age": 30}}`,
			expected:      `{"find": "users", "filter": {"age": 30}}`,
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "non-db attribute",
			input:         `{"find": "users"}`,
			expected:      `{"find": "users"}`,
			attributeKey:  "other.field",
			shouldProcess: false,
		},
		{
			name:          "non-json value",
			input:         "/foo/{bar}",
			expected:      "/foo/{bar}",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.ObfuscateAttribute(tt.input, tt.attributeKey)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.shouldProcess, o.ShouldProcessAttribute(tt.attributeKey))
		})
	}
}

func TestOpenSearchObfuscator(t *testing.T) {
	o := &opensearchObfuscator{
		dbAttributes: dbAttributes{
			attributes: map[string]bool{
				"db.statement": true,
			},
		},
		obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				ReplaceDigits: true,
			},
			Redis: obfuscate.RedisConfig{
				Enabled: true,
			},
			Memcached: obfuscate.MemcachedConfig{
				Enabled:     true,
				KeepCommand: true,
			},
			ES: obfuscate.JSONConfig{
				Enabled: true,
			},
			OpenSearch: obfuscate.JSONConfig{
				Enabled: true,
			},
		}),
		logger: zaptest.NewLogger(t),
	}

	tests := []struct {
		name          string
		input         string
		expected      string
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "search query",
			input:         `{"query": {"match": {"title": "test"}}}`,
			expected:      `{"query":{"match":{"title":"?"}}}`,
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "non-db attribute",
			input:         `{"query": {"match_all": {}}}`,
			expected:      `{"query": {"match_all": {}}}`,
			attributeKey:  "other.field",
			shouldProcess: false,
		},
		{
			name:          "non-json value",
			input:         "/foo/bar",
			expected:      "/foo/bar",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.ObfuscateAttribute(tt.input, tt.attributeKey)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.shouldProcess, o.ShouldProcessAttribute(tt.attributeKey))
		})
	}
}

func TestRedisObfuscatorWithValkeyConfig(t *testing.T) {
	o := &redisObfuscator{
		dbAttributes: dbAttributes{
			attributes: map[string]bool{
				"db.statement": true,
			},
			dbSystems: map[string]bool{
				"valkey": true,
			},
		},
		obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				ReplaceDigits: true,
			},
			Redis: obfuscate.RedisConfig{
				Enabled: true,
			},
			Memcached: obfuscate.MemcachedConfig{
				Enabled:     true,
				KeepCommand: true,
			},
			ES: obfuscate.JSONConfig{
				Enabled: true,
			},
			OpenSearch: obfuscate.JSONConfig{
				Enabled: true,
			},
		}),
	}

	tests := []struct {
		name          string
		input         string
		expected      string
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "valkey command",
			input:         "SET user:123 john",
			expected:      "SET user:123 ?",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "non-db attribute",
			input:         "SET key value",
			expected:      "SET key value",
			attributeKey:  "other.field",
			shouldProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.ObfuscateAttribute(tt.input, tt.attributeKey)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.shouldProcess, o.ShouldProcessAttribute(tt.attributeKey))
		})
	}
}

func TestElasticsearchObfuscator(t *testing.T) {
	o := &esObfuscator{
		dbAttributes: dbAttributes{
			attributes: map[string]bool{
				"db.statement": true,
			},
		},
		obfuscator: obfuscate.NewObfuscator(obfuscate.Config{
			SQL: obfuscate.SQLConfig{
				ReplaceDigits: true,
			},
			Redis: obfuscate.RedisConfig{
				Enabled: true,
			},
			Memcached: obfuscate.MemcachedConfig{
				Enabled:     true,
				KeepCommand: true,
			},
			ES: obfuscate.JSONConfig{
				Enabled: true,
			},
			OpenSearch: obfuscate.JSONConfig{
				Enabled: true,
			},
		}),
		logger: zaptest.NewLogger(t),
	}

	tests := []struct {
		name          string
		input         string
		expected      string
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "search query",
			input:         `{"query": {"match": {"title": "test"}}}`,
			expected:      `{"query":{"match":{"title":"?"}}}`,
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
		{
			name:          "non-db attribute",
			input:         `{"query": {"match_all": {}}}`,
			expected:      `{"query": {"match_all": {}}}`,
			attributeKey:  "other.field",
			shouldProcess: false,
		},
		{
			name:          "non-json value",
			input:         "/svc/resource",
			expected:      "/svc/resource",
			attributeKey:  "db.statement",
			shouldProcess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := o.ObfuscateAttribute(tt.input, tt.attributeKey)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.shouldProcess, o.ShouldProcessAttribute(tt.attributeKey))
		})
	}
}

func TestSupportsSystemEmptySystem(t *testing.T) {
	attrs := dbAttributes{
		attributes: map[string]bool{"db.statement": true},
		dbSystems:  map[string]bool{"mysql": true},
	}
	assert.False(t, attrs.SupportsSystem(""))
}

func TestSupportsSystemNoSystems(t *testing.T) {
	attrs := dbAttributes{
		attributes: map[string]bool{"db.statement": true},
		dbSystems:  nil,
	}
	assert.False(t, attrs.SupportsSystem("mysql"))
}

func TestIsValidJSONEmptyString(t *testing.T) {
	assert.False(t, isValidJSON(""))
	assert.False(t, isValidJSON("   "))
	assert.False(t, isValidJSON("\t\n"))
}
