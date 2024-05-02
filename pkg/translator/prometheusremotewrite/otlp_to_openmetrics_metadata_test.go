// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"
import (
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func TestOtelMetricTypeToPromMetricType(t *testing.T) {
	ts := uint64(time.Now().UnixNano())
	tests := []struct {
		name   string
		metric func() pmetric.Metric
		want   prompb.MetricMetadata_MetricType
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
			want: prompb.MetricMetadata_GAUGE,
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
			want: prompb.MetricMetadata_GAUGE,
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
			want: prompb.MetricMetadata_COUNTER,
		},
		{
			name: "cumulative histogram",
			metric: func() pmetric.Metric {
				m := getHistogramMetric("", pcommon.NewMap(), pmetric.AggregationTemporalityCumulative, 0, 0, 0, []float64{}, []uint64{})
				return m
			},
			want: prompb.MetricMetadata_HISTOGRAM,
		},
		{
			name: "cumulative exponential histogram",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				h := metric.SetEmptyExponentialHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				return metric
			},
			want: prompb.MetricMetadata_HISTOGRAM,
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
			want: prompb.MetricMetadata_SUMMARY,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()

			metricType := otelMetricTypeToPromMetricType(metric)

			assert.Equal(t, tt.want, metricType)
		})
	}
}

func TestOtelMetricsToMetadata(t *testing.T) {
	ts := uint64(time.Now().UnixNano())
	tests := []struct {
		name    string
		metrics pmetric.Metrics
		want    []*prompb.MetricMetadata
	}{
		{
			name:    "all types§",
			metrics: GenerateMetricsAllTypesNoDataPointsHelp(),
			want: []*prompb.MetricMetadata{
				{
					Type: prompb.MetricMetadata_GAUGE,
					MetricFamilyName: prometheustranslator.BuildCompliantName(getIntGaugeMetric(
						testdata.TestGaugeDoubleMetricName,
						pcommon.NewMap(),
						1, ts,
					), "", false),
					Help: "gauge description",
				},
				{
					Type: prompb.MetricMetadata_GAUGE,
					MetricFamilyName: prometheustranslator.BuildCompliantName(getIntGaugeMetric(
						testdata.TestGaugeIntMetricName,
						pcommon.NewMap(),
						1, ts,
					), "", false),
					Help: "gauge description",
				},
				{
					Type: prompb.MetricMetadata_COUNTER,
					MetricFamilyName: prometheustranslator.BuildCompliantName(getIntGaugeMetric(
						testdata.TestSumDoubleMetricName,
						pcommon.NewMap(),
						1, ts,
					), "", false),
					Help: "sum description",
				},
				{
					Type: prompb.MetricMetadata_COUNTER,
					MetricFamilyName: prometheustranslator.BuildCompliantName(getIntGaugeMetric(
						testdata.TestSumIntMetricName,
						pcommon.NewMap(),
						1, ts,
					), "", false),
					Help: "sum description",
				},
				{
					Type: prompb.MetricMetadata_HISTOGRAM,
					MetricFamilyName: prometheustranslator.BuildCompliantName(getIntGaugeMetric(
						testdata.TestDoubleHistogramMetricName,
						pcommon.NewMap(),
						1, ts,
					), "", false),
					Help: "histogram description",
				},
				{
					Type: prompb.MetricMetadata_SUMMARY,
					MetricFamilyName: prometheustranslator.BuildCompliantName(getIntGaugeMetric(
						testdata.TestDoubleSummaryMetricName,
						pcommon.NewMap(),
						1, ts,
					), "", false),
					Help: "summary description",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaData := OtelMetricsToMetadata(tt.metrics, false)

			for i := 0; i < len(metaData); i++ {
				assert.Equal(t, tt.want[i].Type, metaData[i].Type)
				assert.Equal(t, tt.want[i].MetricFamilyName, metaData[i].MetricFamilyName)
				assert.Equal(t, tt.want[i].Help, metaData[i].Help)
			}

		})
	}
}

func GenerateMetricsAllTypesNoDataPointsHelp() pmetric.Metrics {
	md := testdata.GenerateMetricsOneEmptyInstrumentationLibrary()
	ilm0 := md.ResourceMetrics().At(0).ScopeMetrics().At(0)
	ms := ilm0.Metrics()
	initMetric(ms.AppendEmpty(), testdata.TestGaugeDoubleMetricName, pmetric.MetricTypeGauge, "gauge description")
	initMetric(ms.AppendEmpty(), testdata.TestGaugeIntMetricName, pmetric.MetricTypeGauge, "gauge description")
	initMetric(ms.AppendEmpty(), testdata.TestSumDoubleMetricName, pmetric.MetricTypeSum, "sum description")
	initMetric(ms.AppendEmpty(), testdata.TestSumIntMetricName, pmetric.MetricTypeSum, "sum description")
	initMetric(ms.AppendEmpty(), testdata.TestDoubleHistogramMetricName, pmetric.MetricTypeHistogram, "histogram description")
	initMetric(ms.AppendEmpty(), testdata.TestDoubleSummaryMetricName, pmetric.MetricTypeSummary, "summary description")
	return md
}

func initMetric(m pmetric.Metric, name string, ty pmetric.MetricType, desc string) {
	m.SetName(name)
	m.SetDescription(desc)
	//exhaustive:enforce
	switch ty {
	case pmetric.MetricTypeGauge:
		m.SetEmptyGauge()

	case pmetric.MetricTypeSum:
		sum := m.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	case pmetric.MetricTypeHistogram:
		histo := m.SetEmptyHistogram()
		histo.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	case pmetric.MetricTypeExponentialHistogram:
		m.SetEmptyExponentialHistogram()

	case pmetric.MetricTypeSummary:
		m.SetEmptySummary()
	}
}
