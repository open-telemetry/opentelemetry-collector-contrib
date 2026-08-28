package dorisexporter

// Regression tests for #50569: the Doris exporter must not fail the whole
// export batch when a metric data point carries a non-finite float value
// (NaN, +Inf, -Inf), because encoding/json cannot serialize such values.

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func testMetricsExporter() *metricsExporter {
	return &metricsExporter{
		commonExporter: &commonExporter{
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
			logger:            zap.NewNop(),
			timeZone:          time.UTC,
		},
	}
}

// buildGaugeMetrics returns a pmetric.Metrics containing one Gauge metric
// with three data points whose values are NaN, +Inf and 42 respectively.
func buildGaugeMetricsWithNonFinite() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test_metric")
	m.SetDescription("desc")
	m.SetUnit("1")
	gauge := m.SetEmptyGauge()

	dpNaN := gauge.DataPoints().AppendEmpty()
	dpNaN.SetDoubleValue(math.NaN())

	dpInf := gauge.DataPoints().AppendEmpty()
	dpInf.SetDoubleValue(math.Inf(1))

	dpValid := gauge.DataPoints().AppendEmpty()
	dpValid.SetDoubleValue(42.0)
	return md
}

func TestGaugeModelDropsNonFiniteDataPoints(t *testing.T) {
	e := testMetricsExporter()
	model := &metricModelGauge{}
	dm := &dMetric{ServiceName: "svc", MetricName: "test_metric"}

	pm := buildGaugeMetricsWithNonFinite().ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.NoError(t, model.add(pm, dm, e))

	require.Equal(t, 1, model.size(), "only the finite data point should be kept")
	require.Equal(t, 42.0, model.data[0].Value)

	// serialization must succeed and contain exactly the valid data point
	out, err := model.bytes()
	require.NoError(t, err, "bytes() must not fail when non-finite data points are dropped")
	body := string(out)
	assert.True(t, strings.Contains(body, "\"value\":42"))
	assert.False(t, strings.Contains(body, "NaN"), "NaN must not appear in output")
}

func TestToJSONLinesSkipsNonFiniteWhenAlreadyInBatch(t *testing.T) {
	// Defense in depth: even if a non-finite value slips into the internal
	// structures, serialization must not fail the whole batch.
	data := []*dMetricGauge{
		{Value: 1.0},
		{Value: math.NaN()},
		{Value: math.Inf(-1)},
		{Value: 2.0},
	}
	out, err := toJSONLines(data)
	require.NoError(t, err, "toJSONLines must not fail on non-finite values")
	body := string(out)
	assert.True(t, strings.Contains(body, "\"value\":1"))
	assert.True(t, strings.Contains(body, "\"value\":2"))
}
