// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/url"
)

const dbSystemKey = "db.system"

var (
	mysqlSystem         = "mysql"
	redisSystem         = "redis"
	memcachedSystem     = "memcached"
	mongoSystem         = "mongodb"
	openSearchSystem    = "opensearch"
	elasticsearchSystem = "elasticsearch"
)

func TestURLSanitizationSpanName(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		spanName string
		kind     ptrace.SpanKind
		expected string
	}{
		{
			name: "url_enabled_client_span",
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled: true,
				},
			},
			spanName: "/users/123/profile",
			kind:     ptrace.SpanKindClient,
			expected: "/users/*/profile",
		},
		{
			name: "url_enabled_non_client_server",
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled: true,
				},
			},
			spanName: "/users/123/profile",
			kind:     ptrace.SpanKindInternal,
			expected: "/users/123/profile",
		},
		{
			name: "url_and_db_sanitizers_enabled_without_db_system",
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled: true,
				},
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig: db.SQLConfig{
						Enabled: true,
					},
				},
			},
			spanName: "/payments/123/details",
			kind:     ptrace.SpanKindClient,
			expected: "/payments/*/details",
		},
		{
			name: "url_and_db_sanitizers_enabled_with_db_system",
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled: true,
				},
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig: db.SQLConfig{
						Enabled: true,
					},
				},
			},
			spanName: "SELECT balance FROM accounts WHERE id = 7",
			kind:     ptrace.SpanKindClient,
			expected: "SELECT balance FROM accounts WHERE id = ?",
		},
		{
			name: "url_enabled_with_query_string",
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled: true,
				},
			},
			spanName: "GET /orders/123?sql=SELECT+1",
			kind:     ptrace.SpanKindClient,
			expected: "GET /orders/*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attrs := map[string]string{}
			if strings.HasPrefix(tc.spanName, "SELECT") {
				attrs[dbSystemKey] = mysqlSystem
			}

			result := runSpanNameTest(t, tc.config, tc.spanName, tc.kind, attrs)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestURLSanitizationSpanNameWithBlockedValues(t *testing.T) {
	t.Run("span name with blocked value should not be sanitized", func(t *testing.T) {
		config := &Config{
			AllowAllKeys:  true,
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			URLSanitization: url.URLSanitizationConfig{
				Enabled: true,
			},
		}

		result := runSpanNameTest(t, config, "/users/4111111111111111/profile", ptrace.SpanKindInternal, nil)
		assert.Equal(t, "/users/4111111111111111/profile", result)
	})

	t.Run("span name with service identifiers should be sanitized when allowed", func(t *testing.T) {
		config := &Config{
			AllowAllKeys: true,
			URLSanitization: url.URLSanitizationConfig{
				Enabled: true,
			},
		}

		result := runSpanNameTest(t, config, "payments-dev-sql-adapter-queue /api/process/123", ptrace.SpanKindServer, nil)
		assert.Equal(t, "payments-dev-sql-adapter-queue /api/process/*", result)
	})

	t.Run("span name with secret pattern should not be sanitized", func(t *testing.T) {
		config := &Config{
			AllowAllKeys:  true,
			BlockedValues: []string{"secret-[0-9]+"},
			URLSanitization: url.URLSanitizationConfig{
				Enabled: true,
			},
		}

		result := runSpanNameTest(t, config, "payments-service /api/secret-123/process", ptrace.SpanKindInternal, nil)
		assert.Equal(t, "payments-service /api/secret-123/process", result)
	})

	t.Run("span name without slash should not be sanitized even when URL sanitization enabled", func(t *testing.T) {
		config := &Config{
			AllowAllKeys:  true,
			AllowedValues: []string{".*"},
			URLSanitization: url.URLSanitizationConfig{
				Enabled: true,
			},
		}

		clientResult := runSpanNameTest(
			t, config, "us-west2-inventory-dev-catalog-sql-adapter-queue process", ptrace.SpanKindClient, nil,
		)
		serverResult := runSpanNameTest(
			t, config, "eu-central1-shipping-prod-delivery-processor handle", ptrace.SpanKindServer, nil,
		)
		internalResult := runSpanNameTest(
			t, config, "cache-service-lookup get-item", ptrace.SpanKindInternal, nil,
		)

		assert.Equal(t, "us-west2-inventory-dev-catalog-sql-adapter-queue process", clientResult)
		assert.Equal(t, "eu-central1-shipping-prod-delivery-processor handle", serverResult)
		assert.Equal(t, "cache-service-lookup get-item", internalResult)
	})

	t.Run("span name with slash should be sanitized when conditions met", func(t *testing.T) {
		config := &Config{
			AllowAllKeys: true,
			URLSanitization: url.URLSanitizationConfig{
				Enabled: true,
			},
		}

		result := runSpanNameTest(t, config, "GET /api/v1/payments/123", ptrace.SpanKindClient, nil)
		assert.Equal(t, "GET /api/v1/payments/*", result)
	})
}

