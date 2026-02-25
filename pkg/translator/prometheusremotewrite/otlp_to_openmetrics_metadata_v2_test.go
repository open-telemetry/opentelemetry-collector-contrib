// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"
import (
	"testing"
	"time"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func TestOtelMetricTypeToPromMetricTypeV2(t *testing.T) {
	ts := uint64(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   writev2.Metadata_MetricType
	}{
		{
			name: "gauge",
			metric: func() pmetric.Metric {
				return getIntGaugeMetric(
					"test",
					pcommon.NewMap(),
					1, ts,
				)
			},
			want: writev2.Metadata_METRIC_TYPE_GAUGE,
		},
		{
			name: "sum",
			metric: func() pmetric.Metric {
				return getIntSumMetric(
					"test",
					pcommon.NewMap(),
					pmetric.AggregationTemporalityCumulative,
					1, ts,
				)
			},
			want: writev2.Metadata_METRIC_TYPE_GAUGE,
		},
		{
			name: "monotonic cumulative",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_sum")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetEmptySum().SetIsMonotonic(true)

				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(1)

				return metric
			},
			want: writev2.Metadata_METRIC_TYPE_COUNTER,
		},
		{
			name: "cumulative histogram",
			metric: func() pmetric.Metric {
				m := getHistogramMetric("", pcommon.NewMap(), pmetric.AggregationTemporalityCumulative, 0, 0, 0, []float64{}, []uint64{})
				return m
			},
			want: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
		},
		{
			name: "cumulative exponential histogram",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				h := metric.SetEmptyExponentialHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				return metric
			},
			want: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
		},
		{
			name: "summary with start time",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_summary")
				metric.SetEmptySummary()

				dp := metric.Summary().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(ts))
				dp.SetStartTimestamp(pcommon.Timestamp(ts))

				return metric
			},
			want: writev2.Metadata_METRIC_TYPE_SUMMARY,
		},
		{
			name: "unknown from metadata",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_sum")
				metric.Metadata().PutStr(prometheustranslator.MetricMetadataTypeKey, "unknown")

				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(1)

				return metric
			},
			want: writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		},
		{
			name: "info from metadata",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_sum")
				metric.Metadata().PutStr(prometheustranslator.MetricMetadataTypeKey, "info")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetEmptySum().SetIsMonotonic(false)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(1)

				return metric
			},
			want: writev2.Metadata_METRIC_TYPE_INFO,
		},
		{
			name: "stateset from metadata",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_sum")
				metric.Metadata().PutStr(prometheustranslator.MetricMetadataTypeKey, "stateset")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetEmptySum().SetIsMonotonic(false)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(1)

				return metric
			},
			want: writev2.Metadata_METRIC_TYPE_STATESET,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()

			metricType := otelMetricTypeToPromMetricTypeV2(metric)

			assert.Equal(t, tt.want, metricType)
		})
	}
}
