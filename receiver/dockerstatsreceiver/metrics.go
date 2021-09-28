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

	dtypes "github.com/docker/docker/api/types"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

const (
	metricPrefix = "container."
)

func ContainerStatsToMetrics(
	now pdata.Timestamp,
	containerStats *dtypes.StatsJSON,
	container docker.Container,
	config *Config,
) (pdata.Metrics, error) {
	md := pdata.NewMetrics()
	rs := md.ResourceMetrics().AppendEmpty()
	rs.SetSchemaUrl(conventions.SchemaURL)
	resourceAttr := rs.Resource().Attributes()
	resourceAttr.UpsertString(conventions.AttributeContainerID, container.ID)
	resourceAttr.UpsertString(conventions.AttributeContainerImageName, container.Config.Image)
	resourceAttr.UpsertString(conventions.AttributeContainerName, strings.TrimPrefix(container.Name, "/"))
	resourceAttr.UpsertString("container.hostname", container.Config.Hostname)
	updateConfiguredResourceAttributes(resourceAttr, container, config)
	ils := rs.InstrumentationLibraryMetrics().AppendEmpty()

	appendBlockioMetrics(ils.Metrics(), &containerStats.BlkioStats, now)
	appendCPUMetrics(ils.Metrics(), &containerStats.CPUStats, &containerStats.PreCPUStats, now, config.ProvidePerCoreCPUMetrics)
	appendMemoryMetrics(ils.Metrics(), &containerStats.MemoryStats, now)
	appendNetworkMetrics(ils.Metrics(), &containerStats.Networks, now)

	return md, nil
}

func updateConfiguredResourceAttributes(resourceAttr pdata.AttributeMap, container docker.Container, config *Config) {
	for k, label := range config.EnvVarsToMetricLabels {
		if v := container.EnvMap[k]; v != "" {
			resourceAttr.UpsertString(label, v)
		}
	}

	for k, label := range config.ContainerLabelsToMetricLabels {
		if v := container.Config.Labels[k]; v != "" {
			resourceAttr.UpsertString(label, v)
		}
	}
}

type blkioStat struct {
	name    string
	unit    string
	entries []dtypes.BlkioStatEntry
}

// metrics for https://www.kernel.org/doc/Documentation/cgroup-v1/blkio-controller.txt
func appendBlockioMetrics(dest pdata.MetricSlice, blkioStats *dtypes.BlkioStats, ts pdata.Timestamp) {
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
			labelValues := []string{strconv.FormatUint(stat.Major, 10), strconv.FormatUint(stat.Minor, 10)}
			populateCumulative(dest.AppendEmpty(), metricName, blkiostat.unit, int64(stat.Value), ts, labelKeys, labelValues)
		}
	}
}

func appendCPUMetrics(dest pdata.MetricSlice, cpuStats *dtypes.CPUStats, previousCPUStats *dtypes.CPUStats, ts pdata.Timestamp, providePerCoreMetrics bool) {
	populateCumulative(dest.AppendEmpty(), "cpu.usage.system", "ns", int64(cpuStats.SystemUsage), ts, nil, nil)
	populateCumulative(dest.AppendEmpty(), "cpu.usage.total", "ns", int64(cpuStats.CPUUsage.TotalUsage), ts, nil, nil)

	populateCumulative(dest.AppendEmpty(), "cpu.usage.kernelmode", "ns", int64(cpuStats.CPUUsage.UsageInKernelmode), ts, nil, nil)
	populateCumulative(dest.AppendEmpty(), "cpu.usage.usermode", "ns", int64(cpuStats.CPUUsage.UsageInUsermode), ts, nil, nil)

	populateCumulative(dest.AppendEmpty(), "cpu.throttling_data.periods", "1", int64(cpuStats.ThrottlingData.Periods), ts, nil, nil)
	populateCumulative(dest.AppendEmpty(), "cpu.throttling_data.throttled_periods", "1", int64(cpuStats.ThrottlingData.ThrottledPeriods), ts, nil, nil)
	populateCumulative(dest.AppendEmpty(), "cpu.throttling_data.throttled_time", "ns", int64(cpuStats.ThrottlingData.ThrottledTime), ts, nil, nil)

	populateGaugeF(dest.AppendEmpty(), "cpu.percent", "1", calculateCPUPercent(previousCPUStats, cpuStats), ts, nil, nil)

	if !providePerCoreMetrics {
		return
	}

	percpuValues := make([]int64, 0, len(cpuStats.CPUUsage.PercpuUsage))
	percpuLabelKeys := []string{"core"}
	percpuLabelValues := make([][]string, 0, len(cpuStats.CPUUsage.PercpuUsage))
	for coreNum, v := range cpuStats.CPUUsage.PercpuUsage {
		percpuValues = append(percpuValues, int64(v))
		percpuLabelValues = append(percpuLabelValues, []string{fmt.Sprintf("cpu%s", strconv.Itoa(coreNum))})
	}
	populateCumulativeMultiPoints(dest.AppendEmpty(), "cpu.usage.percpu", "ns", percpuValues, ts, percpuLabelKeys, percpuLabelValues)
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

func appendMemoryMetrics(dest pdata.MetricSlice, memoryStats *dtypes.MemoryStats, ts pdata.Timestamp) {
	totalUsage := int64(memoryStats.Usage - memoryStats.Stats["total_cache"])
	populateGauge(dest.AppendEmpty(), "memory.usage.limit", int64(memoryStats.Limit), ts)
	populateGauge(dest.AppendEmpty(), "memory.usage.total", totalUsage, ts)

	var pctUsed float64
	if float64(memoryStats.Limit) == 0 {
		pctUsed = 0
	} else {
		pctUsed = 100.0 * (float64(memoryStats.Usage) - float64(memoryStats.Stats["cache"])) / float64(memoryStats.Limit)
	}

	populateGaugeF(dest.AppendEmpty(), "memory.percent", "1", pctUsed, ts, nil, nil)
	populateGauge(dest.AppendEmpty(), "memory.usage.max", int64(memoryStats.MaxUsage), ts)

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
			populateCumulative(dest.AppendEmpty(), metricName, "1", int64(v), ts, nil, nil)
		} else {
			populateGauge(dest.AppendEmpty(), metricName, int64(v), ts)
		}
	}
}

