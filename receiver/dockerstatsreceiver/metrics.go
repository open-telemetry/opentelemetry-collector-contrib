// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dockerstatsreceiver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	dtypes "github.com/docker/docker/api/types"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

const (
	metricPrefix = "container."
)

// client.ContainerInspect() response container
// stats and translated environment string map
// for potential labels
type DockerContainer struct {
	*dtypes.ContainerJSON
	EnvMap map[string]string
}

func ContainerStatsToMetrics(
	containerStats *dtypes.StatsJSON,
	container *DockerContainer,
	config *Config,
) (*consumerdata.MetricsData, error) {
	now, _ := ptypes.TimestampProto(time.Now())

	var metrics []*metricspb.Metric
	metrics = append(metrics, blockioMetrics(&containerStats.BlkioStats, now)...)
	metrics = append(metrics, cpuMetrics(&containerStats.CPUStats, &containerStats.PreCPUStats, now, config.ProvidePerCoreCPUMetrics)...)
	metrics = append(metrics, memoryMetrics(&containerStats.MemoryStats, now)...)
	metrics = append(metrics, networkMetrics(&containerStats.Networks, now)...)

	if len(metrics) == 0 {
		return nil, nil
	}

	md := &consumerdata.MetricsData{
		Metrics: metrics,
		Resource: &resourcepb.Resource{
			Type: "container",
			Labels: map[string]string{
				"container.hostname":                container.Config.Hostname,
				conventions.AttributeContainerID:    container.ID,
				conventions.AttributeContainerImage: container.Config.Image,
				conventions.AttributeContainerName:  strings.TrimPrefix(container.Name, "/"),
			},
		},
	}

	updateConfiguredResourceLabels(md, container, config)

	return md, nil
}

func updateConfiguredResourceLabels(md *consumerdata.MetricsData, container *DockerContainer, config *Config) {
	for k, label := range config.EnvVarsToMetricLabels {
		if v := container.EnvMap[k]; v != "" {
			md.Resource.Labels[label] = v
		}
	}

	for k, label := range config.ContainerLabelsToMetricLabels {
		if v := container.Config.Labels[k]; v != "" {
			md.Resource.Labels[label] = v
		}
	}
}

type blkioStat struct {
	name    string
	unit    string
	entries []dtypes.BlkioStatEntry
}

// metrics for https://www.kernel.org/doc/Documentation/cgroup-v1/blkio-controller.txt
func blockioMetrics(
	blkioStats *dtypes.BlkioStats,
	ts *timestamp.Timestamp,
) []*metricspb.Metric {
	var metrics []*metricspb.Metric

	for _, blkiostat := range []blkioStat{
		{"io_merged_recursive", "1", blkioStats.IoMergedRecursive},
		{"io_queued_recursive", "1", blkioStats.IoQueuedRecursive},
		{"io_service_bytes_recursive", "By", blkioStats.IoServiceBytesRecursive},
		{"io_service_time_recursive", "ns", blkioStats.IoServiceTimeRecursive},
		{"io_serviced_recursive", "1", blkioStats.IoServicedRecursive},
		{"io_time_recursive", "ms", blkioStats.IoTimeRecursive},
		{"io_wait_time_recursive", "1", blkioStats.IoWaitTimeRecursive},
		{"sectors_recursive", "1", blkioStats.SectorsRecursive},
	} {
		labelKeys := []string{"device_major", "device_minor"}
		for _, stat := range blkiostat.entries {
			if stat.Op == "" {
				continue
			}

			statName := fmt.Sprintf("%s.%s", blkiostat.name, strings.ToLower(stat.Op))
			metricName := fmt.Sprintf("blockio.%s", statName)
			labelValues := [][]string{{strconv.FormatUint(stat.Major, 10), strconv.FormatUint(stat.Minor, 10)}}
			metrics = append(metrics, Cumulative(metricName, []int64{int64(stat.Value)}, ts, blkiostat.unit, labelKeys, labelValues))
		}
	}

	return metrics
}

