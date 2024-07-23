// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

type testTelemetry struct {
	reader        *metric.ManualReader
	meterProvider *metric.MeterProvider
}

type expectedMetrics struct {
	// processor_filter_metrics_filtered
	metricDataPointsFiltered int64
	// processor_filter_logs_filtered
	logsFiltered int64
	// processor_filter_spans_filtered
	spansFiltered int64
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

func (tt *testTelemetry) NewProcessorCreateSettings() processor.Settings {
	settings := processortest.NewNopSettings()
	settings.MeterProvider = tt.meterProvider
	settings.ID = component.NewID(metadata.Type)

	return settings
}

func (tt *testTelemetry) assertMetrics(t *testing.T, expected expectedMetrics) {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &md))

	if expected.metricDataPointsFiltered > 0 {
		name := "otelcol_processor_filter_datapoints.filtered"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of metric data points dropped by the filter processor",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      expected.metricDataPointsFiltered,
						Attributes: attribute.NewSet(attribute.String("filter", "filter")),
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.logsFiltered > 0 {
		name := "otelcol_processor_filter_logs.filtered"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of logs dropped by the filter processor",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      expected.logsFiltered,
						Attributes: attribute.NewSet(attribute.String("filter", "filter")),
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
	if expected.spansFiltered > 0 {
		name := "otelcol_processor_filter_spans.filtered"
		got := tt.getMetric(name, md)
		want := metricdata.Metrics{
			Name:        name,
			Description: "Number of spans dropped by the filter processor",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Value:      expected.spansFiltered,
						Attributes: attribute.NewSet(attribute.String("filter", "filter")),
					},
				},
			},
		}
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}
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
