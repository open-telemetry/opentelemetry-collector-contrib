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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
)

func TestYAML(t *testing.T) {
	var cfg metricstransformprocessor.Config
	err := yamlToConfig(&cfg)
	require.NoError(t, err)
	require.EqualValues(t, cfg.Transforms, []metricstransformprocessor.Transform{
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:   "system.cpu.load_average.1m",
				MatchType: "strict",
			},
			Action:  "insert",
			NewName: "system.load.1",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:   "system.cpu.load_average.5m",
				MatchType: "strict",
			},
			Action:  "insert",
			NewName: "system.load.5",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:   "system.cpu.load_average.15m",
				MatchType: "strict",
			},
			Action:  "insert",
			NewName: "system.load.15",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.cpu.utilization",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "idle"},
			},
			Action:  "insert",
			NewName: "system.cpu.idle",
			Operations: []metricstransformprocessor.Operation{
				{
					Action: "experimental_scale_value",
					Scale:  100,
				},
			},
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.cpu.utilization",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "user"},
			},
			Action:  "insert",
			NewName: "system.cpu.user",
			Operations: []metricstransformprocessor.Operation{
				{
					Action: "experimental_scale_value",
					Scale:  100,
				},
			},
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.cpu.utilization",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "system"},
			},
			Action:  "insert",
			NewName: "system.cpu.system",
			Operations: []metricstransformprocessor.Operation{
				{
					Action: "experimental_scale_value",
					Scale:  100,
				},
			},
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.cpu.utilization",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "wait"},
			},
			Action:  "insert",
			NewName: "system.cpu.iowait",
			Operations: []metricstransformprocessor.Operation{
				{
					Action: "experimental_scale_value",
					Scale:  100,
				},
			},
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.cpu.utilization",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "steal"},
			},
			Action:  "insert",
			NewName: "system.cpu.stolen",
			Operations: []metricstransformprocessor.Operation{
				{
					Action: "experimental_scale_value",
					Scale:  100,
				},
			},
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:   "system.memory.usage",
				MatchType: "strict",
			},
			Action:  "insert",
			NewName: "system.mem.total",
			Operations: []metricstransformprocessor.Operation{
				{
					Action: "experimental_scale_value",
					Scale:  1e-06,
				},
			},
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.memory.usage",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "free"},
			},
			Action:  "insert",
			NewName: "system.mem.usable",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.memory.usage",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "cached"},
			},
			Action:  "insert",
			NewName: "system.mem.usable",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.memory.usage",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "buffered"},
			},
			Action:  "insert",
			NewName: "system.mem.usable",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:   "system.mem.usable",
				MatchType: "strict",
			},
			Action: "update",
			Operations: []metricstransformprocessor.Operation{
				{
					Action:           "aggregate_label_values",
					Label:            "state",
					AggregationType:  "sum",
					AggregatedValues: []string{"free", "cached", "buffered"},
					NewValue:         "usable",
				},
				{
					Action: "experimental_scale_value",
					Scale:  1e-06,
				},
			},
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.network.io",
				MatchType:   "strict",
				MatchLabels: map[string]string{"direction": "receive"},
			},
			Action:  "insert",
			NewName: "system.net.bytes_rcvd",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.network.io",
				MatchType:   "strict",
				MatchLabels: map[string]string{"direction": "transmit"},
			},
			Action:  "insert",
			NewName: "system.net.bytes_sent",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.paging.usage",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "free"},
			},
			Action:  "insert",
			NewName: "system.swap.free",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:     "system.paging.usage",
				MatchType:   "strict",
				MatchLabels: map[string]string{"state": "used"},
			},
			Action:  "insert",
			NewName: "system.swap.used",
		},
		{
			MetricIncludeFilter: metricstransformprocessor.FilterConfig{
				Include:   "system.filesystem.utilization",
				MatchType: "strict",
			},
			Action:  "insert",
			NewName: "system.disk.in_use",
		},
	})
}

