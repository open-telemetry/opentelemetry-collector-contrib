// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metrics"
)

// fakeBatch records the rows appended to it. Methods that are not overridden panic, so a test
// fails loudly if the exporter touches the batch in an unexpected way.
type fakeBatch struct {
	driver.Batch
	conn  *fakeConn
	query string
	rows  [][]any
	sent  bool
}

func (b *fakeBatch) Append(v ...any) error {
	b.rows = append(b.rows, v)
	return nil
}

func (b *fakeBatch) Send() error {
	b.sent = true
	return nil
}

func (*fakeBatch) Close() error { return nil }

// fakeConn is a driver.Conn that hands out fakeBatches, so no ClickHouse server is needed.
type fakeConn struct {
	driver.Conn
	mu      sync.Mutex
	batches []*fakeBatch
	err     error
}

func (c *fakeConn) PrepareBatch(_ context.Context, query string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	b := &fakeBatch{conn: c, query: query}
	c.batches = append(c.batches, b)
	return b, nil
}

// rowsByTable returns the number of rows sent to each table, keyed by table name.
func (c *fakeConn) rowsByTable(t *testing.T, cfg *Config) map[string]int {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	tables := map[string]string{
		cfg.MetricsTables.Gauge.Name:                "gauge",
		cfg.MetricsTables.Sum.Name:                  "sum",
		cfg.MetricsTables.Histogram.Name:            "histogram",
		cfg.MetricsTables.ExponentialHistogram.Name: "exponential_histogram",
		cfg.MetricsTables.Summary.Name:              "summary",
	}
	out := map[string]int{}
	for _, b := range c.batches {
		require.True(t, b.sent, "batch for %q was prepared but never sent", b.query)
		found := false
		for name, kind := range tables {
			if strings.Contains(b.query, `."`+name+`"`) {
				out[kind] += len(b.rows)
				found = true
				break
			}
		}
		require.True(t, found, "unexpected insert query %q", b.query)
	}
	return out
}

func newSkipTestExporter(t *testing.T) (*metricsExporter, *fakeConn, *observer.ObservedLogs) {
	t.Helper()
	metrics.SetLogger(zap.NewNop())
	core, logs := observer.New(zap.DebugLevel)
	exp := newMetricsExporter(zap.New(core), withDefaultConfig())
	conn := &fakeConn{}
	exp.db = conn // never call start(); no ClickHouse server involved
	return exp, conn, logs
}

func appendGauge(sm pmetric.ScopeMetrics, name string, points int) {
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	g := m.SetEmptyGauge()
	for range points {
		dp := g.DataPoints().AppendEmpty()
		dp.SetDoubleValue(42)
		dp.SetTimestamp(pcommon.Timestamp(1_700_000_000_000_000_000))
	}
}

func appendSum(sm pmetric.ScopeMetrics, name string) {
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)
	m.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(1)
}

// appendUntyped adds a metric whose data type is never set, as emitted by e.g. OTel Python SDK < 1.21.0
// for a histogram with no measurements.
func appendUntyped(sm pmetric.ScopeMetrics, name string) {
	sm.Metrics().AppendEmpty().SetName(name)
}

func TestPushMetricsData_SkipsUntypedMetrics(t *testing.T) {
	tests := []struct {
		name          string
		build         func(md pmetric.Metrics)
		expectedRows  map[string]int
		expectSkipped int
	}{
		{
			name: "untyped metric in a different resource",
			build: func(md pmetric.Metrics) {
				good := md.ResourceMetrics().AppendEmpty()
				appendGauge(good.ScopeMetrics().AppendEmpty(), "good", 2)
				bad := md.ResourceMetrics().AppendEmpty()
				appendUntyped(bad.ScopeMetrics().AppendEmpty(), "bad")
			},
			expectedRows:  map[string]int{"gauge": 2},
			expectSkipped: 1,
		},
		{
			name: "untyped metric in a different scope of the same resource",
			build: func(md pmetric.Metrics) {
				rm := md.ResourceMetrics().AppendEmpty()
				appendGauge(rm.ScopeMetrics().AppendEmpty(), "good", 1)
				appendUntyped(rm.ScopeMetrics().AppendEmpty(), "bad")
			},
			expectedRows:  map[string]int{"gauge": 1},
			expectSkipped: 1,
		},
		{
			name: "untyped metric between valid metrics of different types in the same scope",
			build: func(md pmetric.Metrics) {
				sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				appendGauge(sm, "gauge", 1)
				appendUntyped(sm, "bad")
				appendSum(sm, "sum")
			},
			expectedRows:  map[string]int{"gauge": 1, "sum": 1},
			expectSkipped: 1,
		},
		{
			name: "multiple untyped metrics",
			build: func(md pmetric.Metrics) {
				sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				appendUntyped(sm, "bad1")
				appendGauge(sm, "good", 1)
				appendUntyped(sm, "bad2")
			},
			expectedRows:  map[string]int{"gauge": 1},
			expectSkipped: 2,
		},
		{
			name: "only untyped metrics",
			build: func(md pmetric.Metrics) {
				appendUntyped(md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty(), "bad")
			},
			expectedRows:  map[string]int{},
			expectSkipped: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, conn, logs := newSkipTestExporter(t)
			md := pmetric.NewMetrics()
			tt.build(md)

			// A nil error means exporterhelper neither retries nor drops the request.
			require.NoError(t, exp.pushMetricsData(t.Context(), md))
			assert.Equal(t, tt.expectedRows, conn.rowsByTable(t, exp.cfg))

			warns := logs.FilterLevelExact(zap.WarnLevel).All()
			require.Len(t, warns, 1, "expected exactly one summary warning per batch")
			fields := warns[0].ContextMap()
			assert.EqualValues(t, tt.expectSkipped, fields["skipped_metrics"])
			assert.Equal(t, map[string]int{"Empty": tt.expectSkipped}, fields["skipped_by_type"])
		})
	}
}

func TestPushMetricsData_NoWarningWithoutSkippedMetrics(t *testing.T) {
	exp, conn, logs := newSkipTestExporter(t)
	md := pmetric.NewMetrics()
	appendGauge(md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty(), "good", 3)

	require.NoError(t, exp.pushMetricsData(t.Context(), md))
	assert.Equal(t, map[string]int{"gauge": 3}, conn.rowsByTable(t, exp.cfg))
	assert.Zero(t, logs.FilterLevelExact(zap.WarnLevel).Len())
}

func TestPushMetricsData_SkippedMetricNamesAreBounded(t *testing.T) {
	exp, _, logs := newSkipTestExporter(t)
	md := pmetric.NewMetrics()
	sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	for range maxSkippedMetricNames * 3 {
		appendUntyped(sm, "bad")
	}

	require.NoError(t, exp.pushMetricsData(t.Context(), md))

	warns := logs.FilterLevelExact(zap.WarnLevel).All()
	require.Len(t, warns, 1)
	fields := warns[0].ContextMap()
	assert.EqualValues(t, maxSkippedMetricNames*3, fields["skipped_metrics"])
	assert.Len(t, fields["example_metric_names"], maxSkippedMetricNames)
}

// Insert failures must still be returned so they are retried.
func TestPushMetricsData_InsertErrorStillReturned(t *testing.T) {
	exp, conn, _ := newSkipTestExporter(t)
	errInsert := errors.New("insert failed")
	conn.err = errInsert

	md := pmetric.NewMetrics()
	sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	appendGauge(sm, "good", 1)
	appendUntyped(sm, "bad")

	require.ErrorIs(t, exp.pushMetricsData(t.Context(), md), errInsert)
}
