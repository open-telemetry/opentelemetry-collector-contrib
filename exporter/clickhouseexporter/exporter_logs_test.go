// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLogsExporter_New(t *testing.T) {
	type validate func(*testing.T, *logsExporter, error)

	failWithMsg := func(msg string) validate {
		return func(t *testing.T, _ *logsExporter, err error) {
			require.ErrorContains(t, err, msg)
		}
	}

	tests := map[string]struct {
		config *Config
		want   validate
	}{
		"no dsn": {
			config: withDefaultConfig(),
			want:   failWithMsg("parse dsn address failed"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var err error
			exporter := newLogsExporter(zap.NewNop(), test.config)

			if exporter != nil {
				err = errors.Join(err, exporter.start(t.Context(), nil))
				defer func() {
					require.NoError(t, exporter.shutdown(t.Context()))
				}()
			}

			test.want(t, exporter, err)
		})
	}
}

func TestRenderCreateLogsTableSQL(t *testing.T) {
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = defaultEndpoint
		c.Database = "test_db"
		c.LogsTableName = "otel_logs"
		c.TTL = 72 * time.Hour
	})

	t.Run("bloom filter indexes for CH < 26.2", func(t *testing.T) {
		sql, err := renderCreateLogsTableSQL(cfg, false)
		require.NoError(t, err)

		require.Contains(t, sql, "TYPE bloom_filter")
		require.Contains(t, sql, "TYPE tokenbf_v1")
		require.NotContains(t, sql, "TYPE text(")

		require.Contains(t, sql, "toStartOfFiveMinutes(Timestamp)")
		require.Contains(t, sql, "`test_db`.`otel_logs`")
		require.NotContains(t, sql, "TimestampTime")
		require.Contains(t, sql, "EventName")
		require.Contains(t, sql, "__otel_materialized_k8s.namespace.name")
		require.Contains(t, sql, "__otel_materialized_deployment.environment.name")
		require.Contains(t, sql, "toDateTime(Timestamp)")
	})

	t.Run("full text search indexes for CH >= 26.2", func(t *testing.T) {
		sql, err := renderCreateLogsTableSQL(cfg, true)
		require.NoError(t, err)

		require.Contains(t, sql, "TYPE text(tokenizer = 'array')")
		require.Contains(t, sql, "TYPE text(tokenizer = 'splitByNonAlpha')")
		require.NotContains(t, sql, "TYPE bloom_filter")
		require.NotContains(t, sql, "tokenbf_v1")

		require.Contains(t, sql, "toStartOfFiveMinutes(Timestamp)")
		require.Contains(t, sql, "`test_db`.`otel_logs`")
		require.NotContains(t, sql, "TimestampTime")
		require.Contains(t, sql, "EventName")
		require.Contains(t, sql, "__otel_materialized_k8s.namespace.name")
	})

	t.Run("no TTL when zero", func(t *testing.T) {
		noTTLCfg := withDefaultConfig(func(c *Config) {
			c.Endpoint = defaultEndpoint
			c.Database = "test_db"
			c.TTL = 0
		})
		sql, err := renderCreateLogsTableSQL(noTTLCfg, false)
		require.NoError(t, err)
		require.NotContains(t, sql, "TTL")
	})
}

func TestRenderCreateLogsJSONTableSQL(t *testing.T) {
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = defaultEndpoint
		c.Database = "test_db"
		c.LogsTableName = "otel_logs_json"
		c.TTL = 72 * time.Hour
	})

	sql, err := renderCreateLogsJSONTableSQL(cfg)
	require.NoError(t, err)

	require.Contains(t, sql, "`ResourceAttributes` JSON")
	require.Contains(t, sql, "`ScopeAttributes` JSON")
	require.Contains(t, sql, "`LogAttributes` JSON")

	require.Contains(t, sql, "toStartOfFiveMinutes(Timestamp)")
	require.Contains(t, sql, "`test_db`.`otel_logs_json`")

	require.Contains(t, sql, "TYPE bloom_filter")
	require.NotContains(t, sql, "TYPE text(")
}