func appendNetworkMetrics(dest pdata.MetricSlice, networks *map[string]dtypes.NetworkStats, ts pdata.Timestamp) {
	if networks == nil || *networks == nil {
		return
	}

	labelKeys := []string{"interface"}
	for nic, stats := range *networks {
		labelValues := []string{nic}

		populateCumulative(dest.AppendEmpty(), "network.io.usage.rx_bytes", "By", int64(stats.RxBytes), ts, labelKeys, labelValues)
		populateCumulative(dest.AppendEmpty(), "network.io.usage.tx_bytes", "By", int64(stats.TxBytes), ts, labelKeys, labelValues)

		populateCumulative(dest.AppendEmpty(), "network.io.usage.rx_dropped", "1", int64(stats.RxDropped), ts, labelKeys, labelValues)
		populateCumulative(dest.AppendEmpty(), "network.io.usage.rx_errors", "1", int64(stats.RxErrors), ts, labelKeys, labelValues)
		populateCumulative(dest.AppendEmpty(), "network.io.usage.rx_packets", "1", int64(stats.RxPackets), ts, labelKeys, labelValues)
		populateCumulative(dest.AppendEmpty(), "network.io.usage.tx_dropped", "1", int64(stats.TxDropped), ts, labelKeys, labelValues)
		populateCumulative(dest.AppendEmpty(), "network.io.usage.tx_errors", "1", int64(stats.TxErrors), ts, labelKeys, labelValues)
		populateCumulative(dest.AppendEmpty(), "network.io.usage.tx_packets", "1", int64(stats.TxPackets), ts, labelKeys, labelValues)
	}
}

func populateCumulative(dest pdata.Metric, name string, unit string, val int64, ts pdata.Timestamp, labelKeys []string, labelValues []string) {
	populateMetricMetadata(dest, name, unit, pdata.MetricDataTypeSum)
	sum := dest.Sum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntVal(val)
	dp.SetTimestamp(ts)
	populateAttributes(dp.Attributes(), labelKeys, labelValues)
}

func populateCumulativeMultiPoints(dest pdata.Metric, name string, unit string, vals []int64, ts pdata.Timestamp, labelKeys []string, labelValues [][]string) {
	populateMetricMetadata(dest, name, unit, pdata.MetricDataTypeSum)
	sum := dest.Sum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	dps := sum.DataPoints()
	dps.EnsureCapacity(len(vals))
	for i := range vals {
		dp := dps.AppendEmpty()
		dp.SetIntVal(vals[i])
		dp.SetTimestamp(ts)
		populateAttributes(dp.Attributes(), labelKeys, labelValues[i])
	}
}

func populateGauge(dest pdata.Metric, name string, val int64, ts pdata.Timestamp) {
	// Unit, labelKeys, labelValues always constants, when that changes add them as argument to the func.
	populateMetricMetadata(dest, name, "By", pdata.MetricDataTypeGauge)
	sum := dest.Gauge()
	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntVal(val)
	dp.SetTimestamp(ts)
	populateAttributes(dp.Attributes(), nil, nil)
}

func populateGaugeF(dest pdata.Metric, name string, unit string, val float64, ts pdata.Timestamp, labelKeys []string, labelValues []string) {
	populateMetricMetadata(dest, name, unit, pdata.MetricDataTypeGauge)
	sum := dest.Gauge()
	dp := sum.DataPoints().AppendEmpty()
	dp.SetDoubleVal(val)
	dp.SetTimestamp(ts)
	populateAttributes(dp.Attributes(), labelKeys, labelValues)
}

func populateMetricMetadata(dest pdata.Metric, name string, unit string, ty pdata.MetricDataType) {
	dest.SetName(metricPrefix + name)
	dest.SetUnit(unit)
	dest.SetDataType(ty)
}

func populateAttributes(dest pdata.AttributeMap, labelKeys []string, labelValues []string) {
	for i := range labelKeys {
		dest.UpsertString(labelKeys[i], labelValues[i])
	}
}
