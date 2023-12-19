// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	ocprom "contrib.go.opencensus.io/exporter/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

func TestFilterProcessorMetrics(t *testing.T) {
	viewNames := []string{
		"datapoints.filtered",
		"logs.filtered",
		"spans.filtered",
	}
	views := metricViews()
	for i, viewName := range viewNames {
		assert.Equal(t, "processor/filter/"+viewName, views[i].Name)
	}
}

type testTelemetry struct {
	meter         view.Meter
	promHandler   http.Handler
	meterProvider *sdkmetric.MeterProvider
}

type expectedMetrics struct {
	// processor_filter_metrics_filtered
	metricDataPointsFiltered float64
	// processor_filter_logs_filtered
	logsFiltered float64
	// processor_filter_spans_filtered
	spansFiltered float64
}

func telemetryTest(t *testing.T, name string, testFunc func(t *testing.T, tel testTelemetry)) {
	t.Run(name+"WithOC", func(t *testing.T) {
		testFunc(t, setupTelemetry(t))
	})
}

func setupTelemetry(t *testing.T) testTelemetry {
	// Unregister the views first since they are registered by the init, this way we reset them.
	views := metricViews()
	view.Unregister(views...)
	require.NoError(t, view.Register(views...))

	telemetry := testTelemetry{
		meter: view.NewMeter(),
	}

	promReg := prometheus.NewRegistry()

	ocExporter, err := ocprom.NewExporter(ocprom.Options{Registry: promReg})
	require.NoError(t, err)

	telemetry.promHandler = ocExporter

	view.RegisterExporter(ocExporter)
	t.Cleanup(func() { view.UnregisterExporter(ocExporter) })

	return telemetry
}

func (tt *testTelemetry) NewProcessorCreateSettings() processor.CreateSettings {
	settings := processortest.NewNopCreateSettings()
	settings.MeterProvider = tt.meterProvider
	settings.ID = component.NewID(metadata.Type)

	return settings
}

func (tt *testTelemetry) assertMetrics(t *testing.T, expected expectedMetrics) {
	for _, v := range metricViews() {
		// Forces a flush for the opencensus view data.
		_, _ = view.RetrieveData(v.Name)
	}

	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	tt.promHandler.ServeHTTP(rr, req)

	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(rr.Body)
	require.NoError(t, err)

	if expected.metricDataPointsFiltered > 0 {
		name := "processor_filter_datapoints_filtered"
		metric := tt.getMetric(t, name, io_prometheus_client.MetricType_COUNTER, metrics)

		assertFloat(t, expected.metricDataPointsFiltered, metric.GetCounter().GetValue(), name)
	}
	if expected.logsFiltered > 0 {
		name := "processor_filter_logs_filtered"
		metric := tt.getMetric(t, name, io_prometheus_client.MetricType_COUNTER, metrics)

		assertFloat(t, expected.logsFiltered, metric.GetCounter().GetValue(), name)
	}
	if expected.spansFiltered > 0 {
		name := "processor_filter_spans_filtered"
		metric := tt.getMetric(t, name, io_prometheus_client.MetricType_COUNTER, metrics)

		assertFloat(t, expected.spansFiltered, metric.GetCounter().GetValue(), name)
	}
}

func (tt *testTelemetry) getMetric(t *testing.T, name string, mtype io_prometheus_client.MetricType, got map[string]*io_prometheus_client.MetricFamily) *io_prometheus_client.Metric {
	metricFamily, ok := got[name]
	require.True(t, ok, "expected metric '%s' not found", name)
	require.Equal(t, mtype, metricFamily.GetType())

	metric, err := getSingleMetric(metricFamily)
	require.NoError(t, err)

	return metric
}

func getSingleMetric(metric *io_prometheus_client.MetricFamily) (*io_prometheus_client.Metric, error) {
	if l := len(metric.Metric); l != 1 {
		return nil, fmt.Errorf("expected metric '%s' with one set of attributes, but found %d", metric.GetName(), l)
	}
	first := metric.Metric[0]

	if len(first.Label) != 1 || "filter" != first.Label[0].GetName() {
		return nil, fmt.Errorf("expected metric '%s' with a single `filter=\"\"` attribute but got '%s'", metric.GetName(), first.GetLabel())
	}

	return first, nil
}

func assertFloat(t *testing.T, expected, got float64, metric string) {
	if math.Abs(expected-got) > 0.00001 {
		assert.Failf(t, "unexpected metric value", "value for metric '%s' did not match, expected '%f' got '%f'", metric, expected, got)
	}
}