func TestDBSpanNameUntouchedWhenDBSystemMissing(t *testing.T) {
	config := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{
				Enabled: true,
			},
		},
	}

	result := runSpanNameTest(
		t, config, "SELECT count(*) FROM orders WHERE customer_id = 42", ptrace.SpanKindClient, nil,
	)
	assert.Equal(t, "SELECT count(*) FROM orders WHERE customer_id = 42", result)
}

func TestSpanNameURLAndDBSanitizationCombined(t *testing.T) {
	config := &Config{
		AllowAllKeys: true,
		URLSanitization: url.URLSanitizationConfig{
			Enabled: true,
		},
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{
				Enabled: true,
			},
		},
	}

	result := runSpanNameTest(
		t,
		config,
		"GET /orders/123/lines/456?sql=SELECT+1",
		ptrace.SpanKindClient,
		map[string]string{dbSystemKey: mysqlSystem},
	)
	assert.Equal(t, "GET /orders/*/lines/*", result)
}

type spanTestCase struct {
	name                   string
	spanName               string
	kind                   ptrace.SpanKind
	attrs                  map[string]string
	urlSanitized           string
	sqlSanitized           string
	redisSanitized         string
	valkeySanitized        string
	memcachedSanitized     string
	mongoSanitized         string
	openSearchSanitized    string
	elasticsearchSanitized string
}

type sanitizerState struct {
	url        bool
	sql        bool
	redis      bool
	valkey     bool
	memcached  bool
	mongo      bool
	openSearch bool
	elastic    bool
}

func TestSpanNameAllSanitizerCombinations(t *testing.T) {
	spanCases := []spanTestCase{
		{
			name:         "url_span",
			spanName:     "/orders/123/detail",
			kind:         ptrace.SpanKindClient,
			urlSanitized: "/orders/*/detail",
		},
		{
			name:         "sql_span",
			spanName:     "SELECT * FROM accounts WHERE id = 42",
			kind:         ptrace.SpanKindClient,
			attrs:        map[string]string{dbSystemKey: mysqlSystem},
			sqlSanitized: "SELECT * FROM accounts WHERE id = ?",
		},
		{
			name:           "redis_span",
			spanName:       "SET user:123 secret",
			kind:           ptrace.SpanKindClient,
			attrs:          map[string]string{dbSystemKey: redisSystem},
			redisSanitized: "SET user:123 ?",
		},
		{
			name:               "memcached_span",
			spanName:           "set key 0 60 5\r\nmysecret\r\n",
			kind:               ptrace.SpanKindClient,
			attrs:              map[string]string{dbSystemKey: memcachedSystem},
			memcachedSanitized: "set key 0 60 5",
		},
		{
			name:           "mongo_span",
			spanName:       `{"find":"users","filter":{"name":"john"}}`,
			kind:           ptrace.SpanKindClient,
			attrs:          map[string]string{dbSystemKey: mongoSystem},
			mongoSanitized: `{"find":"?","filter":{"name":"?"}}`,
		},
		{
			name:                "opensearch_span",
			spanName:            `{"query":{"match":{"title":"secret"}}}`,
			kind:                ptrace.SpanKindClient,
			attrs:               map[string]string{dbSystemKey: openSearchSystem},
			openSearchSanitized: `{"query":{"match":{"title":"?"}}}`,
		},
		{
			name:                   "elasticsearch_span",
			spanName:               `{"query":{"match":{"title":"secret"}}}`,
			kind:                   ptrace.SpanKindClient,
			attrs:                  map[string]string{dbSystemKey: elasticsearchSystem},
			elasticsearchSanitized: `{"query":{"match":{"title":"?"}}}`,
		},
		{
			name:         "query_string_span",
			spanName:     "GET /reports/7?sql=SELECT+1",
			kind:         ptrace.SpanKindClient,
			urlSanitized: "GET /reports/*",
		},
		{
			name:            "valkey_span",
			spanName:        "GET key:123",
			kind:            ptrace.SpanKindClient,
			attrs:           map[string]string{dbSystemKey: "valkey"},
			valkeySanitized: "GET key:123",
		},
	}

	for urlBit := range 2 {
		for mask := range 1 << 7 {
			state := sanitizerState{
				url:        urlBit == 1,
				sql:        mask&1 != 0,
				redis:      mask&(1<<1) != 0,
				valkey:     mask&(1<<2) != 0,
				memcached:  mask&(1<<3) != 0,
				mongo:      mask&(1<<4) != 0,
				openSearch: mask&(1<<5) != 0,
				elastic:    mask&(1<<6) != 0,
			}

			config := &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled: state.url,
				},
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig:        db.SQLConfig{Enabled: state.sql},
					RedisConfig:      db.RedisConfig{Enabled: state.redis},
					ValkeyConfig:     db.ValkeyConfig{Enabled: state.valkey},
					MemcachedConfig:  db.MemcachedConfig{Enabled: state.memcached},
					MongoConfig:      db.MongoConfig{Enabled: state.mongo},
					OpenSearchConfig: db.OpenSearchConfig{Enabled: state.openSearch},
					ESConfig:         db.ESConfig{Enabled: state.elastic},
				},
			}

			for _, sc := range spanCases {
				t.Run(sc.name, func(t *testing.T) {
					result := runSpanNameTest(t, config, sc.spanName, sc.kind, maps.Clone(sc.attrs))
					assert.NotEmpty(t, result, "span name must not be empty")
					assert.NotEqual(t, "...", result, "span name must not be ellipsis")
					assert.Equalf(
						t,
						expectedSpanResult(sc, state),
						result,
						"span=%s url=%t mask=%06b",
						sc.name,
						state.url,
						mask,
					)
				})
			}
		}
	}
}

