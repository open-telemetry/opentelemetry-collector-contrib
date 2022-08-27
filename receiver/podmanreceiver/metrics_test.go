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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestTranslateStatsToMetrics(t *testing.T) {
	ts := time.Now()
	stats := genContainerStats()
	config := genConfig()
	containerDto := genContainer()
	md := containerStatsToMetrics(ts, containerDto, stats, config)
	assertStatsEqualToMetrics(t, stats, md)
}

func assertStatsEqualToMetrics(t *testing.T, podmanStats *containerStats, md pmetric.Metrics) {
	assert.Equal(t, md.ResourceMetrics().Len(), 1)
	rsm := md.ResourceMetrics().At(0)

	resourceAttrs := map[string]string{
		"container.runtime":    "podman",
		"container.id":         "abcd1234",
		"container.name":       "cntrA",
		"container.image.name": "docker.io/library/httpd:latest",
		"label_value1":         "container_label_value_1",
		"env_value2":           "container_env_value_2",
	}
	for k, v := range resourceAttrs {
		attr, exists := rsm.Resource().Attributes().Get(k)
		assert.True(t, exists)
		assert.Equal(t, attr.StringVal(), v)
	}

	assert.Equal(t, rsm.ScopeMetrics().Len(), 1)

	metrics := rsm.ScopeMetrics().At(0).Metrics()
	assert.Equal(t, metrics.Len(), 11)

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		switch m.Name() {
		case "container.memory.usage.limit":
			assertMetricEqual(t, m, pmetric.MetricDataTypeGauge, []point{{intVal: podmanStats.MemLimit}})
		case "container.memory.usage.total":
			assertMetricEqual(t, m, pmetric.MetricDataTypeGauge, []point{{intVal: podmanStats.MemUsage}})
		case "container.memory.percent":
			assertMetricEqual(t, m, pmetric.MetricDataTypeGauge, []point{{doubleVal: podmanStats.MemPerc}})
		case "container.network.io.usage.tx_bytes":
			assertMetricEqual(t, m, pmetric.MetricDataTypeSum, []point{{intVal: podmanStats.NetInput}})
		case "container.network.io.usage.rx_bytes":
			assertMetricEqual(t, m, pmetric.MetricDataTypeSum, []point{{intVal: podmanStats.NetOutput}})

		case "container.blockio.io_service_bytes_recursive.write":
			assertMetricEqual(t, m, pmetric.MetricDataTypeSum, []point{{intVal: podmanStats.BlockOutput}})
		case "container.blockio.io_service_bytes_recursive.read":
			assertMetricEqual(t, m, pmetric.MetricDataTypeSum, []point{{intVal: podmanStats.BlockInput}})

		case "container.cpu.usage.system":
			assertMetricEqual(t, m, pmetric.MetricDataTypeSum, []point{{intVal: podmanStats.CPUSystemNano}})
		case "container.cpu.usage.total":
			assertMetricEqual(t, m, pmetric.MetricDataTypeSum, []point{{intVal: podmanStats.CPUNano}})
		case "container.cpu.percent":
			assertMetricEqual(t, m, pmetric.MetricDataTypeGauge, []point{{doubleVal: podmanStats.CPU}})
		case "container.cpu.usage.percpu":
			points := make([]point, len(podmanStats.PerCPU))
			for i, v := range podmanStats.PerCPU {
				points[i] = point{intVal: v, attributes: map[string]string{"core": fmt.Sprintf("cpu%d", i)}}
			}
			assertMetricEqual(t, m, pmetric.MetricDataTypeSum, points)

		default:
			t.Errorf(fmt.Sprintf("unexpected metric: %s", m.Name()))
		}
	}
}

func assertMetricEqual(t *testing.T, m pmetric.Metric, dt pmetric.MetricDataType, pts []point) {
	assert.Equal(t, m.DataType(), dt)
	switch dt {
	case pmetric.MetricDataTypeGauge:
		assertPoints(t, m.Gauge().DataPoints(), pts)
	case pmetric.MetricDataTypeSum:
		assertPoints(t, m.Sum().DataPoints(), pts)
	default:
		t.Errorf("unexpected data type: %s", dt)
	}
}

func assertPoints(t *testing.T, dpts pmetric.NumberDataPointSlice, pts []point) {
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

func genConfig() *Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Millisecond,
		},
		Endpoint: "/run/user/1000/podman/podman.sock",
		Timeout:  5 * time.Second,
		ContainerLabelsToMetricLabels: map[string]string{
			"container_label_1": "label_value1",
		},
		EnvVarsToMetricLabels: map[string]string{
			"container_env_2": "env_value2",
		},
		APIVersion:    "3.2.0",
		SSHKey:        "/path/to/ssh/private/key",
		SSHPassphrase: "pass",
	}
}

func genContainer() container {
	return container{
		ID: "49a4c52afb06e6b36b2941422a0adf47421dbfbf40503dbe17bd56b4570b6681",
		Config: containerConfig{
			Env: map[string]string{
				"container_env_1": "container_env_value_1",
				"container_env_2": "container_env_value_2",
			},
			Image: "docker.io/library/httpd:latest",
			Labels: map[string]string{
				"container_label_1": "container_label_value_1",
				"container_label_2": "container_label_value_2",
			},
		},
	}
}
