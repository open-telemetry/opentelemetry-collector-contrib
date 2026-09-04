// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type testTelemetry struct {
	reader        *sdkmetric.ManualReader
	meterProvider *sdkmetric.MeterProvider
}

func setupTestTelemetry() testTelemetry {
	reader := sdkmetric.NewManualReader()
	return testTelemetry{
		reader:        reader,
		meterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)),
	}
}

func getMetric(name string, got metricdata.ResourceMetrics) metricdata.Metrics {
	for _, sm := range got.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}
	return metricdata.Metrics{}
}

func newStartedTailStorage(t *testing.T) TailStorage {
	t.Helper()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = t.TempDir()

	ext, err := f.Create(
		t.Context(),
		extensiontest.NewNopSettings(f.Type()),
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, ext.Shutdown(t.Context()))
	})

	storage, ok := ext.(TailStorage)
	require.True(t, ok)
	return storage
}

func appendTraceSpan(storage TailStorage, traceID pcommon.TraceID, spanID pcommon.SpanID, name string) {
	td := ptrace.NewTraces()
	rss := td.ResourceSpans().AppendEmpty()
	span := rss.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	if spanID != (pcommon.SpanID{}) {
		span.SetSpanID(spanID)
	}
	if name != "" {
		span.SetName(name)
	}
	_ = storage.Append(traceID, td)
}

func TestAppendThenTake(t *testing.T) {
	storage := newStartedTailStorage(t)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	appendTraceSpan(storage, traceID, pcommon.SpanID{}, "")

	out, err := storage.Take(traceID)
	require.NoError(t, err)
	require.Equal(t, 1, out.SpanCount())
}

func TestDeleteRemovesOnlyTargetTrace(t *testing.T) {
	storage := newStartedTailStorage(t)

	traceID1 := pcommon.TraceID([16]byte{1, 2, 3, 4})
	traceID2 := pcommon.TraceID([16]byte{1, 2, 3, 5})

	// Append multiple entries for traceID1 to exercise range deletion.
	for i := range 3 {
		appendTraceSpan(storage, traceID1, pcommon.SpanID([8]byte{byte(i + 1)}), "trace1-span")
	}

	appendTraceSpan(storage, traceID2, pcommon.SpanID{}, "")

	err := storage.Delete(traceID1)
	require.NoError(t, err)

	out, err := storage.Take(traceID1)
	require.NoError(t, err)
	require.Equal(t, 0, out.SpanCount())

	out2, err := storage.Take(traceID2)
	require.NoError(t, err)
	require.Equal(t, 1, out2.SpanCount())
}

func TestTakeRemovesOnlyTargetTrace(t *testing.T) {
	storage := newStartedTailStorage(t)

	traceID1 := pcommon.TraceID([16]byte{9, 9, 9, 1})
	traceID2 := pcommon.TraceID([16]byte{9, 9, 9, 2})

	for i := range 3 {
		appendTraceSpan(storage, traceID1, pcommon.SpanID([8]byte{byte(i + 1)}), "")
	}

	appendTraceSpan(storage, traceID2, pcommon.SpanID{}, "")

	out1, err := storage.Take(traceID1)
	require.NoError(t, err)
	require.Equal(t, 3, out1.SpanCount())

	out2, err := storage.Take(traceID1)
	require.NoError(t, err)
	require.Equal(t, 0, out2.SpanCount())

	out3, err := storage.Take(traceID2)
	require.NoError(t, err)
	require.Equal(t, 1, out3.SpanCount())
}