func expectedSpanResult(sc spanTestCase, state sanitizerState) string {
	result := sc.spanName
	if state.url && sc.urlSanitized != "" {
		result = sc.urlSanitized
	}

	if sc.attrs == nil {
		return result
	}

	switch sc.attrs[dbSystemKey] {
	case "mysql":
		if state.sql && sc.sqlSanitized != "" {
			return sc.sqlSanitized
		}
	case "redis":
		if state.redis && sc.redisSanitized != "" {
			return sc.redisSanitized
		}
	case "valkey":
		if state.valkey && sc.valkeySanitized != "" {
			return sc.valkeySanitized
		}
	case "memcached":
		if state.memcached && sc.memcachedSanitized != "" {
			return sc.memcachedSanitized
		}
	case "mongodb":
		if state.mongo && sc.mongoSanitized != "" {
			return sc.mongoSanitized
		}
	case "opensearch":
		if state.openSearch && sc.openSearchSanitized != "" {
			return sc.openSearchSanitized
		}
	case "elasticsearch":
		if state.elastic && sc.elasticsearchSanitized != "" {
			return sc.elasticsearchSanitized
		}
	}
	return result
}

func runSpanNameTest(t *testing.T, config *Config, spanName string, kind ptrace.SpanKind, attrs map[string]string) string {
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()

	span.SetName(spanName)
	span.SetKind(kind)
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}

	processor, err := newRedaction(t.Context(), config, zaptest.NewLogger(t))
	require.NoError(t, err)

	outTraces, err := processor.processTraces(t.Context(), inBatch)
	require.NoError(t, err)

	return outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name()
}

