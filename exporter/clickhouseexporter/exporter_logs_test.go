// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
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

// TestLogsExporter_PushDeferredOnSchemaUnknown is the sister of
// TestTracesJSONExporter_PushDeferredOnSchemaUnknown for the default (non-JSON)
// logs exporter. Verifies that #48875's fix is applied to logs.go: when the
// schema detector is still schemaUnknown (transient ClickHouse outage at start),
// pushLogsData re-probes via ensureDetected and on continued failure returns a
// retryable plain error so exporterhelper's retry_on_failure defers the batch.
func TestLogsExporter_PushDeferredOnSchemaUnknown(t *testing.T) {
	cfg := withDefaultConfig()
	cfg.Endpoint = defaultEndpoint
	exp := newLogsExporter(zaptest.NewLogger(t), cfg)
	exp.db = &stubFailingConn{}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty().Body().SetStr("hello")

	err := exp.pushLogsData(t.Context(), ld)
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema detection deferred",
		"push must return a retryable error wrapping the DESC failure")
	require.Equal(t, schemaUnknown, schemaDetectionState(exp.detector.state.Load()))
	require.Empty(t, exp.insertSQL)
}

// TestLogsJSONExporter_PushDeferredOnSchemaUnknown is the sister of
// TestTracesJSONExporter_PushDeferredOnSchemaUnknown for the JSON logs
// exporter. See #48875.
func TestLogsJSONExporter_PushDeferredOnSchemaUnknown(t *testing.T) {
	cfg := withDefaultConfig()
	cfg.Endpoint = defaultEndpoint
	exp := newLogsJSONExporter(zaptest.NewLogger(t), cfg)
	exp.db = &stubFailingConn{}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty().Body().SetStr("hello")

	err := exp.pushLogsData(t.Context(), ld)
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema detection deferred",
		"push must return a retryable error wrapping the DESC failure")
	require.Equal(t, schemaUnknown, schemaDetectionState(exp.detector.state.Load()))
	require.Empty(t, exp.insertSQL)
}