func TestTransformer(t *testing.T) {
	var called bool
	tt := &Transformer{
		names: map[string]struct{}{"x": {}},
		consumer: func(_ context.Context, _ pmetric.Metrics) error {
			called = true
			return nil
		},
	}
	require.True(t, tt.Has("x"))
	require.False(t, tt.Has("y"))
	err := tt.Transform(context.Background(), pmetric.Metrics{})
	require.NoError(t, err)
	require.True(t, called)
}

func TestTransformFunc(t *testing.T) {
	ctx := context.Background()
	transformer, err := NewInfraTransformer(ctx, component.ExporterCreateSettings{}, config.NewComponentID("test"))
	if err != nil {
		t.Fatal(err)
	}
	md := testutils.NewGaugeMetrics([]testutils.TestGauge{
		{Name: "metric0", DataPoints: []testutils.DataPoint{{Value: 0.1}}},
		{Name: "metric1", DataPoints: []testutils.DataPoint{{Value: 1}}},
		{Name: "metric2", DataPoints: []testutils.DataPoint{{Value: 2}}},
		{Name: "system.cpu.load_average.1m", DataPoints: []testutils.DataPoint{{Value: 4}}},
		{Name: "system.cpu.load_average.5m", DataPoints: []testutils.DataPoint{{Value: 5}}},
		{Name: "system.cpu.load_average.15m", DataPoints: []testutils.DataPoint{{Value: 6}}},
		{Name: "system.paging.usage", DataPoints: []testutils.DataPoint{
			{Value: 55, Attributes: map[string]string{"state": "free"}},
			{Value: 66, Attributes: map[string]string{"state": "used"}},
			{Value: 77},
		}},
		{Name: "system.cpu.utilization", DataPoints: []testutils.DataPoint{
			{Value: 7, Attributes: map[string]string{"state": "idle"}},
			{Value: 8, Attributes: map[string]string{"state": "user"}},
			{Value: 9, Attributes: map[string]string{"state": "wait"}},
			{Value: 10, Attributes: map[string]string{"state": "steal"}},
			{Value: 20, Attributes: map[string]string{"state": "system"}},
			{Value: 100},
		}},
		{Name: "system.memory.usage", DataPoints: []testutils.DataPoint{
			{Value: 11},
			{Value: 12, Attributes: map[string]string{"state": "free"}},
			{Value: 13, Attributes: map[string]string{"state": "cached"}},
			{Value: 14, Attributes: map[string]string{"state": "buffered"}},
			{Value: 101, Attributes: map[string]string{"state": "other"}},
		}},
		{Name: "system.network.io", DataPoints: []testutils.DataPoint{
			{Value: 13, Attributes: map[string]string{"direction": "receive"}},
			{Value: 14, Attributes: map[string]string{"direction": "transmit"}},
			{Value: 102, Attributes: map[string]string{"direction": "other"}},
		}},
		{Name: "system.filesystem.utilization", DataPoints: []testutils.DataPoint{{Value: 15}}},
	})
	require.NoError(t, transformer.Transform(ctx, md))

	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	// NVA specifies a set of Name, Value & Attributes
	type NVA struct {
		N string
		V float64
		A map[string]interface{}
	}
	var all []NVA
	for i := 0; i < ms.Len(); i++ {
		for j := 0; j < ms.At(i).Gauge().DataPoints().Len(); j++ {
			p := ms.At(i).Gauge().DataPoints().At(j)
			nva := NVA{
				N: ms.At(i).Name(),
				V: p.DoubleValue(),
			}
			if p.Attributes().Len() > 0 {
				nva.A = p.Attributes().AsRaw()
			}
			all = append(all, nva)
		}
	}
	require.EqualValues(t, all, []NVA{
		{N: "metric0", V: 0.1},
		{N: "metric1", V: 1},
		{N: "metric2", V: 2},
		{N: "system.cpu.load_average.1m", V: 4},
		{N: "system.cpu.load_average.5m", V: 5},
		{N: "system.cpu.load_average.15m", V: 6},
		{N: "system.paging.usage", V: 55, A: map[string]interface{}{"state": "free"}},
		{N: "system.paging.usage", V: 66, A: map[string]interface{}{"state": "used"}},
		{N: "system.paging.usage", V: 77},
		{N: "system.cpu.utilization", V: 7, A: map[string]interface{}{"state": "idle"}},
		{N: "system.cpu.utilization", V: 8, A: map[string]interface{}{"state": "user"}},
		{N: "system.cpu.utilization", V: 9, A: map[string]interface{}{"state": "wait"}},
		{N: "system.cpu.utilization", V: 10, A: map[string]interface{}{"state": "steal"}},
		{N: "system.cpu.utilization", V: 20, A: map[string]interface{}{"state": "system"}},
		{N: "system.cpu.utilization", V: 100},
		{N: "system.memory.usage", V: 11},
		{N: "system.memory.usage", V: 12, A: map[string]interface{}{"state": "free"}},
		{N: "system.memory.usage", V: 13, A: map[string]interface{}{"state": "cached"}},
		{N: "system.memory.usage", V: 14, A: map[string]interface{}{"state": "buffered"}},
		{N: "system.memory.usage", V: 101, A: map[string]interface{}{"state": "other"}},
		{N: "system.network.io", V: 13, A: map[string]interface{}{"direction": "receive"}},
		{N: "system.network.io", V: 14, A: map[string]interface{}{"direction": "transmit"}},
		{N: "system.network.io", V: 102, A: map[string]interface{}{"direction": "other"}},
		{N: "system.filesystem.utilization", V: 15},
		{N: "system.load.1", V: 4},
		{N: "system.load.5", V: 5},
		{N: "system.load.15", V: 6},
		{N: "system.cpu.idle", V: 700, A: map[string]interface{}{"state": "idle"}},
		{N: "system.cpu.user", V: 800, A: map[string]interface{}{"state": "user"}},
		{N: "system.cpu.system", V: 2000, A: map[string]interface{}{"state": "system"}},
		{N: "system.cpu.iowait", V: 900, A: map[string]interface{}{"state": "wait"}},
		{N: "system.cpu.stolen", V: 1000, A: map[string]interface{}{"state": "steal"}},
		{N: "system.mem.total", V: 0.000011},
		{N: "system.mem.total", V: 0.000012, A: map[string]interface{}{"state": "free"}},
		{N: "system.mem.total", V: 0.000013, A: map[string]interface{}{"state": "cached"}},
		{N: "system.mem.total", V: 0.000014, A: map[string]interface{}{"state": "buffered"}},
		{N: "system.mem.total", V: 0.000101, A: map[string]interface{}{"state": "other"}},
		{N: "system.mem.usable", V: 0.000012, A: map[string]interface{}{"state": "usable"}},
		{N: "system.mem.usable", V: 0.000013, A: map[string]interface{}{"state": "usable"}},
		{N: "system.mem.usable", V: 0.000014, A: map[string]interface{}{"state": "usable"}},
		{N: "system.net.bytes_rcvd", V: 13, A: map[string]interface{}{"direction": "receive"}},
		{N: "system.net.bytes_sent", V: 14, A: map[string]interface{}{"direction": "transmit"}},
		{N: "system.swap.free", V: 55, A: map[string]interface{}{"state": "free"}},
		{N: "system.swap.used", V: 66, A: map[string]interface{}{"state": "used"}},
		{N: "system.disk.in_use", V: 15},
	})
	require.EqualValues(t, transformer.names, map[string]struct{}{
		"system.cpu.idle":       {},
		"system.cpu.iowait":     {},
		"system.cpu.stolen":     {},
		"system.cpu.user":       {},
		"system.cpu.system":     {},
		"system.disk.in_use":    {},
		"system.load.1":         {},
		"system.load.15":        {},
		"system.load.5":         {},
		"system.mem.total":      {},
		"system.mem.usable":     {},
		"system.net.bytes_rcvd": {},
		"system.net.bytes_sent": {},
		"system.swap.free":      {},
		"system.swap.used":      {},
	})
}