func TestDropOnStart(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = t.TempDir()

	zc, logs := observer.New(zap.InfoLevel)
	set := extensiontest.NewNopSettings(f.Type())
	set.Logger = zap.New(zc)

	first, err := f.Create(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NoError(t, first.Start(t.Context(), componenttest.NewNopHost()))

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	appendTraceSpan(first.(TailStorage), traceID, pcommon.SpanID{}, "")

	require.NoError(t, first.Shutdown(t.Context()))

	second, err := f.Create(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NoError(t, second.Start(t.Context(), componenttest.NewNopHost()))

	out, err := second.(TailStorage).Take(traceID)
	require.NoError(t, err)
	require.Equal(t, 0, out.SpanCount())

	require.NoError(t, second.Shutdown(t.Context()))

	assert.Equal(t, 1, logs.FilterMessage("existing database found; dropping all data as persistence across restarts is not supported").Len())
}

type fakeIter struct {
	seekOK    bool
	valid     []bool
	values    [][]byte
	valueErrs []error
	iterErr   error
	idx       int
}

func (f *fakeIter) SeekPrefixGE([]byte) bool { return f.seekOK }

func (f *fakeIter) Valid() bool {
	if f.idx >= len(f.valid) {
		return false
	}
	return f.valid[f.idx]
}

func (f *fakeIter) Next() bool {
	f.idx++
	return f.Valid()
}

func (f *fakeIter) ValueAndErr() ([]byte, error) {
	var val []byte
	if f.idx < len(f.values) {
		val = f.values[f.idx]
	}
	var err error
	if f.idx < len(f.valueErrs) {
		err = f.valueErrs[f.idx]
	}
	return val, err
}

func (f *fakeIter) Error() error { return f.iterErr }

func (*fakeIter) Close() error { return nil }

func TestStorageRecordsReadPathErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		metricName string
		desc       string
		newIter    func() (storageIter, error)
	}{
		{
			name:       "iter create",
			metricName: "otelcol_extension_pebble_tail_storage_read_errors",
			desc:       "Count of Pebble tail storage read-path iterator creation, value read, payload decode, and iterator terminal errors [Development]",
			newIter: func() (storageIter, error) {
				return nil, errors.New("iter create failed")
			},
		},
		{
			name:       "value read",
			metricName: "otelcol_extension_pebble_tail_storage_read_errors",
			desc:       "Count of Pebble tail storage read-path iterator creation, value read, payload decode, and iterator terminal errors [Development]",
			newIter: func() (storageIter, error) {
				return &fakeIter{
					seekOK:    true,
					valid:     []bool{true, false},
					valueErrs: []error{errors.New("value read failed")},
				}, nil
			},
		},
		{
			name:       "decode",
			metricName: "otelcol_extension_pebble_tail_storage_read_errors",
			desc:       "Count of Pebble tail storage read-path iterator creation, value read, payload decode, and iterator terminal errors [Development]",
			newIter: func() (storageIter, error) {
				return &fakeIter{
					seekOK: true,
					valid:  []bool{true, false},
					values: [][]byte{[]byte("not-a-trace")},
				}, nil
			},
		},
		{
			name:       "iter terminal",
			metricName: "otelcol_extension_pebble_tail_storage_read_errors",
			desc:       "Count of Pebble tail storage read-path iterator creation, value read, payload decode, and iterator terminal errors [Development]",
			newIter: func() (storageIter, error) {
				return &fakeIter{
					seekOK:  true,
					valid:   []bool{false},
					iterErr: errors.New("iter terminal failed"),
				}, nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tel := setupTestTelemetry()
			t.Cleanup(func() {
				require.NoError(t, tel.meterProvider.Shutdown(t.Context()))
			})

			set := extensiontest.NewNopSettings(typ)
			set.MeterProvider = tel.meterProvider

			ext, err := newExtension(set, &Config{})
			require.NoError(t, err)

			s := &storage{
				logger:      zap.NewNop(),
				telemetry:   ext.telemetry,
				unmarshaler: &ptrace.ProtoUnmarshaler{},
			}
			s.newIter = tc.newIter

			_ = s.readByTracePrefix([]byte("trace-prefix"))

			var md metricdata.ResourceMetrics
			require.NoError(t, tel.reader.Collect(t.Context(), &md))
			metricdatatest.AssertEqual(t, metricdata.Metrics{
				Name:        tc.metricName,
				Description: tc.desc,
				Unit:        "{errors}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints:  []metricdata.DataPoint[int64]{{Value: 1}},
				},
			}, getMetric(tc.metricName, md), metricdatatest.IgnoreTimestamp())
		})
	}
}
