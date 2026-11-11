// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestCollectExemplars_ErrorsAndEdgeCases(t *testing.T) {
	core, obs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	tests := []struct {
		name              string
		req               *writev2.Request
		expectedExemplars int
		expectedGroups    int
		expectedWarnMsgs  []string
	}{
		{
			name: "invalid exemplar label refs logs warning",
			req: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "request_duration_ms",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						LabelsRefs: []uint32{1, 2},
						Exemplars: []writev2.Exemplar{
							{
								Value:      1,
								Timestamp:  1,
								LabelsRefs: []uint32{100, 101},
							},
						},
					},
				},
			},
			expectedExemplars: 0,
			expectedGroups:    1,
			expectedWarnMsgs: []string{
				"error converting exemplar label refs",
			},
		},
		{
			name: "missing metric name logs warning",
			req: &writev2.Request{
				Symbols: []string{
					"",
					"job", "production/service",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						LabelsRefs: []uint32{1, 2},
						Exemplars: []writev2.Exemplar{
							{Value: 1, Timestamp: 1},
						},
					},
				},
			},
			expectedExemplars: 0,
			expectedGroups:    0,
			expectedWarnMsgs: []string{
				"missing metric name in labels",
			},
		},
		{
			name: "odd LabelsRefs logs warning",
			req: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "request_duration_ms",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						LabelsRefs: []uint32{1}, // odd length
						Exemplars: []writev2.Exemplar{
							{Value: 1, Timestamp: 1},
						},
					},
				},
			},
			expectedExemplars: 0,
			expectedGroups:    0,
			expectedWarnMsgs: []string{
				"failed to extract labels from request symbols",
			},
		},
		{
			name: "empty exemplar labels does not warn",
			req: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "request_duration_ms",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						LabelsRefs: []uint32{1, 2},
						Exemplars: []writev2.Exemplar{
							{
								Value:      1,
								Timestamp:  1,
								LabelsRefs: nil,
							},
						},
					},
				},
			},
			expectedExemplars: 1,
			expectedGroups:    1,
			expectedWarnMsgs:  nil,
		},
		{
			// A series that validateSeriesLabels would refuse loses the whole request, so its exemplars
			// are not collected either.
			name: "series with an empty label name is skipped",
			req: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "request_duration_ms",
					"http_method", "GET",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						LabelsRefs: []uint32{1, 2, 0, 2},
						Exemplars: []writev2.Exemplar{
							{Value: 1, Timestamp: 1, LabelsRefs: []uint32{3, 4}},
						},
					},
				},
			},
			expectedExemplars: 0,
			expectedGroups:    0,
			expectedWarnMsgs:  nil,
		},
		{
			name: "series with a duplicate label name is skipped",
			req: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "request_duration_ms",
					"http_method", "GET",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						LabelsRefs: []uint32{1, 2, 1, 2},
						Exemplars: []writev2.Exemplar{
							{Value: 1, Timestamp: 1, LabelsRefs: []uint32{3, 4}},
						},
					},
				},
			},
			expectedExemplars: 0,
			expectedGroups:    0,
			expectedWarnMsgs:  nil,
		},
		{
			name: "mixed valid and invalid exemplars logs once and keeps valid",
			req: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "request_duration_ms",
					"trace_id", "4bf92f3577b34da6a3ce929d0e0e4736",
					"span_id", "00f067aa0ba902b7",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						LabelsRefs: []uint32{1, 2},
						Exemplars: []writev2.Exemplar{
							{
								Value:      1,
								Timestamp:  1,
								LabelsRefs: []uint32{99, 100}, // invalid
							},
							{
								Value:     2,
								Timestamp: 2,
								LabelsRefs: []uint32{
									3, 4,
									5, 6,
								},
							},
						},
					},
				},
			},
			expectedExemplars: 1,
			expectedGroups:    1,
			expectedWarnMsgs: []string{
				"error converting exemplar label refs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs.TakeAll() // clear logs between subtests

			stats := &promremote.WriteResponseStats{}
			settings := receiver.Settings{
				TelemetrySettings: component.TelemetrySettings{
					Logger: logger,
				},
			}

			result := collectExemplars(tt.req, settings, stats)

			assert.Equal(t, tt.expectedExemplars, stats.Exemplars)
			assert.Len(t, result, tt.expectedGroups)

			warns := obs.FilterLevelExact(zapcore.WarnLevel).All()
			require.Len(t, warns, len(tt.expectedWarnMsgs))

			for i, msg := range tt.expectedWarnMsgs {
				assert.Equal(t, msg, warns[i].Message)
			}
		})
	}
}

