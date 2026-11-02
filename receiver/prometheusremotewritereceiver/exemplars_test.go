// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"strings"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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
