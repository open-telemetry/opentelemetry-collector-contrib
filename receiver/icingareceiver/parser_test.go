// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestAggregateEmpty(t *testing.T) {
	parser := icingaParser{
		ctx:    context.Background(),
		logger: zap.NewNop(),
		config: Config{
			Histograms: []HistogramConfig{},
		},
	}
	parser.initialize()

	parser.Aggregate("")

	metrics := parser.getMetrics()
	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	require.Equal(t, 2, metrics.ResourceMetrics().At(0).ScopeMetrics().Len())

	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, "icinga.gauge.host.state", metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().Len())
	require.Equal(t, 0., metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).DoubleVal())

	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().At(1).Metrics().Len())
	require.Equal(t, "icinga.counter.host.check_result", metrics.ResourceMetrics().At(0).ScopeMetrics().At(1).Metrics().At(0).Name())
	require.Equal(t, 1, metrics.ResourceMetrics().At(0).ScopeMetrics().At(1).Metrics().At(0).Sum().DataPoints().Len())
	require.Equal(t, 0., metrics.ResourceMetrics().At(0).ScopeMetrics().At(1).Metrics().At(0).Sum().DataPoints().At(0).DoubleVal())
}

func TestAggregateService(t *testing.T) {
	parser := icingaParser{
		ctx:    context.Background(),
		logger: zap.NewNop(),
		config: Config{},
	}
	parser.initialize()

	event := `
	{
		"check_result": {
		  "active": true,
		  "check_source": "foo.host.local",
		  "command": ["/bin/icingacli", "businessprocess", "process", "check"],
		  "execution_end": 1654687722.7553989887,
		  "execution_start": 1654687722.6818370819,
		  "exit_status": 0.0,
		  "output": "OK",
		  "performance_data": ["time=0.024881s;;;0.000000"],
		  "schedule_end": 1654687722.7554409504,
		  "schedule_start": 1654687722.6800000668,
		  "state": 2.0,
		  "type": "CheckResult"
		},
		"host": "foobar",
		"service": "check_test",
		"timestamp": 1654687722.7575340271,
		"type": "CheckResult"
	  }`

	parser.Aggregate(event)
	scopeMetrics := parser.getMetrics().ResourceMetrics().At(0).ScopeMetrics().Sort(sortMetrics)

	require.Equal(t, 3, scopeMetrics.Len())

	requireGaugeMetric(t, scopeMetrics.At(0), "icinga.gauge.service.state", 2., "")
	requireGaugeMetric(t, scopeMetrics.At(1), "icinga.gauge.service.perf.millis", 24.881, "ms")
	requireSumMetric(t, scopeMetrics.At(2), "icinga.counter.service.check_result", int64(1), "")

	parser.Aggregate(event)
	scopeMetrics = parser.getMetrics().ResourceMetrics().At(0).ScopeMetrics().Sort(sortMetrics)
	require.Equal(t, 3, scopeMetrics.Len())

	requireSumMetric(t, scopeMetrics.At(2), "icinga.counter.service.check_result", int64(2), "")
}
func TestAggregateHistogram(t *testing.T) {
	parser := icingaParser{
		ctx:    context.Background(),
		logger: zap.NewNop(),
		config: Config{
			Histograms: []HistogramConfig{
				{Service: "check_test", Host: "foobar", Type: "time", Values: []float64{10., 50.}},
			},
		},
	}
	parser.initialize()

	event := `
	{
		"check_result": {
		  "active": true,
		  "check_source": "foo.host.local",
		  "command": ["/bin/icingacli", "businessprocess", "process", "check"],
		  "execution_end": 1654687722.7553989887,
		  "execution_start": 1654687722.6818370819,
		  "exit_status": 0.0,
		  "output": "OK",
		  "performance_data": ["time=0.024881s;;;0.000000"],
		  "schedule_end": 1654687722.7554409504,
		  "schedule_start": 1654687722.6800000668,
		  "state": 2.0,
		  "type": "CheckResult"
		},
		"host": "foobar",
		"service": "check_test",
		"timestamp": 1654687722.7575340271,
		"type": "CheckResult"
	  }`

	parser.Aggregate(event)
	scopeMetrics := parser.getMetrics().ResourceMetrics().At(0).ScopeMetrics().Sort(sortMetrics)

	requireHistogramMetric(t, scopeMetrics.At(0), "icinga.histogram.service.perf.millis", int64(1), "ms", "+Inf")
	requireHistogramMetric(t, scopeMetrics.At(1), "icinga.histogram.service.perf.millis", int64(0), "ms", "10.000000")
	requireHistogramMetric(t, scopeMetrics.At(2), "icinga.histogram.service.perf.millis", int64(1), "ms", "50.000000")
}

