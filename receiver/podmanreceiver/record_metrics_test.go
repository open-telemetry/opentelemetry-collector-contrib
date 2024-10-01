// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package podmanreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type point struct {
	intVal     uint64
	doubleVal  float64
	attributes map[string]string
}

func assertStatsEqualToMetrics(t *testing.T, podmanStats *containerStats, md pmetric.Metrics) {
	assert.Equal(t, 1, md.ResourceMetrics().Len())
	rsm := md.ResourceMetrics().At(0)

	resourceAttrs := map[string]string{
		"container.runtime":    "podman",
		"container.id":         "abcd1234",
		"container.name":       "cntrA",
		"container.image.name": "localimage",
	}
	for k, v := range resourceAttrs {
		attr, exists := rsm.Resource().Attributes().Get(k)
		assert.True(t, exists)
		assert.Equal(t, v, attr.Str())
	}

	assert.Equal(t, 1, rsm.ScopeMetrics().Len())

	metrics := rsm.ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 11, metrics.Len())

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		switch m.Name() {
		case "container.memory.usage.limit":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: podmanStats.MemLimit}})
		case "container.memory.usage.total":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: podmanStats.MemUsage}})
		case "container.memory.percent":
			assertMetricEqual(t, m, pmetric.MetricTypeGauge, []point{{doubleVal: podmanStats.MemPerc}})
		case "container.network.io.usage.tx_bytes":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: podmanStats.NetInput}})
		case "container.network.io.usage.rx_bytes":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: podmanStats.NetOutput}})

		case "container.blockio.io_service_bytes_recursive.write":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: podmanStats.BlockOutput}})
		case "container.blockio.io_service_bytes_recursive.read":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: podmanStats.BlockInput}})

		case "container.cpu.usage.system":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: toSecondsWithNanosecondPrecision(podmanStats.CPUSystemNano)}})
		case "container.cpu.usage.total":
			assertMetricEqual(t, m, pmetric.MetricTypeSum, []point{{intVal: toSecondsWithNanosecondPrecision(podmanStats.CPUNano)}})
		case "container.cpu.percent":
			assertMetricEqual(t, m, pmetric.MetricTypeGauge, []point{{doubleVal: podmanStats.CPU}})
		case "container.cpu.usage.percpu":
			points := make([]point, len(podmanStats.PerCPU))
			for i, v := range podmanStats.PerCPU {
				points[i] = point{intVal: toSecondsWithNanosecondPrecision(v), attributes: map[string]string{"core": fmt.Sprintf("cpu%d", i)}}
			}
			assertMetricEqual(t, m, pmetric.MetricTypeSum, points)

		default:
			t.Errorf("unexpected metric: %s", m.Name())
		}
	}
}

func assertMetricEqual(t *testing.T, m pmetric.Metric, dt pmetric.MetricType, pts []point) {
	assert.Equal(t, m.Type(), dt)
	switch dt {
	case pmetric.MetricTypeGauge:
		assertPoints(t, m.Gauge().DataPoints(), pts)
	case pmetric.MetricTypeSum:
		assertPoints(t, m.Sum().DataPoints(), pts)
	case pmetric.MetricTypeEmpty:
		t.Errorf("unexpected data type: %s", dt)
	case pmetric.MetricTypeHistogram:
		t.Errorf("unexpected data type: %s", dt)
	case pmetric.MetricTypeExponentialHistogram:
		t.Errorf("unexpected data type: %s", dt)
	case pmetric.MetricTypeSummary:
		t.Errorf("unexpected data type: %s", dt)
	default:
		t.Errorf("unexpected data type: %s", dt)
	}
}

func assertPoints(t *testing.T, dpts pmetric.NumberDataPointSlice, pts []point) {
	assert.Len(t, pts, dpts.Len())
	for i, expected := range pts {
		got := dpts.At(i)
		assert.Equal(t, got.IntValue(), int64(expected.intVal))
		assert.Equal(t, expected.doubleVal, got.DoubleValue())
		for k, expectedV := range expected.attributes {
			gotV, exists := got.Attributes().Get(k)
			assert.True(t, exists)
			assert.Equal(t, expectedV, gotV.Str())
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