func TestAllDBSystemsSpanName(t *testing.T) {
	testCases := []struct {
		name        string
		dbSystem    string
		dbSystemKey string // defaults to "db.system" if empty
		spanName    string
		configFunc  func() *Config
		expected    string
	}{
		{
			name:     "mysql_with_db.system",
			dbSystem: mysqlSystem,
			spanName: "SELECT * FROM users WHERE id = 123",
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						SQLConfig: db.SQLConfig{Enabled: true},
					},
				}
			},
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			name:        "mysql_with_db.system.name",
			dbSystem:    mysqlSystem,
			dbSystemKey: "db.system.name",
			spanName:    "SELECT * FROM payments WHERE account_id = 123",
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						SQLConfig: db.SQLConfig{Enabled: true},
					},
				}
			},
			expected: "SELECT * FROM payments WHERE account_id = ?",
		},
		{
			name:     "redis",
			dbSystem: redisSystem,
			spanName: "SET user:12345 my-secret",
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						RedisConfig: db.RedisConfig{Enabled: true},
					},
				}
			},
			expected: "SET user:12345 ?",
		},
		{
			name:     "postgresql",
			dbSystem: "postgresql",
			spanName: "SELECT * FROM users WHERE id = 42",
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						SQLConfig: db.SQLConfig{Enabled: true},
					},
				}
			},
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			name:     "mariadb",
			dbSystem: "mariadb",
			spanName: "UPDATE accounts SET balance = 100 WHERE id = 7",
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						SQLConfig: db.SQLConfig{Enabled: true},
					},
				}
			},
			expected: "UPDATE accounts SET balance = ? WHERE id = ?",
		},
		{
			name:     "sqlite",
			dbSystem: "sqlite",
			spanName: "DELETE FROM sessions WHERE expired < 1234567890",
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						SQLConfig: db.SQLConfig{Enabled: true},
					},
				}
			},
			expected: "DELETE FROM sessions WHERE expired < ?",
		},
		{
			name:     "mongodb",
			dbSystem: mongoSystem,
			spanName: `{"find":"users","filter":{"name":"john","age":30}}`,
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						MongoConfig: db.MongoConfig{Enabled: true},
					},
				}
			},
			expected: `{"find":"?","filter":{"name":"?","age":"?"}}`,
		},
		{
			name:     "elasticsearch",
			dbSystem: elasticsearchSystem,
			spanName: `{"query":{"match":{"title":"secret-document"}}}`,
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						ESConfig: db.ESConfig{Enabled: true},
					},
				}
			},
			expected: `{"query":{"match":{"title":"?"}}}`,
		},
		{
			name:     "opensearch",
			dbSystem: openSearchSystem,
			spanName: `{"query":{"term":{"user_id":"12345"}}}`,
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						OpenSearchConfig: db.OpenSearchConfig{Enabled: true},
					},
				}
			},
			expected: `{"query":{"term":{"user_id":"?"}}}`,
		},
		{
			name:     "memcached",
			dbSystem: memcachedSystem,
			spanName: "set mykey 0 3600 11\r\nsecret-data\r\n",
			configFunc: func() *Config {
				return &Config{
					AllowAllKeys: true,
					DBSanitizer: db.DBSanitizerConfig{
						MemcachedConfig: db.MemcachedConfig{Enabled: true},
					},
				}
			},
			expected: "set mykey 0 3600 11",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := dbSystemKey
			if tc.dbSystemKey != "" {
				key = tc.dbSystemKey
			}
			result := runSpanNameTest(
				t, tc.configFunc(), tc.spanName, ptrace.SpanKindClient,
				map[string]string{key: tc.dbSystem},
			)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValkeySpanNameObfuscation(t *testing.T) {
	config := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			ValkeyConfig: db.ValkeyConfig{
				Enabled: true,
			},
		},
	}

	result := runSpanNameTest(
		t, config, "SET user:123 secret-value", ptrace.SpanKindClient,
		map[string]string{dbSystemKey: "valkey"},
	)
	assert.Equal(t, "SET user:123 ?", result)
}

func TestSpanKindFiltering(t *testing.T) {
	config := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{Enabled: true},
		},
	}

	testCases := []struct {
		name           string
		kind           ptrace.SpanKind
		shouldSanitize bool
	}{
		{"client", ptrace.SpanKindClient, true},
		{"server", ptrace.SpanKindServer, true},
		{"internal", ptrace.SpanKindInternal, true},
		{"producer", ptrace.SpanKindProducer, false},
		{"consumer", ptrace.SpanKindConsumer, false},
		{"unspecified", ptrace.SpanKindUnspecified, false},
	}

	spanName := "SELECT * FROM users WHERE id = 42"
	expected := "SELECT * FROM users WHERE id = ?"

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := runSpanNameTest(
				t, config, spanName, tc.kind,
				map[string]string{dbSystemKey: mysqlSystem},
			)
			if tc.shouldSanitize {
				assert.Equal(t, expected, result)
			} else {
				assert.Equal(t, spanName, result)
			}
		})
	}
}

func TestSpanNameAllowedValueSkipsSanitization(t *testing.T) {
	config := &Config{
		AllowAllKeys:  true,
		AllowedValues: []string{"SELECT.*"},
		URLSanitization: url.URLSanitizationConfig{
			Enabled: true,
		},
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{Enabled: true},
		},
	}

	result := runSpanNameTest(
		t, config, "SELECT * FROM users WHERE id = 123", ptrace.SpanKindClient,
		map[string]string{"db.system": "mysql"},
	)
	assert.Equal(t, "SELECT * FROM users WHERE id = 123", result)
}

