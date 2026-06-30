// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventhub

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/trigger"
)

type fakeMetricsUnmarshaler struct {
	metrics pmetric.Metrics
	err     error
}

func (f *fakeMetricsUnmarshaler) UnmarshalMetrics(_ []byte) (pmetric.Metrics, error) {
	if f.err != nil {
		return pmetric.Metrics{}, f.err
	}
	return f.metrics, nil
}

// makeMetrics returns pmetric.Metrics with a single int gauge data point.
func makeMetrics() pmetric.Metrics {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	met := sm.Metrics().AppendEmpty()
	met.SetName("test.metric")
	met.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	return m
}

// makeMetricsWithoutDataPoints returns pmetric.Metrics with a metric descriptor but no data points.
func makeMetricsWithoutDataPoints() pmetric.Metrics {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Metrics().AppendEmpty().SetName("empty.metric")
	return m
}

func TestMetricsConsumer_ConsumeEvents(t *testing.T) {
	tests := map[string]struct {
		content          [][]byte
		metadata         map[string]string
		unmarshaler      pmetric.Unmarshaler
		wantErr          string
		wantBatches      int
		wantDataPointCount int
	}{
		"success_single_message": {
			content:         [][]byte{[]byte("m1")},
			metadata:        map[string]string{AttrEventHubName: "single"},
			unmarshaler:     &fakeMetricsUnmarshaler{metrics: makeMetrics()},
			wantBatches:        1,
			wantDataPointCount: 1,
		},
		"success_multiple_messages_merged": {
			content: [][]byte{[]byte("a"), []byte("b")},
			metadata: map[string]string{
				AttrEventHubName:        "hub",
				AttrEventHubPartitionID: "7",
			},
			unmarshaler:        &fakeMetricsUnmarshaler{metrics: makeMetrics()},
			wantBatches:        1,
			wantDataPointCount: 2,
		},
		"metadata_applied_to_resource_attributes": {
			content: [][]byte{[]byte("m")},
			metadata: map[string]string{
				AttrEventHubName:          "myhub",
				AttrEventHubPartitionID:   "1",
				AttrEventHubNamespace:     "test-namespace.servicebus.windows.net",
				AttrEventHubConsumerGroup: "test",
			},
			unmarshaler:        &fakeMetricsUnmarshaler{metrics: makeMetrics()},
			wantBatches:        1,
			wantDataPointCount: 1,
		},
		"unmarshal_error": {
			content:     [][]byte{[]byte("bad")},
			unmarshaler: &fakeMetricsUnmarshaler{err: errors.New("unmarshal failed")},
			wantErr:     "unmarshal message 0",
		},
		"no_metrics_to_consume_empty_content": {
			content:     [][]byte{},
			unmarshaler: &fakeMetricsUnmarshaler{metrics: makeMetrics()},
			wantErr:     "no metrics to consume",
		},
		"no_metrics_to_consume_all_unmarshaled_empty": {
			content:     [][]byte{[]byte("a"), []byte("b")},
			unmarshaler: &fakeMetricsUnmarshaler{metrics: pmetric.NewMetrics()},
			wantErr:     "no metrics to consume",
		},
		"no_metrics_to_consume_metric_without_data_points": {
			content:     [][]byte{[]byte("a")},
			unmarshaler: &fakeMetricsUnmarshaler{metrics: makeMetricsWithoutDataPoints()},
			wantErr:     "no metrics to consume",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sink := new(consumertest.MetricsSink)
			consumer := NewMetricsConsumer(tt.unmarshaler, sink)

			err := consumer.ConsumeEvents(t.Context(), trigger.ParsedRequest{
				Content:  tt.content,
				Metadata: tt.metadata,
			})

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Len(t, sink.AllMetrics(), tt.wantBatches)
				return
			}
			require.NoError(t, err)
			all := sink.AllMetrics()
			require.Len(t, all, tt.wantBatches, "metric batches (ConsumeMetrics calls)")
			if tt.wantBatches > 0 {
				assert.Equal(t, tt.wantDataPointCount, all[0].DataPointCount(), "data points in merged batch")
			}
			if len(tt.metadata) > 0 && tt.wantBatches > 0 {
				md := all[0]
				for ri := 0; ri < md.ResourceMetrics().Len(); ri++ {
					resource := md.ResourceMetrics().At(ri).Resource()
					for key, wantVal := range tt.metadata {
						attr, ok := resource.Attributes().Get(key)
						require.True(t, ok, "resource %d attribute %q", ri, key)
						assert.Equal(t, wantVal, attr.Str(), "resource %d attribute %q", ri, key)
					}
				}
			}
		})
	}
}
