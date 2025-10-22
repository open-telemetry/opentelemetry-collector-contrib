package starttimeattribute

import (
	"context"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/testhelper"
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
	testStartTimestamp := pcommon.NewTimestampFromTime(testStartTime)
	currentTime := time.Now()
	currentTimestamp := pcommon.NewTimestampFromTime(currentTime)
	nonZeroStartTime := time.Now().Add(-30 * time.Minute)
	nonZeroStartTimestamp := pcommon.NewTimestampFromTime(nonZeroStartTime)

	mockClient := &mockPodClient{
		startTimes: map[string]time.Time{
			"10.0.0.1":       testStartTime,
			"default/my-pod": testStartTime,
			"uid-12345":      testStartTime,
		},
	}

	// Test case 1: cumulative metric with pod IP

	script1 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Cumulative metric with pod IP - start time should be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(testStartTimestamp)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	adjuster1, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filter.NoOpFilter{},
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster1, script1)

	// Test case 2: cumulative metric with pod name and namespace
	script2 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Cumulative metric with pod name and namespace - start time should be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.name", "my-pod")
				rm.Resource().Attributes().PutStr("k8s.namespace.name", "default")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.name", "my-pod")
				rm.Resource().Attributes().PutStr("k8s.namespace.name", "default")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(testStartTimestamp)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	adjuster2, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filter.NoOpFilter{},
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster2, script2)

	// Test case 3: cumulative metric with pod UID
	script3 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Cumulative metric with pod UID - start time should be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.uid", "uid-12345")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.uid", "uid-12345")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(testStartTimestamp)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	adjuster3, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filter.NoOpFilter{},
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster3, script3)

	// Test case 4: delta metric should not be adjusted
	script4 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Delta metric with pod IP - start time should NOT be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	adjuster4, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filter.NoOpFilter{},
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster4, script4)

	// Test case 5: metric without pod identifier
	script5 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Metric without pod identifier - start time should NOT be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("other", "value")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("other", "value")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	adjuster5, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filter.NoOpFilter{},
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster5, script5)

	// Test case 6: metric excluded by NoOp filter
	script6 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Metric excluded by NoOp filter - start time should NOT be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	adjuster6, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filter.NoOpFilter{NoMatch: true},
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster6, script6)

	// Test case 7: metric included by filter
	script7 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Metric included by filter - start time should be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(testStartTimestamp)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	filterSet7, err := filter.NewFilter(filter.FilterConfig{
		Metrics: []string{testMetricName},
		Config: filterset.Config{
			MatchType: filterset.Strict,
		},
	}, filter.FilterConfig{})
	require.NoError(t, err)

	adjuster7, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filterSet7,
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster7, script7)

	// Test case 8: metric excluded by filter
	script8 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Metric excluded by filter - start time should NOT be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(0)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	filterSet8, err := filter.NewFilter(filter.FilterConfig{},
		filter.FilterConfig{
			Metrics: []string{testMetricName},
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
		})
	require.NoError(t, err)

	adjuster8, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter: filterSet8,
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster8, script8)

	// Test case 9: metric with non-zero start time should not be adjusted
	script9 := []*testhelper.MetricsAdjusterTest{
		{
			Description: "Metric with non-zero start time - start time should NOT be adjusted",
			Metrics: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(nonZeroStartTimestamp)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
			Adjusted: func() pmetric.Metrics {
				m := pmetric.NewMetrics()
				rm := m.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("k8s.pod.ip", "10.0.0.1")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName(testMetricName)
				sum := metric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := sum.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(nonZeroStartTimestamp)
				dp.SetTimestamp(currentTimestamp)
				dp.SetDoubleValue(100.0)
				return m
			}(),
		},
	}

	adjuster9, err := NewAdjusterWithFactory(component.TelemetrySettings{Logger: componenttest.NewNopTelemetrySettings().Logger},
		func(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
			return mockClient, nil
		}, AttributesFilterConfig{}, false, common.AdjustmentOptions{
			Filter:         filter.NoOpFilter{},
			SkipIfCTExists: true,
		})
	require.NoError(t, err)
	testhelper.RunScript(t, adjuster9, script9)
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
