// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pebbletailstorageextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
)

func TestDefaultConfigReadErrorPolicy(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	assert.Equal(t, ReadErrorPolicyDropTrace, cfg.OnReadError)
}

func newStorageWithReadFailure(t *testing.T, policy ReadErrorPolicy, traceID pcommon.TraceID) (*storage, testTelemetry) {
	t.Helper()

	tel := setupTestTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tel.meterProvider.Shutdown(t.Context()))
	})
	set := extensiontest.NewNopSettings(typ)
	set.MeterProvider = tel.meterProvider
	ext, err := newExtension(set, &Config{})
	require.NoError(t, err)

	s, err := newStorage(t.Context(), &Config{Directory: t.TempDir(), OnReadError: policy}, zap.NewNop(), ext.telemetry)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, s.Close())
	})

	good := ptrace.NewTraces()
	good.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("readable")
	require.NoError(t, s.Append(traceID, good))
	require.NoError(t, s.Append(traceID, ptrace.NewTraces()))

	realIter := s.newIter
	s.newIter = func() (storageIter, error) {
		iter, err := realIter()
		if err != nil {
			return nil, err
		}
		return &corruptSecondValueIter{storageIter: iter}, nil
	}
	return s, tel
}

type corruptSecondValueIter struct {
	storageIter
	n int
}

func (c *corruptSecondValueIter) ValueAndErr() ([]byte, error) {
	c.n++
	if c.n == 2 {
		return []byte("not-a-trace"), nil
	}
	return c.storageIter.ValueAndErr()
}

func assertCounter(t *testing.T, tel testTelemetry, name string, want int64) {
	t.Helper()
	var md metricdata.ResourceMetrics
	require.NoError(t, tel.reader.Collect(t.Context(), &md))
	m := getMetric(name, md)
	if want == 0 {
		assert.Empty(t, m.Name, "metric %s should not be recorded", name)
		return
	}
	require.NotEmpty(t, m.Name, "metric %s not recorded", name)
	metricdatatest.AssertEqual(t, metricdata.Metrics{
		Name:        m.Name,
		Description: m.Description,
		Unit:        "{traces}",
		Data: metricdata.Sum[int64]{
			IsMonotonic: true,
			Temporality: metricdata.CumulativeTemporality,
			DataPoints:  []metricdata.DataPoint[int64]{{Value: want}},
		},
	}, m, metricdatatest.IgnoreTimestamp())
}

func TestTakeReadErrorDropTrace(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{7})
	s, tel := newStorageWithReadFailure(t, ReadErrorPolicyDropTrace, traceID)

	td, err := s.Take(traceID)
	require.ErrorIs(t, err, errReadErrorDropTrace)
	assert.Equal(t, 0, td.ResourceSpans().Len())

	assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_trace_drops", 1)
	assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_partial_returns", 0)

	s.newIter = func() (storageIter, error) { return s.db.NewIter(nil) }
	td, err = s.Take(traceID)
	require.NoError(t, err)
	assert.Equal(t, 0, td.ResourceSpans().Len())
}

func TestTakeReadErrorReturnPartial(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{8})
	s, tel := newStorageWithReadFailure(t, ReadErrorPolicyReturnPartial, traceID)

	td, err := s.Take(traceID)
	require.NoError(t, err)
	require.Equal(t, 1, td.ResourceSpans().Len())
	assert.Equal(t, "readable", td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())

	assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_partial_returns", 1)
	assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_trace_drops", 0)

	s.newIter = func() (storageIter, error) { return s.db.NewIter(nil) }
	td, err = s.Take(traceID)
	require.NoError(t, err)
	assert.Equal(t, 0, td.ResourceSpans().Len())
}

func TestTakeReadErrorDefaultsToDropTrace(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{9})
	s, _ := newStorageWithReadFailure(t, "", traceID)

	_, err := s.Take(traceID)
	require.ErrorIs(t, err, errReadErrorDropTrace)
}

func TestTakeWithoutReadErrorIsUnchanged(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{10})
	s, tel := newStorageWithReadFailure(t, ReadErrorPolicyDropTrace, traceID)
	s.newIter = func() (storageIter, error) { return s.db.NewIter(nil) }

	td, err := s.Take(traceID)
	require.NoError(t, err)
	assert.Equal(t, 1, td.ResourceSpans().Len())
	assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_trace_drops", 0)
	assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_partial_returns", 0)
}
