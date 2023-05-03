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

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

type point struct {
	intVal     uint64
	doubleVal  float64
	attributes map[string]string
}

func containerStatsToMetrics(ts time.Time, container container, stats *containerStats) pmetric.Metrics {
	pbts := pcommon.NewTimestampFromTime(ts)

	md := pmetric.NewMetrics()
	rs := md.ResourceMetrics().AppendEmpty()

	resourceAttr := rs.Resource().Attributes()
	resourceAttr.PutStr(conventions.AttributeContainerRuntime, "podman")
	resourceAttr.PutStr(conventions.AttributeContainerName, stats.Name)
	resourceAttr.PutStr(conventions.AttributeContainerID, stats.ContainerID)
	resourceAttr.PutStr(conventions.AttributeContainerImageName, container.Image)

	ms := rs.ScopeMetrics().AppendEmpty().Metrics()
	appendIOMetrics(ms, stats, pbts)
	appendCPUMetrics(ms, stats, pbts)
	appendNetworkMetrics(ms, stats, pbts)
	appendMemoryMetrics(ms, stats, pbts)

	return md
}

func appendMemoryMetrics(ms pmetric.MetricSlice, stats *containerStats, ts pcommon.Timestamp) {
	gaugeI(ms, "memory.usage.limit", "By", []point{{intVal: stats.MemLimit}}, ts)
	gaugeI(ms, "memory.usage.total", "By", []point{{intVal: stats.MemUsage}}, ts)
	gaugeF(ms, "memory.percent", "1", []point{{doubleVal: stats.MemPerc}}, ts)
}

func appendNetworkMetrics(ms pmetric.MetricSlice, stats *containerStats, ts pcommon.Timestamp) {
	sum(ms, "network.io.usage.tx_bytes", "By", []point{{intVal: stats.NetInput}}, ts)
	sum(ms, "network.io.usage.rx_bytes", "By", []point{{intVal: stats.NetOutput}}, ts)
}

func appendIOMetrics(ms pmetric.MetricSlice, stats *containerStats, ts pcommon.Timestamp) {
	sum(ms, "blockio.io_service_bytes_recursive.write", "By", []point{{intVal: stats.BlockOutput}}, ts)
	sum(ms, "blockio.io_service_bytes_recursive.read", "By", []point{{intVal: stats.BlockInput}}, ts)
}

func appendCPUMetrics(ms pmetric.MetricSlice, stats *containerStats, ts pcommon.Timestamp) {
	sum(ms, "cpu.usage.system", "ns", []point{{intVal: stats.CPUSystemNano}}, ts)
	sum(ms, "cpu.usage.total", "ns", []point{{intVal: stats.CPUNano}}, ts)
	gaugeF(ms, "cpu.percent", "1", []point{{doubleVal: stats.CPU}}, ts)

	points := make([]point, len(stats.PerCPU))
	for i, cpu := range stats.PerCPU {
		points[i] = point{
			intVal: cpu,
			attributes: map[string]string{
				"core": fmt.Sprintf("cpu%d", i),
			},
		}
	}
	sum(ms, "cpu.usage.percpu", "ns", points, ts)
}

func initMetric(ms pmetric.MetricSlice, name, unit string) pmetric.Metric {
	m := ms.AppendEmpty()
	m.SetName(fmt.Sprintf("container.%s", name))
	m.SetUnit(unit)
	return m
}

func sum(ilm pmetric.MetricSlice, metricName string, unit string, points []point, ts pcommon.Timestamp) {
	metric := initMetric(ilm, metricName, unit)
	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dataPoints := sum.DataPoints()

	for _, pt := range points {
		dataPoint := dataPoints.AppendEmpty()
		dataPoint.SetTimestamp(ts)
		dataPoint.SetIntValue(int64(pt.intVal))
		setDataPointAttributes(dataPoint, pt.attributes)
	}
}

func gauge(ms pmetric.MetricSlice, metricName string, unit string) pmetric.NumberDataPointSlice {
	metric := initMetric(ms, metricName, unit)
	gauge := metric.SetEmptyGauge()
	return gauge.DataPoints()
}

func gaugeI(ms pmetric.MetricSlice, metricName string, unit string, points []point, ts pcommon.Timestamp) {
	dataPoints := gauge(ms, metricName, unit)
	for _, pt := range points {
		dataPoint := dataPoints.AppendEmpty()
		dataPoint.SetTimestamp(ts)
		dataPoint.SetIntValue(int64(pt.intVal))
		setDataPointAttributes(dataPoint, pt.attributes)
	}
}

func gaugeF(ms pmetric.MetricSlice, metricName string, unit string, points []point, ts pcommon.Timestamp) {
	dataPoints := gauge(ms, metricName, unit)
	for _, pt := range points {
		dataPoint := dataPoints.AppendEmpty()
		dataPoint.SetTimestamp(ts)
		dataPoint.SetDoubleValue(pt.doubleVal)
		setDataPointAttributes(dataPoint, pt.attributes)
	}
}

func setDataPointAttributes(dataPoint pmetric.NumberDataPoint, attributes map[string]string) {
	for k, v := range attributes {
		dataPoint.Attributes().PutStr(k, v)
	}
}
