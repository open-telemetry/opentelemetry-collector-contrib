// Copyright 2020, OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestTranslateStatsToMetrics(t *testing.T) {
	ts := time.Now()
	stats := genContainerStats()
	metrics := translateStatsToMetrics(stats, ts)
	assert.NotNil(t, metrics)

	assertStatsEqualToMetrics(t, stats, metrics)
}

func assertStatsEqualToMetrics(t *testing.T, podmanStats *containerStats, pdataMetrics pdata.Metrics) {
	assert.Equal(t, pdataMetrics.ResourceMetrics().Len(), 1)
	rsm := pdataMetrics.ResourceMetrics().At(0)

	resourceAttrs := map[string]string{
		"container.id":   "abcd1234",
		"container.name": "cntrA",
	}
	for k, v := range resourceAttrs {
		attr, exists := rsm.Resource().Attributes().Get(k)
		assert.True(t, exists)
		assert.Equal(t, attr.StringVal(), v)
	}

	assert.Equal(t, rsm.InstrumentationLibraryMetrics().Len(), 1)

	metrics := rsm.InstrumentationLibraryMetrics().At(0).Metrics()
	assert.Equal(t, metrics.Len(), 11)

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		switch m.Name() {
		case "container.memory.usage.limit":
			assertMetricEqual(t, m, pdata.MetricDataTypeGauge, []point{{intVal: podmanStats.MemLimit}})
		case "container.memory.usage.total":
			assertMetricEqual(t, m, pdata.MetricDataTypeGauge, []point{{intVal: podmanStats.MemUsage}})
		case "container.memory.percent":
			assertMetricEqual(t, m, pdata.MetricDataTypeGauge, []point{{doubleVal: podmanStats.MemPerc}})
		case "container.network.io.usage.tx_bytes":
			assertMetricEqual(t, m, pdata.MetricDataTypeSum, []point{{intVal: podmanStats.NetInput}})
		case "container.network.io.usage.rx_bytes":
			assertMetricEqual(t, m, pdata.MetricDataTypeSum, []point{{intVal: podmanStats.NetOutput}})

		case "container.blockio.io_service_bytes_recursive.write":
			assertMetricEqual(t, m, pdata.MetricDataTypeSum, []point{{intVal: podmanStats.BlockOutput}})
		case "container.blockio.io_service_bytes_recursive.read":
			assertMetricEqual(t, m, pdata.MetricDataTypeSum, []point{{intVal: podmanStats.BlockInput}})

		case "container.cpu.usage.system":
			assertMetricEqual(t, m, pdata.MetricDataTypeSum, []point{{intVal: podmanStats.CPUSystemNano}})
		case "container.cpu.usage.total":
			assertMetricEqual(t, m, pdata.MetricDataTypeSum, []point{{intVal: podmanStats.CPUNano}})
		case "container.cpu.percent":
			assertMetricEqual(t, m, pdata.MetricDataTypeGauge, []point{{doubleVal: podmanStats.CPU}})
		case "container.cpu.usage.percpu":
			points := make([]point, len(podmanStats.PerCPU))
			for i, v := range podmanStats.PerCPU {
				points[i] = point{intVal: v, attributes: map[string]string{"core": fmt.Sprintf("cpu%d", i)}}
			}
			assertMetricEqual(t, m, pdata.MetricDataTypeSum, points)

		default:
			t.Errorf(fmt.Sprintf("unexpected metric: %s", m.Name()))
		}
	}
}

func assertMetricEqual(t *testing.T, m pdata.Metric, dt pdata.MetricDataType, pts []point) {
	assert.Equal(t, m.DataType(), dt)
	switch dt {
	case pdata.MetricDataTypeGauge:
		assertPoints(t, m.Gauge().DataPoints(), pts)
	case pdata.MetricDataTypeSum:
		assertPoints(t, m.Sum().DataPoints(), pts)
	default:
		t.Errorf("unexpected data type: %s", dt)
	}
}

func assertPoints(t *testing.T, dpts pdata.NumberDataPointSlice, pts []point) {
	assert.Equal(t, dpts.Len(), len(pts))
	for i, expected := range pts {
		got := dpts.At(i)
		assert.Equal(t, got.IntVal(), int64(expected.intVal))
		assert.Equal(t, got.DoubleVal(), expected.doubleVal)
		for k, expectedV := range expected.attributes {
			gotV, exists := got.Attributes().Get(k)
			assert.True(t, exists)
			assert.Equal(t, gotV.StringVal(), expectedV)
		}
	}
}

func genContainerStats() *containerStats {
	return &containerStats{
		ContainerID:   "abcd1234",
		Name:          "cntrA",
		PerCPU:        []uint64{40, 50, 20, 15},
		CPU:           78.67,
		CPUNano:       3451990,
		CPUSystemNano: 4573681,
		SystemNano:    3493456,
		MemUsage:      87,
		MemLimit:      200,
		MemPerc:       43.5,
		NetInput:      349323,
		NetOutput:     762442,
		BlockInput:    943894,
		BlockOutput:   324234,
		PIDs:          3,
	}
}
