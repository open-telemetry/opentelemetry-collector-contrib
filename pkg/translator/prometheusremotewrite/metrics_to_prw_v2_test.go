// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"fmt"
	"testing"
	"time"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"

	"github.com/stretchr/testify/require"
)

func TestFromMetricsV2(t *testing.T) {
	settings := Settings{
		Namespace:           "",
		ExternalLabels:      nil,
		DisableTargetInfo:   false,
		ExportCreatedMetric: false,
		AddMetricSuffixes:   false,
		SendMetadata:        false,
	}

	ts := uint64(time.Now().UnixNano())
	payload := createExportRequestWithTimestamp(5, 0, 1, 3, 0, pcommon.Timestamp(ts))
	want := func() map[string]*writev2.TimeSeries {
		// TODO consider translating labels back using symbols table and checking them
		/*labels := labels.Labels{
			labels.Label{
				Name:  labels.MetricName,
				Value: "test",
			},
		}*/
		return map[string]*writev2.TimeSeries{
			"0": {
				LabelsRefs: []uint32{1, 2},
				Samples: []writev2.Sample{
					{Timestamp: convertTimeStamp(pcommon.Timestamp(ts)), Value: 1.23},
				},
			},
		}
	}
	tsMap, err := FromMetricsV2(payload.Metrics(), settings)
	wanted := want()
	require.NoError(t, err)
	require.NotNil(t, tsMap)
	require.Equal(t, wanted, tsMap)
}

func createExportRequestWithTimestamp(resourceAttributeCount int, histogramCount int, nonHistogramCount int, labelsPerMetric int, exemplarsPerSeries int, timestamp pcommon.Timestamp) pmetricotlp.ExportRequest {
	request := pmetricotlp.NewExportRequest()

	rm := request.Metrics().ResourceMetrics().AppendEmpty()
	generateAttributes(rm.Resource().Attributes(), "resource", resourceAttributeCount)

	metrics := rm.ScopeMetrics().AppendEmpty().Metrics()

	for i := 1; i <= histogramCount; i++ {
		m := metrics.AppendEmpty()
		m.SetEmptyHistogram()
		m.SetName(fmt.Sprintf("histogram-%v", i))
		m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		h := m.Histogram().DataPoints().AppendEmpty()
		h.SetTimestamp(timestamp)

		// Set 50 samples, 10 each with values 0.5, 1, 2, 4, and 8
		h.SetCount(50)
		h.SetSum(155)
		h.BucketCounts().FromRaw([]uint64{10, 10, 10, 10, 10, 0})
		h.ExplicitBounds().FromRaw([]float64{.5, 1, 2, 4, 8, 16}) // Bucket boundaries include the upper limit (ie. each sample is on the upper limit of its bucket)

		generateAttributes(h.Attributes(), "series", labelsPerMetric)
		generateExemplars(h.Exemplars(), exemplarsPerSeries, timestamp)
	}

	for i := 1; i <= nonHistogramCount; i++ {
		m := metrics.AppendEmpty()
		m.SetEmptySum()
		m.SetName(fmt.Sprintf("sum-%v", i))
		m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		point := m.Sum().DataPoints().AppendEmpty()
		point.SetTimestamp(timestamp)
		point.SetDoubleValue(1.23)
		generateAttributes(point.Attributes(), "series", labelsPerMetric)
		generateExemplars(point.Exemplars(), exemplarsPerSeries, timestamp)
	}

	for i := 1; i <= nonHistogramCount; i++ {
		m := metrics.AppendEmpty()
		m.SetEmptyGauge()
		m.SetName(fmt.Sprintf("gauge-%v", i))
		point := m.Gauge().DataPoints().AppendEmpty()
		point.SetTimestamp(timestamp)
		point.SetDoubleValue(1.23)
		generateAttributes(point.Attributes(), "series", labelsPerMetric)
		generateExemplars(point.Exemplars(), exemplarsPerSeries, timestamp)
	}

	return request
}
