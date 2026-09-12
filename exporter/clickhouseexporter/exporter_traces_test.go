// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderCreateTracesTableSQL(t *testing.T) {
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = defaultEndpoint
		c.Database = "test_db"
		c.TracesTableName = "otel_traces"
		c.TTL = 72 * time.Hour
	})

	t.Run("bloom filter indexes for CH < 26.2", func(t *testing.T) {
		sql, err := renderCreateTracesTableSQL(cfg, false)
		require.NoError(t, err)

		require.Contains(t, sql, "TYPE bloom_filter")
		require.NotContains(t, sql, "TYPE text(")

		require.Contains(t, sql, "`test_db`.`otel_traces`")
		require.Contains(t, sql, "PRIMARY KEY (ServiceName, SpanName, toDateTime(Timestamp))")
		require.Contains(t, sql, "ORDER BY (ServiceName, SpanName, toDateTime(Timestamp), Timestamp)")
		require.Contains(t, sql, "Duration UInt64 CODEC(T64, ZSTD(1))")
		require.Contains(t, sql, "ResourceAttributeItems Array(String) ALIAS")
		require.Contains(t, sql, "SpanAttributeItems Array(String) ALIAS")
		require.Contains(t, sql, "TYPE minmax")
		require.Contains(t, sql, "TTL toDateTime(Timestamp) + toIntervalDay(3)")

		require.Contains(t, sql, "INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter")
		require.Contains(t, sql, "INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter")

		require.NotContains(t, sql, "__otel_materialized")
	})

	t.Run("full text search indexes for CH >= 26.2", func(t *testing.T) {
		sql, err := renderCreateTracesTableSQL(cfg, true)
		require.NoError(t, err)

		require.Contains(t, sql, "TYPE text(tokenizer = 'array')")
		require.NotContains(t, sql, "TYPE bloom_filter")

		require.Contains(t, sql, "INDEX idx_trace_id TraceId TYPE text(tokenizer = 'array')")
		require.Contains(t, sql, "INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE text(tokenizer = 'array')")
		require.Contains(t, sql, "INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE text(tokenizer = 'array')")
		require.Contains(t, sql, "INDEX idx_res_attr_items ResourceAttributeItems TYPE text(tokenizer = 'array')")
		require.Contains(t, sql, "INDEX idx_span_attr_items SpanAttributeItems TYPE text(tokenizer = 'array')")
		require.Contains(t, sql, "`test_db`.`otel_traces`")
		require.Contains(t, sql, "TYPE minmax")

		require.NotContains(t, sql, "mapValues")
		require.NotContains(t, sql, "idx_res_attr_value")
		require.NotContains(t, sql, "idx_span_attr_value")

		require.NotContains(t, sql, "__otel_materialized")
	})

	t.Run("no TTL when zero", func(t *testing.T) {
		noTTLCfg := withDefaultConfig(func(c *Config) {
			c.Endpoint = defaultEndpoint
			c.Database = "test_db"
			c.TTL = 0
		})

		sql, err := renderCreateTracesTableSQL(noTTLCfg, false)
		require.NoError(t, err)
		require.NotContains(t, sql, "TTL ")
	})
}

func TestRenderCreateTraceIDTsTableSQL(t *testing.T) {
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = defaultEndpoint
		c.Database = "test_db"
		c.TracesTableName = "otel_traces"
		c.TTL = 72 * time.Hour
	})

	sql, err := renderCreateTraceIDTsTableSQL(cfg)
	require.NoError(t, err)

	require.Contains(t, sql, "`test_db`.`otel_traces_trace_id_ts`")
	require.Contains(t, sql, "ENGINE = AggregatingMergeTree()")
	require.Contains(t, sql, "Start SimpleAggregateFunction(min, DateTime)")
	require.Contains(t, sql, "End SimpleAggregateFunction(max, DateTime)")
	require.Contains(t, sql, "PARTITION BY toStartOfWeek(Start)")
	require.Contains(t, sql, "ORDER BY (TraceId)")
	require.NotContains(t, sql, "bloom_filter")
	require.Contains(t, sql, "TTL toDateTime(Start) + toIntervalDay(3)")
}

func TestRenderTraceIDTsMaterializedViewSQL(t *testing.T) {
	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = defaultEndpoint
		c.Database = "test_db"
		c.TracesTableName = "otel_traces"
	})

	sql, err := renderTraceIDTsMaterializedViewSQL(cfg)
	require.NoError(t, err)

	require.Contains(t, sql, "`test_db`.`otel_traces_trace_id_ts_mv`")
	require.Contains(t, sql, "TO `test_db`.`otel_traces_trace_id_ts`")
	require.Contains(t, sql, "FROM `test_db`.`otel_traces`")
	require.Contains(t, sql, "GROUP BY TraceId")
}

func TestTraceIDTsTableEngineString(t *testing.T) {
	tests := []struct {
		name     string
		engine   TableEngine
		expected string
	}{
		{name: "default", engine: TableEngine{}, expected: "AggregatingMergeTree()"},
		{name: "explicit MergeTree", engine: TableEngine{Name: "MergeTree"}, expected: "AggregatingMergeTree()"},
		{
			name:     "ReplicatedMergeTree",
			engine:   TableEngine{Name: "ReplicatedMergeTree", Params: "'/clickhouse/tables/{shard}/table_name', '{replica}'"},
			expected: "ReplicatedAggregatingMergeTree('/clickhouse/tables/{shard}/table_name', '{replica}')",
		},
		{
			name:     "custom engine unchanged",
			engine:   TableEngine{Name: "SharedMergeTree", Params: "'a', 'b'"},
			expected: "SharedMergeTree('a', 'b')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := withDefaultConfig(func(c *Config) {
				c.TableEngine = tt.engine
			})

			require.Equal(t, tt.expected, cfg.traceIDTsTableEngineString())
		})
	}
}
