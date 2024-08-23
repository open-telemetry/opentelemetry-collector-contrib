// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
)

type testTelemetry struct {
	reader        *metric.ManualReader
	meterProvider *metric.MeterProvider
}

type expectedMetrics struct {
	mNumGroupedSpans    int64
	mNumNonGroupedSpans int64
	mDistSpanGroups     int64

	mNumGroupedLogs    int64
	mNumNonGroupedLogs int64
	mDistLogGroups     int64

	mNumGroupedMetrics    int64
	mNumNonGroupedMetrics int64
	mDistMetricGroups     int64
}

func setupTelemetry() testTelemetry {
	reader := metric.NewManualReader()
	return testTelemetry{
		reader:        reader,
		meterProvider: metric.NewMeterProvider(metric.WithReader(reader)),
	}
}

func (tt *testTelemetry) assertMetrics(t *testing.T, expected expectedMetrics) {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &md))
	if expected.mNumGroupedLogs > 0 {
		name := "otelcol_processor_groupbyattrs_num_grouped_logs"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of logs that had attributes grouped",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: expected.mNumGroupedLogs,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.mNumGroupedMetrics > 0 {
		name := "otelcol_processor_groupbyattrs_num_grouped_metrics"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of metrics that had attributes grouped",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: expected.mNumGroupedMetrics,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.mNumGroupedSpans > 0 {
		name := "otelcol_processor_groupbyattrs_num_grouped_spans"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of spans that had attributes grouped",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: expected.mNumGroupedSpans,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.mNumNonGroupedLogs > 0 {
		name := "otelcol_processor_groupbyattrs_num_non_grouped_logs"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of logs that did not have attributes grouped",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: expected.mNumNonGroupedLogs,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.mNumNonGroupedMetrics > 0 {
		name := "otelcol_processor_groupbyattrs_num_non_grouped_metrics"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of metrics that did not have attributes grouped",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: expected.mNumNonGroupedMetrics,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.mNumNonGroupedSpans > 0 {
		name := "otelcol_processor_groupbyattrs_num_non_grouped_spans"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of spans that did not have attributes grouped",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value: expected.mNumNonGroupedSpans,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.mDistLogGroups > 0 {
		name := "otelcol_processor_groupbyattrs_log_groups"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Distribution of groups extracted for logs",
			Unit:        "1",
			Data: metricdata.Histogram[int64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        1,
						Min:          metricdata.NewExtrema(expected.mDistLogGroups),
						Max:          metricdata.NewExtrema(expected.mDistLogGroups),
						Sum:          expected.mDistLogGroups,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
	}
	if expected.mDistMetricGroups > 0 {
		name := "otelcol_processor_groupbyattrs_metric_groups"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Distribution of groups extracted for metrics",
			Unit:        "1",
			Data: metricdata.Histogram[int64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        2,
						Min:          metricdata.NewExtrema(expected.mDistMetricGroups),
						Max:          metricdata.NewExtrema(expected.mDistMetricGroups),
						Sum:          2 * expected.mDistMetricGroups,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
	}
	if expected.mDistSpanGroups > 0 {
		name := "otelcol_processor_groupbyattrs_span_groups"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Distribution of groups extracted for spans",
			Unit:        "1",
			Data: metricdata.Histogram[int64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[int64]{
					{
						Attributes:   *attribute.EmptySet(),
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        1,
						Min:          metricdata.NewExtrema(expected.mDistSpanGroups),
						Max:          metricdata.NewExtrema(expected.mDistSpanGroups),
						Sum:          expected.mDistSpanGroups,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
	}
}

func (tt *testTelemetry) NewProcessorCreateSettings() processor.Settings {
	settings := processortest.NewNopSettings()
	settings.MeterProvider = tt.meterProvider
	settings.ID = component.NewID(metadata.Type)

	return settings
}

func (tt *testTelemetry) getMetric(name string, got metricdata.ResourceMetrics) metricdata.Metrics {
	for _, sm := range got.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	return metricdata.Metrics{}
}