func cpuMetrics(
	cpuStats *dtypes.CPUStats,
	previousCPUStats *dtypes.CPUStats,
	ts *timestamp.Timestamp,
	providePerCoreMetrics bool,
) []*metricspb.Metric {
	var metrics []*metricspb.Metric

	metrics = append(metrics, []*metricspb.Metric{
		Cumulative("cpu.usage.system", []int64{int64(cpuStats.SystemUsage)}, ts, "ns", nil, nil),
		Cumulative("cpu.usage.total", []int64{int64(cpuStats.CPUUsage.TotalUsage)}, ts, "ns", nil, nil),
	}...)

	metrics = append(metrics, []*metricspb.Metric{
		Cumulative("cpu.usage.kernelmode", []int64{int64(cpuStats.CPUUsage.UsageInKernelmode)}, ts, "ns", nil, nil),
		Cumulative("cpu.usage.usermode", []int64{int64(cpuStats.CPUUsage.UsageInUsermode)}, ts, "ns", nil, nil),
	}...)

	metrics = append(metrics, []*metricspb.Metric{
		Cumulative("cpu.throttling_data.periods", []int64{int64(cpuStats.ThrottlingData.Periods)}, ts, "1", nil, nil),
		Cumulative("cpu.throttling_data.throttled_periods", []int64{int64(cpuStats.ThrottlingData.ThrottledPeriods)}, ts, "1", nil, nil),
		Cumulative("cpu.throttling_data.throttled_time", []int64{int64(cpuStats.ThrottlingData.ThrottledTime)}, ts, "ns", nil, nil),
	}...)

	metrics = append(metrics, GaugeF("cpu.percent", []float64{calculateCPUPercent(previousCPUStats, cpuStats)}, ts, "1", nil, nil))

	if !providePerCoreMetrics {
		return metrics
	}

	percpuValues := make([]int64, 0, len(cpuStats.CPUUsage.PercpuUsage))
	percpuLabelKeys := []string{"core"}
	percpuLabelValues := make([][]string, 0, len(cpuStats.CPUUsage.PercpuUsage))
	for coreNum, v := range cpuStats.CPUUsage.PercpuUsage {
		percpuValues = append(percpuValues, int64(v))
		percpuLabelValues = append(percpuLabelValues, []string{fmt.Sprintf("cpu%s", strconv.Itoa(coreNum))})
	}
	metrics = append(metrics, Cumulative("cpu.usage.percpu", percpuValues, ts, "ns", percpuLabelKeys, percpuLabelValues))
	return metrics
}

// From container.calculateCPUPercentUnix()
// https://github.com/docker/cli/blob/dbd96badb6959c2b7070664aecbcf0f7c299c538/cli/command/container/stats_helpers.go
// Copyright 2012-2017 Docker, Inc.
// This product includes software developed at Docker, Inc. (https://www.docker.com).
// The following is courtesy of our legal counsel:
// Use and transfer of Docker may be subject to certain restrictions by the
// United States and other governments.
// It is your responsibility to ensure that your use and/or transfer does not
// violate applicable laws.
// For more information, please see https://www.bis.doc.gov
// See also https://www.apache.org/dev/crypto.html and/or seek legal counsel.
func calculateCPUPercent(previous *dtypes.CPUStats, v *dtypes.CPUStats) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUUsage.TotalUsage) - float64(previous.CPUUsage.TotalUsage)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.SystemUsage) - float64(previous.SystemUsage)
		onlineCPUs  = float64(v.OnlineCPUs)
	)

	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(v.CPUUsage.PercpuUsage))
	}
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}
	return cpuPercent
}

var memoryStatsThatAreCumulative = map[string]bool{
	"pgfault":          true,
	"pgmajfault":       true,
	"pgpgin":           true,
	"pgpgout":          true,
	"total_pgfault":    true,
	"total_pgmajfault": true,
	"total_pgpgin":     true,
	"total_pgpgout":    true,
}

func memoryMetrics(
	memoryStats *dtypes.MemoryStats,
	ts *timestamp.Timestamp,
) []*metricspb.Metric {
	var metrics []*metricspb.Metric

	totalUsage := int64(memoryStats.Usage - memoryStats.Stats["total_cache"])
	metrics = append(metrics, []*metricspb.Metric{
		Gauge("memory.usage.limit", []int64{int64(memoryStats.Limit)}, ts, "By", nil, nil),
		Gauge("memory.usage.total", []int64{totalUsage}, ts, "By", nil, nil),
	}...)

	var pctUsed float64
	if float64(memoryStats.Limit) == 0 {
		pctUsed = 0
	} else {
		pctUsed = 100.0 * (float64(memoryStats.Usage) - float64(memoryStats.Stats["cache"])) / float64(memoryStats.Limit)
	}

	metrics = append(metrics, []*metricspb.Metric{
		GaugeF("memory.percent", []float64{pctUsed}, ts, "1", nil, nil),
		Gauge("memory.usage.max", []int64{int64(memoryStats.MaxUsage)}, ts, "By", nil, nil),
	}...)

	// Sorted iteration for reproducibility, largely for testing
	sortedNames := make([]string, 0, len(memoryStats.Stats))
	for statName := range memoryStats.Stats {
		sortedNames = append(sortedNames, statName)
	}
	sort.Strings(sortedNames)

	for _, statName := range sortedNames {
		v := memoryStats.Stats[statName]
		metricName := fmt.Sprintf("memory.%s", statName)
		if _, exists := memoryStatsThatAreCumulative[statName]; exists {
			metrics = append(metrics, Cumulative(metricName, []int64{int64(v)}, ts, "1", nil, nil))
		} else {
			metrics = append(metrics, Gauge(metricName, []int64{int64(v)}, ts, "By", nil, nil))
		}
	}

	return metrics
}

