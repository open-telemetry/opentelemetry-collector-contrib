// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

type failingTailStorage struct {
	appendErr error
	takeErr   error
	deleteErr error
}

func (f *failingTailStorage) Append(_ pcommon.TraceID, _ ptrace.Traces) error { return f.appendErr }

func (f *failingTailStorage) Take(_ pcommon.TraceID) (ptrace.Traces, error) {
	return ptrace.NewTraces(), f.takeErr
}

func (f *failingTailStorage) Delete(_ pcommon.TraceID) error { return f.deleteErr }

func (*failingTailStorage) Close() error { return nil }

func TestExtensionRecordsStorageErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		metricName string
		desc       string
		run        func(t *testing.T, ext *pebbleTailStorageExtension)
	}{
		{
			name:       "append",
			metricName: "otelcol_extension_pebble_tail_storage_append_errors",
			desc:       "Count of errors returned by the Pebble tail storage extension Append operation [Development]",
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{appendErr: errors.New("append failed")}
				require.Error(t, ext.Append(pcommon.TraceID([16]byte{1}), newTestTraces()))
			},
		},
		{
			name:       "take",
			metricName: "otelcol_extension_pebble_tail_storage_take_errors",
			desc:       "Count of errors returned by the Pebble tail storage extension Take operation [Development]",
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{takeErr: errors.New("take failed")}
				_, err := ext.Take(pcommon.TraceID([16]byte{1}))
				require.Error(t, err)
			},
		},
		{
			name:       "delete",
			metricName: "otelcol_extension_pebble_tail_storage_delete_errors",
			desc:       "Count of errors returned by the Pebble tail storage extension Delete operation [Development]",
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{deleteErr: errors.New("delete failed")}
				require.Error(t, ext.Delete(pcommon.TraceID([16]byte{1})))
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

			tc.run(t, ext)

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

func newTestTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1}))
	span.SetSpanID(pcommon.SpanID([8]byte{1}))
	return td
}
