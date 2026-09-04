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
	"go.opentelemetry.io/otel/attribute"
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

func TestExtensionRecordsStorageOperations(t *testing.T) {
	const metricName = "otelcol_extension_pebble_tail_storage_operations"
	const desc = "Count of Pebble tail storage operations by operation and outcome [Development]"

	for _, tc := range []struct {
		name      string
		operation string
		outcome   string
		run       func(t *testing.T, ext *pebbleTailStorageExtension)
	}{
		{
			name:      "append success",
			operation: operationAppend,
			outcome:   outcomeSuccess,
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{}
				require.NoError(t, ext.Append(pcommon.TraceID([16]byte{1}), newTestTraces()))
			},
		},
		{
			name:      "append failure",
			operation: operationAppend,
			outcome:   outcomeFailure,
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{appendErr: errors.New("append failed")}
				require.Error(t, ext.Append(pcommon.TraceID([16]byte{1}), newTestTraces()))
			},
		},
		{
			name:      "take success",
			operation: operationTake,
			outcome:   outcomeSuccess,
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{}
				_, err := ext.Take(pcommon.TraceID([16]byte{1}))
				require.NoError(t, err)
			},
		},
		{
			name:      "take failure",
			operation: operationTake,
			outcome:   outcomeFailure,
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{takeErr: errors.New("take failed")}
				_, err := ext.Take(pcommon.TraceID([16]byte{1}))
				require.Error(t, err)
			},
		},
		{
			name:      "delete success",
			operation: operationDelete,
			outcome:   outcomeSuccess,
			run: func(t *testing.T, ext *pebbleTailStorageExtension) {
				ext.storage = &failingTailStorage{}
				require.NoError(t, ext.Delete(pcommon.TraceID([16]byte{1})))
			},
		},
		{
			name:      "delete failure",
			operation: operationDelete,
			outcome:   outcomeFailure,
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
				Name:        metricName,
				Description: desc,
				Unit:        "{operations}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{{
						Attributes: attribute.NewSet(
							attribute.String(attrOperation, tc.operation),
							attribute.String(attrOutcome, tc.outcome),
						),
						Value: 1,
					}},
				},
			}, getMetric(metricName, md), metricdatatest.IgnoreTimestamp())
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
