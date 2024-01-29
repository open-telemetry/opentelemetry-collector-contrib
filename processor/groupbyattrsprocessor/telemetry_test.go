// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
	"github.com/stretchr/testify/require"
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

func telemetryTest(t *testing.T, name string, testFunc func(t *testing.T, tel testTelemetry)) {
	t.Run(name, func(t *testing.T) {
		testFunc(t, setupTelemetry())
	})
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
		name := "processor/groupbyattrs/num_grouped_logs"
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
	if expected.mNumNonGroupedLogs > 0 {
		name := "processor/groupbyattrs/num_non_grouped_logs"
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
	if expected.mDistLogGroups > 0 {
		name := "processor/groupbyattrs/log_groups"
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
						BucketCounts: []uint64{0, uint64(expected.mDistLogGroups), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Count:        1,
						Min:          metricdata.NewExtrema(int64(0)),
						Max:          metricdata.NewExtrema(expected.mDistLogGroups),
						Sum:          expected.mDistLogGroups,
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
	}
}

func (tt *testTelemetry) NewProcessorCreateSettings() processor.CreateSettings {
	settings := processortest.NewNopCreateSettings()
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
