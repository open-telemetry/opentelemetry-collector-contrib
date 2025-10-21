package starttimeattribute

import (
	"context"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/filter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type mockPodClient struct {
	startTimes map[string]time.Time
}

func (m *mockPodClient) GetPodStartTime(ctx context.Context, podID podIdentifier) time.Time {
	key := podID.Value
	if t, ok := m.startTimes[key]; ok {
		return t
	}
	return time.Time{}
}

const testMetricName = "test_metric"

func TestAdjustMetrics(t *testing.T) {
	testStartTime := time.Now().Add(-1 * time.Hour)

	mockClient := &mockPodClient{
		startTimes: map[string]time.Time{
			"10.0.0.1":       testStartTime,
			"default/my-pod": testStartTime,
			"uid-12345":      testStartTime,
		},
	}

	tests := []struct {
		name             string
		attrs            map[string]string
		expectAdjusted   bool
		isCumulative     bool
		startTimeUnixSec int64
		filter           filterset.FilterSet
	}{
		{
			name: "cumulative metric with pod IP",
			attrs: map[string]string{
				"k8s.pod.ip": "10.0.0.1",
			},
			expectAdjusted: true,
			isCumulative:   true,
		},
		{
			name: "cumulative metric with pod name",
			attrs: map[string]string{
				"k8s.pod.name":       "my-pod",
				"k8s.namespace.name": "default",
			},
			expectAdjusted: true,
			isCumulative:   true,
		},
		{
			name: "cumulative metric with pod UID",
			attrs: map[string]string{
				"k8s.pod.uid": "uid-12345",
			},
			expectAdjusted: true,
			isCumulative:   true,
		},
		{
			name:           "delta metric should not be adjusted",
			attrs:          map[string]string{"k8s.pod.ip": "10.0.0.1"},
			expectAdjusted: false,
			isCumulative:   false,
		},
		{
			name:           "metric without pod identifier",
			attrs:          map[string]string{"other": "value"},
			expectAdjusted: false,
			isCumulative:   true,
		},
		{
			name: "metric excluded by filter",
			attrs: map[string]string{
				"k8s.pod.ip": "10.0.0.1",
			},
			expectAdjusted: false,
			isCumulative:   true,
			filter:         filter.NoOpFilter{NoMatch: true},
		},
		{
			name: "metric excluded because it has a non-zero start time",
			attrs: map[string]string{
				"k8s.pod.ip": "10.0.0.1",
			},
			expectAdjusted:   false,
			isCumulative:     true,
			startTimeUnixSec: time.Now().Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := createTestMetrics(testMetricName, tt.attrs, tt.isCumulative, tt.startTimeUnixSec)
			originalStartTime := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).StartTimestamp()
			metricNameFilter := tt.filter
			if metricNameFilter == nil {
				metricNameFilter = filter.NoOpFilter{}
			}
			adjuster, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
				func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter) (podClient, error) {
					return mockClient, nil
				}, metricNameFilter, AttributesFilterConfig{}, true, 5*time.Minute)
			require.NoError(t, err)
			adjustedMetrics, err := adjuster.AdjustMetrics(context.Background(), metrics)
			require.NoError(t, err)

			adjustedStartTime := adjustedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).StartTimestamp()

			if tt.expectAdjusted {
				expectedTime := pcommon.NewTimestampFromTime(testStartTime)
				assert.Equal(t, expectedTime, adjustedStartTime)
				assert.NotEqual(t, originalStartTime, adjustedStartTime)
			} else {
				assert.Equal(t, originalStartTime, adjustedStartTime)
			}
		})
	}
}

func createTestMetrics(name string, attrs map[string]string, cumulative bool, startTime int64) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	resource := rm.Resource()
	for k, v := range attrs {
		resource.Attributes().PutStr(k, v)
	}

	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName(name)

	sum := m.SetEmptySum()
	if cumulative {
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	} else {
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	}

	dp := sum.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(startTime, 0)))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(100.0)

	return metrics
}

func TestExtractPodIdentifier(t *testing.T) {
	adjuster := &Adjuster{}

	tests := []struct {
		name     string
		attrs    map[string]string
		expected *podIdentifier
	}{
		{
			name:     "pod IP",
			attrs:    map[string]string{"k8s.pod.ip": "10.0.0.1"},
			expected: &podIdentifier{Value: "10.0.0.1", Type: podIP},
		},
		{
			name: "pod name with namespace",
			attrs: map[string]string{
				"k8s.pod.name":       "my-pod",
				"k8s.namespace.name": "default",
			},
			expected: &podIdentifier{Value: "default/my-pod", Type: podName},
		},
		{
			name:     "pod UID",
			attrs:    map[string]string{"k8s.pod.uid": "uid-12345"},
			expected: &podIdentifier{Value: "uid-12345", Type: podUID},
		},
		{
			name:     "no pod identifier",
			attrs:    map[string]string{"other": "value"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			for k, v := range tt.attrs {
				attrs.PutStr(k, v)
			}

			result := adjuster.extractPodIdentifier(attrs)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Value, result.Value)
				assert.Equal(t, tt.expected.Type, result.Type)
			}
		})
	}
}

func TestIsCumulativeMetric(t *testing.T) {
	adjuster := &Adjuster{}

	tests := []struct {
		name         string
		metricType   pmetric.MetricType
		temporality  pmetric.AggregationTemporality
		isCumulative bool
	}{
		{
			name:         "cumulative sum",
			metricType:   pmetric.MetricTypeSum,
			temporality:  pmetric.AggregationTemporalityCumulative,
			isCumulative: true,
		},
		{
			name:         "delta sum",
			metricType:   pmetric.MetricTypeSum,
			temporality:  pmetric.AggregationTemporalityDelta,
			isCumulative: false,
		},
		{
			name:         "cumulative histogram",
			metricType:   pmetric.MetricTypeHistogram,
			temporality:  pmetric.AggregationTemporalityCumulative,
			isCumulative: true,
		},
		{
			name:         "gauge is not cumulative",
			metricType:   pmetric.MetricTypeGauge,
			temporality:  pmetric.AggregationTemporalityUnspecified,
			isCumulative: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()

			switch tt.metricType {
			case pmetric.MetricTypeSum:
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(tt.temporality)
			case pmetric.MetricTypeHistogram:
				hist := metric.SetEmptyHistogram()
				hist.SetAggregationTemporality(tt.temporality)
			case pmetric.MetricTypeGauge:
				metric.SetEmptyGauge()
			}

			result := adjuster.isCumulativeMetric(metric)
			assert.Equal(t, tt.isCumulative, result)
		})
	}
}