func networkMetrics(
	networks *map[string]dtypes.NetworkStats,
	ts *timestamp.Timestamp,
) []*metricspb.Metric {
	if networks == nil || *networks == nil {
		return nil
	}

	var metrics []*metricspb.Metric

	labelKeys := []string{"interface"}
	for nic, stats := range *networks {
		labelValues := [][]string{{nic}}

		metrics = append(metrics, []*metricspb.Metric{
			Cumulative("network.io.usage.rx_bytes", []int64{int64(stats.RxBytes)}, ts, "By", labelKeys, labelValues),
			Cumulative("network.io.usage.tx_bytes", []int64{int64(stats.TxBytes)}, ts, "By", labelKeys, labelValues),
		}...)

		metrics = append(metrics, []*metricspb.Metric{
			Cumulative("network.io.usage.rx_dropped", []int64{int64(stats.RxDropped)}, ts, "1", labelKeys, labelValues),
			Cumulative("network.io.usage.rx_errors", []int64{int64(stats.RxErrors)}, ts, "1", labelKeys, labelValues),
			Cumulative("network.io.usage.rx_packets", []int64{int64(stats.RxPackets)}, ts, "1", labelKeys, labelValues),
			Cumulative("network.io.usage.tx_dropped", []int64{int64(stats.TxDropped)}, ts, "1", labelKeys, labelValues),
			Cumulative("network.io.usage.tx_errors", []int64{int64(stats.TxErrors)}, ts, "1", labelKeys, labelValues),
			Cumulative("network.io.usage.tx_packets", []int64{int64(stats.TxPackets)}, ts, "1", labelKeys, labelValues),
		}...)
	}

	return metrics
}

func Cumulative(name string, vals []int64, ts *timestamp.Timestamp, unit string, labelKeys []string, labelValues [][]string) *metricspb.Metric {
	return metric(name, metricspb.MetricDescriptor_CUMULATIVE_INT64, ts, unit, labelKeys, labelValues, vals, nil)
}

func Gauge(name string, vals []int64, ts *timestamp.Timestamp, unit string, labelKeys []string, labelValues [][]string) *metricspb.Metric {
	return metric(name, metricspb.MetricDescriptor_GAUGE_INT64, ts, unit, labelKeys, labelValues, vals, nil)
}

func GaugeF(name string, vals []float64, ts *timestamp.Timestamp, unit string, labelKeys []string, labelValues [][]string) *metricspb.Metric {
	return metric(name, metricspb.MetricDescriptor_GAUGE_DOUBLE, ts, unit, labelKeys, labelValues, nil, vals)
}

func metric(
	name string,
	mdType metricspb.MetricDescriptor_Type,
	ts *timestamp.Timestamp,
	unit string,
	labelKeys []string,
	labelValues [][]string,
	intVals []int64,
	floatVals []float64,
) *metricspb.Metric {
	metric := &metricspb.Metric{}
	pts := points(intVals, floatVals, ts)
	lKeys, lValues := labelKeysAndValues(labelKeys, labelValues)

	metric.MetricDescriptor = &metricspb.MetricDescriptor{
		Name:      fmt.Sprintf("%s%s", metricPrefix, name),
		Type:      mdType,
		Unit:      unit,
		LabelKeys: lKeys,
	}

	metric.Timeseries = make([]*metricspb.TimeSeries, len(pts))
	for i, p := range pts {
		series := &metricspb.TimeSeries{
			Points: []*metricspb.Point{p},
		}

		if len(lValues) != 0 {
			series.LabelValues = lValues[i]
		}

		metric.Timeseries[i] = series
	}

	return metric
}

func points(
	intVals []int64,
	floatVals []float64,
	ts *timestamp.Timestamp,
) []*metricspb.Point {
	if intVals != nil {
		points := make([]*metricspb.Point, len(intVals))
		for i, v := range intVals {
			point := &metricspb.Point{
				Timestamp: ts,
				Value:     &metricspb.Point_Int64Value{Int64Value: v},
			}
			points[i] = point
		}
		return points
	}

	if floatVals != nil {
		points := make([]*metricspb.Point, len(floatVals))
		for i, v := range floatVals {
			point := &metricspb.Point{
				Timestamp: ts,
				Value:     &metricspb.Point_DoubleValue{DoubleValue: v},
			}
			points[i] = point
		}
		return points
	}

	return nil
}

func labelKeysAndValues(keys []string, values [][]string) ([]*metricspb.LabelKey, [][]*metricspb.LabelValue) {
	retKeys := make([]*metricspb.LabelKey, len(keys))
	retValues := make([][]*metricspb.LabelValue, len(values))
	for i, k := range keys {
		retKeys[i] = &metricspb.LabelKey{Key: k}
	}
	for i, vs := range values {
		retValues[i] = make([]*metricspb.LabelValue, len(vs))
		for j, v := range vs {
			retValues[i][j] = &metricspb.LabelValue{Value: v, HasValue: true}
		}
	}
	return retKeys, retValues
}