func TestSanitizeSpanNameDisabledForURL(t *testing.T) {
	falseBool := false
	config := &Config{
		AllowAllKeys: true,
		URLSanitization: url.URLSanitizationConfig{
			Enabled:          true,
			SanitizeSpanName: &falseBool,
		},
	}

	result := runSpanNameTest(
		t, config, "/users/123/profile", ptrace.SpanKindClient, nil,
	)
	assert.Equal(t, "/users/123/profile", result)
}

func TestSanitizeSpanNameDisabledForDB(t *testing.T) {
	falseBool := false
	config := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig:        db.SQLConfig{Enabled: true},
			SanitizeSpanName: &falseBool,
		},
	}

	result := runSpanNameTest(
		t, config, "SELECT * FROM users WHERE id = 123", ptrace.SpanKindClient,
		map[string]string{"db.system": "mysql"},
	)
	assert.Equal(t, "SELECT * FROM users WHERE id = 123", result)
}

func TestSanitizeSpanNameEnabledExplicitly(t *testing.T) {
	trueBool := true
	config := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig:        db.SQLConfig{Enabled: true},
			SanitizeSpanName: &trueBool,
		},
	}

	result := runSpanNameTest(
		t, config, "SELECT * FROM users WHERE id = 123", ptrace.SpanKindClient,
		map[string]string{"db.system": "mysql"},
	)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", result)
}

func TestSpanNameEdgeCases(t *testing.T) {
	config := &Config{
		AllowAllKeys: true,
		URLSanitization: url.URLSanitizationConfig{
			Enabled: true,
		},
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{Enabled: true},
		},
	}

	testCases := []struct {
		name     string
		spanName string
		kind     ptrace.SpanKind
		attrs    map[string]string
		expected string
	}{
		{
			name:     "empty_span_name",
			spanName: "",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "",
		},
		{
			name:     "only_numbers",
			spanName: "12345",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "12345",
		},
		{
			name:     "only_slash",
			spanName: "/",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "/",
		},
		{
			name:     "unicode_in_url",
			spanName: "/users/测试/profile",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "/users/*/profile",
		},
		{
			name:     "special_chars_in_url",
			spanName: "/api/v1/items/@special/data",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "/api/v1/items/*/data",
		},
		{
			name:     "very_long_span_name",
			spanName: "/api/" + strings.Repeat("segment/", 100) + "end",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			// URL sanitizer has a limit, so it only processes the first few segments
			expected: "/api/segment/segment/segment/segment/segment/segment/segment/segment",
		},
		{
			name:     "whitespace_only",
			spanName: "   ",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "   ",
		},
		{
			name:     "sql_with_newlines",
			spanName: "SELECT *\nFROM users\nWHERE id = 42",
			kind:     ptrace.SpanKindClient,
			attrs:    map[string]string{dbSystemKey: mysqlSystem},
			expected: "SELECT *\nFROM users\nWHERE id = ?",
		},
		{
			name:     "url_with_fragment",
			spanName: "GET /page/123#section",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "GET /page/*",
		},
		{
			name:     "mixed_slashes",
			spanName: "/v1/api\\path/123",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			// Backslash is treated as a separator, so both /api and path are sanitized
			expected: "/v1/*/*",
		},
		{
			name:     "no_sanitization_without_db_system",
			spanName: "SELECT * FROM users WHERE id = 42",
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: "SELECT * FROM users WHERE id = 42",
		},
		{
			name:     "json_like_but_not_db",
			spanName: `{"message":"hello"}`,
			kind:     ptrace.SpanKindClient,
			attrs:    nil,
			expected: `{"message":"hello"}`,
		},
		{
			name:     "span_name_without_slash_sanitized_to_asterisk",
			spanName: "okey-dokey-0",
			kind:     ptrace.SpanKindServer,
			attrs:    map[string]string{"http.method": "GET"},
			expected: "okey-dokey-0",
		},
		{
			name:     "span_name_without_slash_sanitized_to_asterisk_client",
			spanName: "simple-operation-123",
			kind:     ptrace.SpanKindClient,
			attrs:    map[string]string{"http.request.method": "POST"},
			expected: "simple-operation-123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := runSpanNameTest(t, config, tc.spanName, tc.kind, tc.attrs)
			assert.Equal(t, tc.expected, result)
		})
	}
}