func TestNormalizeUnit(t *testing.T) {
	value, unit, displayName := normalizeUnit(.1, "foobar")
	require.Equal(t, .1, value)
	require.Equal(t, "", unit)
	require.Equal(t, "current", displayName)

	value, unit, displayName = normalizeUnit(.1, "")
	require.Equal(t, .1, value)
	require.Equal(t, "", unit)
	require.Equal(t, "current", displayName)

	value, unit, displayName = normalizeUnit(.1, "-")
	require.Equal(t, .1, value)
	require.Equal(t, "", unit)
	require.Equal(t, "current", displayName)

	value, unit, displayName = normalizeUnit(.1, "%")
	require.Equal(t, .1, value)
	require.Equal(t, "%", unit)
	require.Equal(t, "percent", displayName)

	value, unit, displayName = normalizeUnit(.1, "Synced")
	require.Equal(t, .1, value)
	require.Equal(t, "", unit)
	require.Equal(t, "synced", displayName)

	value, unit, displayName = normalizeUnit(.1, "ms")
	require.Equal(t, .1, value)
	require.Equal(t, "ms", unit)
	require.Equal(t, "millis", displayName)

	value, unit, displayName = normalizeUnit(.1, "s")
	require.Equal(t, 100., value)
	require.Equal(t, "ms", unit)
	require.Equal(t, "millis", displayName)

	value, unit, displayName = normalizeUnit(.1, "us")
	require.Equal(t, .0001, value)
	require.Equal(t, "ms", unit)
	require.Equal(t, "millis", displayName)

	value, unit, displayName = normalizeUnit(.1, "ns")
	require.Equal(t, 1.0000000000000001e-07, value)
	require.Equal(t, "ms", unit)
	require.Equal(t, "millis", displayName)

	value, unit, displayName = normalizeUnit(10, "B")
	require.Equal(t, 10., value)
	require.Equal(t, "B", unit)
	require.Equal(t, "bytes", displayName)

	value, unit, displayName = normalizeUnit(10, "KB")
	require.Equal(t, 10.*1024, value)
	require.Equal(t, "B", unit)
	require.Equal(t, "bytes", displayName)

	value, unit, displayName = normalizeUnit(10, "kB")
	require.Equal(t, 10.*1024, value)
	require.Equal(t, "B", unit)
	require.Equal(t, "bytes", displayName)

	value, unit, displayName = normalizeUnit(10, "MB")
	require.Equal(t, 10.*1024*1024, value)
	require.Equal(t, "B", unit)
	require.Equal(t, "bytes", displayName)

	value, unit, displayName = normalizeUnit(10, "GB")
	require.Equal(t, 10.*1024*1024*1024, value)
	require.Equal(t, "B", unit)
	require.Equal(t, "bytes", displayName)

	value, unit, displayName = normalizeUnit(10, "TB")
	require.Equal(t, 10.*1024*1024*1024*1024, value)
	require.Equal(t, "B", unit)
	require.Equal(t, "bytes", displayName)
}

func requireGaugeMetric(
	t *testing.T,
	metric pmetric.ScopeMetrics,
	name string,
	value interface{},
	unit string,
) {
	require.Equal(t, 1, metric.Metrics().Len())
	require.Equal(t, name, metric.Metrics().At(0).Name())
	require.Equal(t, 1, metric.Metrics().At(0).Gauge().DataPoints().Len())
	require.Equal(t, unit, metric.Metrics().At(0).Unit())
	require.Equal(t, value, metric.Metrics().At(0).Gauge().DataPoints().At(0).DoubleVal(), "expected metric=%s to have value=%s", name, value)
}

func requireSumMetric(
	t *testing.T,
	metric pmetric.ScopeMetrics,
	name string,
	value interface{},
	unit string,
) {
	require.Equal(t, 1, metric.Metrics().Len())
	require.Equal(t, name, metric.Metrics().At(0).Name())
	require.Equal(t, 1, metric.Metrics().At(0).Sum().DataPoints().Len())
	require.Equal(t, unit, metric.Metrics().At(0).Unit())
	require.Equal(t, value, metric.Metrics().At(0).Sum().DataPoints().At(0).IntVal(), "expected metric=%s to have value=%s", name, value)
}

func requireHistogramMetric(
	t *testing.T,
	metric pmetric.ScopeMetrics,
	name string,
	value interface{},
	unit string,
	le string,
) {
	require.Equal(t, 1, metric.Metrics().Len())
	require.Equal(t, name, metric.Metrics().At(0).Name())
	require.Equal(t, 1, metric.Metrics().At(0).Sum().DataPoints().Len())
	require.Equal(t, unit, metric.Metrics().At(0).Unit())
	actualLe, _ := metric.Metrics().At(0).Sum().DataPoints().At(0).Attributes().Get("le")
	require.Equal(t, le, actualLe.StringVal())
	require.Equal(t, value, metric.Metrics().At(0).Sum().DataPoints().At(0).IntVal(), "expected metric=%s to have value=%s", name, value)
}

// Sort the metrics deterministically to make assertions easier.
func sortMetrics(a, b pmetric.ScopeMetrics) bool {
	if a.Metrics().At(0).Name() == b.Metrics().At(0).Name() {
		leA, okA := a.Metrics().At(0).Sum().DataPoints().At(0).Attributes().Get("le")
		leB, okB := b.Metrics().At(0).Sum().DataPoints().At(0).Attributes().Get("le")
		if !okA || !okB {
			return false
		}
		return leA.StringVal() < leB.StringVal()
	}
	return a.Metrics().At(0).Name() > b.Metrics().At(0).Name()
}
