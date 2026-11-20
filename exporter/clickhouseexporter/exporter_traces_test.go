// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

// recordingBatch captures every row passed to Append.
type recordingBatch struct {
	noopBatch
	rows [][]any
}

func (b *recordingBatch) Append(v ...any) error {
	b.rows = append(b.rows, append([]any(nil), v...))
	return nil
}

// recordingConn returns the same recordingBatch for every PrepareBatch call.
type recordingConn struct {
	noopConn
	batch *recordingBatch
}

func (c recordingConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return c.batch, nil
}

// insertColumnsAndPositions splits a rendered INSERT statement into its column names and value placeholders.
func insertColumnsAndPositions(t *testing.T, sql string) (columns, positions []string) {
	t.Helper()

	columnsStart := strings.Index(sql, "(")
	valuesIdx := strings.Index(sql, ") VALUES (")
	require.Positive(t, columnsStart)
	require.Greater(t, valuesIdx, columnsStart)

	splitList := func(s string) []string {
		var out []string
		for item := range strings.SplitSeq(s, ",") {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	}

	columns = splitList(sql[columnsStart+1 : valuesIdx])
	positions = splitList(strings.TrimSuffix(strings.TrimSpace(sql[valuesIdx+len(") VALUES ("):]), ")"))
	return columns, positions
}

func TestRenderInsertTracesSQL(t *testing.T) {
	tests := map[string]struct {
		scopeAttributes bool
	}{
		"without schema features": {scopeAttributes: false},
		"with ScopeAttributes":    {scopeAttributes: true},
	}

	cfg := withDefaultConfig(func(c *Config) {
		c.Endpoint = defaultEndpoint
		c.Database = "test_db"
		c.TracesTableName = "test_traces"
	})

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			exporter := newTracesExporter(zap.NewNop(), cfg)
			exporter.schemaFeatures.ScopeAttributes = test.scopeAttributes
			exporter.renderInsertTracesSQL()

			require.True(t, strings.HasPrefix(exporter.insertSQL, `INSERT INTO "test_db"."test_traces" (`), exporter.insertSQL)

			columns, positions := insertColumnsAndPositions(t, exporter.insertSQL)
			require.Len(t, positions, len(columns), "every column must have a matching placeholder")
			for _, p := range positions {
				require.Equal(t, "?", p)
			}

			if test.scopeAttributes {
				require.Len(t, columns, 23)
				require.Equal(t, tracesColumnScopeAttributes, columns[len(columns)-1])
			} else {
				require.Len(t, columns, 22)
				require.NotContains(t, columns, tracesColumnScopeAttributes)
			}
		})
	}
}

func TestRenderCreateTracesTableSQL(t *testing.T) {
	sql := renderCreateTracesTableSQL(withDefaultConfig())
	require.Contains(t, sql, "ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(3)),")
}

func TestPushTraceDataScopeAttributes(t *testing.T) {
	newTraces := func() (ptrace.Traces, pcommon.Map) {
		traces := ptrace.NewTraces()
		ss := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		ss.Scope().SetName("io.opentelemetry.contrib.clickhouse")
		ss.Scope().Attributes().PutStr("lib", "clickhouse")
		ss.Scope().Attributes().PutInt("lib.version", 2)
		ss.Spans().AppendEmpty().SetName("span-1")
		ss.Spans().AppendEmpty().SetName("span-2")
		return traces, ss.Scope().Attributes()
	}

	tests := map[string]struct {
		scopeAttributes bool
		wantColumns     int
	}{
		"without schema features": {scopeAttributes: false, wantColumns: 22},
		"with ScopeAttributes":    {scopeAttributes: true, wantColumns: 23},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			batch := &recordingBatch{}
			exporter := newTracesExporter(zap.NewNop(), withDefaultConfig())
			exporter.db = recordingConn{batch: batch}
			exporter.schemaFeatures.ScopeAttributes = test.scopeAttributes
			exporter.renderInsertTracesSQL()

			traces, scopeAttrs := newTraces()
			require.NoError(t, exporter.pushTraceData(t.Context(), traces))
			require.Len(t, batch.rows, 2)

			// Row width must match the rendered INSERT so the driver can bind every value.
			columns, _ := insertColumnsAndPositions(t, exporter.insertSQL)
			for _, row := range batch.rows {
				require.Len(t, row, test.wantColumns)
				require.Len(t, row, len(columns))
				if test.scopeAttributes {
					require.Equal(t, internal.AttributesToMap(scopeAttrs), row[len(row)-1])
				}
			}
		})
	}
}