func TestCollectExemplars_DropsEmptyLabelNames(t *testing.T) {
	// The exemplar label name resolves to symbol 0, the empty string the writev2 symbol table
	// starts with, so the name is empty without the request being malformed on its face. Such a
	// label has no attribute key to land under; the rest of the exemplar is kept.
	t.Run("empty label name is dropped", func(t *testing.T) {
		req := &writev2.Request{
			Symbols: []string{"", "__name__", "request_duration_ms", "orphan_value", "http_method", "GET"},
			Timeseries: []writev2.TimeSeries{
				{
					Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM},
					LabelsRefs: []uint32{1, 2},
					Exemplars: []writev2.Exemplar{
						{
							Value:      2.5,
							Timestamp:  1,
							LabelsRefs: []uint32{0, 3, 4, 5},
						},
					},
				},
			},
		}

		stats := &promremote.WriteResponseStats{}
		settings := receiver.Settings{
			TelemetrySettings: component.TelemetrySettings{
				Logger: zap.NewNop(),
			},
		}

		result := collectExemplars(req, settings, stats)
		require.Equal(t, 1, stats.Exemplars)

		// The exemplar stays filed under the series it arrived with, and only the empty-named label
		// is gone: the value, the timestamp and the labels around it are kept.
		exemplars, ok := result[makeExemplarKey(labels.FromStrings("__name__", "request_duration_ms"))]
		require.True(t, ok, "the exemplar stays under its own series")
		require.Equal(t, 1, exemplars.Len())
		assert.Equal(t, map[string]any{"http_method": "GET"}, exemplars.At(0).FilteredAttributes().AsRaw())
		assert.Equal(t, 2.5, exemplars.At(0).DoubleValue())
		assert.Equal(t, pcommon.Timestamp(int64(time.Millisecond)), exemplars.At(0).Timestamp())
	})
}

func TestSetTraceAndSpan(t *testing.T) {
	const (
		validTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
		validSpanID  = "00f067aa0ba902b7"
	)

	tests := []struct {
		name              string
		labels            labels.Labels
		wantTraceID       string // hex string, "" means the exemplar carries no trace ID
		wantSpanID        string // hex string, "" means the exemplar carries no span ID
		wantFilteredAttrs map[string]any
	}{
		{
			name:              "valid ids are converted and are not kept as attributes",
			labels:            labels.FromStrings("http_method", "GET", "span_id", validSpanID, "trace_id", validTraceID),
			wantTraceID:       validTraceID,
			wantSpanID:        validSpanID,
			wantFilteredAttrs: map[string]any{"http_method": "GET"},
		},
		{
			name:              "trace: short id is not zero padded and is kept as an attribute",
			labels:            labels.FromStrings("span_id", validSpanID, "trace_id", "aaaaaaaa"),
			wantTraceID:       "",
			wantSpanID:        validSpanID,
			wantFilteredAttrs: map[string]any{"trace_id": "aaaaaaaa"},
		},
		{
			name:              "trace: long id is not truncated and is kept as an attribute",
			labels:            labels.FromStrings("trace_id", strings.Repeat("a", 48)),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"trace_id": strings.Repeat("a", 48)},
		},
		{
			name:              "trace: odd length id is kept as an attribute",
			labels:            labels.FromStrings("trace_id", strings.Repeat("a", 31)),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"trace_id": strings.Repeat("a", 31)},
		},
		{
			name:              "trace: non hex id is kept as an attribute",
			labels:            labels.FromStrings("trace_id", strings.Repeat("z", 32)),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"trace_id": strings.Repeat("z", 32)},
		},
		{
			name:              "trace: all zero id is not a valid id and is kept as an attribute",
			labels:            labels.FromStrings("trace_id", strings.Repeat("0", 32)),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"trace_id": strings.Repeat("0", 32)},
		},
		{
			name:              "span: short id is not zero padded and is kept as an attribute",
			labels:            labels.FromStrings("span_id", "00f0", "trace_id", validTraceID),
			wantTraceID:       validTraceID,
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"span_id": "00f0"},
		},
		{
			name:              "span: long id is not truncated and is kept as an attribute",
			labels:            labels.FromStrings("span_id", strings.Repeat("b", 32)),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"span_id": strings.Repeat("b", 32)},
		},
		{
			name:              "span: odd length id is kept as an attribute",
			labels:            labels.FromStrings("span_id", strings.Repeat("b", 15)),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"span_id": strings.Repeat("b", 15)},
		},
		{
			name:              "span: non hex id is kept as an attribute",
			labels:            labels.FromStrings("span_id", strings.Repeat("z", 16)),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"span_id": strings.Repeat("z", 16)},
		},
		{
			name:              "span: all zero id is not a valid id and is kept as an attribute",
			labels:            labels.FromStrings("span_id", strings.Repeat("0", 16), "trace_id", validTraceID),
			wantTraceID:       validTraceID,
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"span_id": strings.Repeat("0", 16)},
		},
		{
			name:              "empty label values are dropped, as an empty label means an absent one",
			labels:            labels.FromStrings("http_method", "GET", "empty_label", "", "trace_id", "", "span_id", ""),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"http_method": "GET"},
		},
		{
			name:              "a label with an empty name is dropped, as it cannot become an attribute key",
			labels:            labels.FromStrings("", "orphan_value", "http_method", "GET"),
			wantTraceID:       "",
			wantSpanID:        "",
			wantFilteredAttrs: map[string]any{"http_method": "GET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exemplar := pmetric.NewExemplar()

			traceIDSet, spanIDSet := setTraceAndSpan(exemplar, tt.labels)
			copyExemplarAttributes(exemplar.FilteredAttributes(), tt.labels, traceIDSet, spanIDSet)

			// TraceID.String()/SpanID.String() return "" for an all-zero (unset) ID
			// and the lowercase hex encoding otherwise.
			assert.Equal(t, tt.wantTraceID, exemplar.TraceID().String())
			assert.Equal(t, tt.wantSpanID, exemplar.SpanID().String())
			assert.Equal(t, tt.wantFilteredAttrs, exemplar.FilteredAttributes().AsRaw())
		})
	}
}
