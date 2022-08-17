// Copyright  The OpenTelemetry Authors
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

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	dtypes "github.com/docker/docker/api/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

const defaultResourcesLen = 5

type resultV2 struct {
	stats     *dtypes.StatsJSON
	container *docker.Container
	err       error
}

func (r *receiver) scrapeV2(ctx context.Context) (pmetric.Metrics, error) {
	containers := r.client.Containers()
	results := make(chan resultV2, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, container := range containers {
		go func(c docker.Container) {
			defer wg.Done()
			statsJSON, err := r.client.FetchContainerStatsAsJSON(ctx, c)
			if err != nil {
				results <- resultV2{nil, &c, err}
				return
			}

			results <- resultV2{
				stats:     statsJSON,
				container: &c,
				err:       nil}
		}(container)
	}

	wg.Wait()
	close(results)

	var errs error

	now := pcommon.NewTimestampFromTime(time.Now())
	md := pmetric.NewMetrics()
	for res := range results {
		if res.err != nil {
			// Don't know the number of failed stats, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			continue
		}
		r.recordContainerStats(now, res.stats, res.container).ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	}

	return md, errs
}

func (r *receiver) recordContainerStats(now pcommon.Timestamp, containerStats *dtypes.StatsJSON, container *docker.Container) pmetric.Metrics {
	r.recordCPUMetrics(now, &containerStats.CPUStats, &containerStats.PreCPUStats)
	r.recordMemoryMetrics(now, &containerStats.MemoryStats)
	r.recordBlkioMetrics(now, &containerStats.BlkioStats)
	r.recordNetworkMetrics(now, &containerStats.Networks)

	// Always-present resource attrs + the user-configured resource attrs
	resourceCapacity := defaultResourcesLen + len(r.config.EnvVarsToMetricLabels) + len(r.config.ContainerLabelsToMetricLabels)
	resourceMetricsOptions := make([]metadata.ResourceMetricsOption, 0, resourceCapacity)
	resourceMetricsOptions = append(resourceMetricsOptions,
		metadata.WithContainerRuntime("docker"),
		metadata.WithContainerHostname(container.Config.Hostname),
		metadata.WithContainerID(container.ID),
		metadata.WithContainerImageName(container.Config.Image),
		metadata.WithContainerName(strings.TrimPrefix(container.Name, "/")))

	for k, label := range r.config.EnvVarsToMetricLabels {
		if v := container.EnvMap[k]; v != "" {
			resourceMetricsOptions = append(resourceMetricsOptions, func(rm pmetric.ResourceMetrics) {
				rm.Resource().Attributes().UpsertString(label, v)
			})
		}
	}
	for k, label := range r.config.ContainerLabelsToMetricLabels {
		if v := container.Config.Labels[k]; v != "" {
			resourceMetricsOptions = append(resourceMetricsOptions, func(rm pmetric.ResourceMetrics) {
				rm.Resource().Attributes().UpsertString(label, v)
			})
		}
	}

	return r.mb.Emit(resourceMetricsOptions...)
}

func (r *receiver) recordMemoryMetrics(now pcommon.Timestamp, memoryStats *dtypes.MemoryStats) {
	totalCache := memoryStats.Stats["total_cache"]
	totalUsage := memoryStats.Usage - totalCache
	r.mb.RecordContainerMemoryUsageMaxDataPoint(now, int64(memoryStats.MaxUsage))
	r.mb.RecordContainerMemoryPercentDataPoint(now, calculateMemoryPercent(memoryStats))
	r.mb.RecordContainerMemoryUsageTotalDataPoint(now, int64(totalUsage))
	r.mb.RecordContainerMemoryUsageLimitDataPoint(now, int64(memoryStats.Limit))

	recorders := map[string]func(pcommon.Timestamp, int64){
		"cache":                     r.mb.RecordContainerMemoryCacheDataPoint,
		"total_cache":               r.mb.RecordContainerMemoryTotalCacheDataPoint,
		"rss":                       r.mb.RecordContainerMemoryRssDataPoint,
		"total_rss":                 r.mb.RecordContainerMemoryTotalRssDataPoint,
		"rss_huge":                  r.mb.RecordContainerMemoryRssHugeDataPoint,
		"total_rss_huge":            r.mb.RecordContainerMemoryTotalRssHugeDataPoint,
		"dirty":                     r.mb.RecordContainerMemoryDirtyDataPoint,
		"total_dirty":               r.mb.RecordContainerMemoryTotalDirtyDataPoint,
		"writeback":                 r.mb.RecordContainerMemoryWritebackDataPoint,
		"total_writeback":           r.mb.RecordContainerMemoryTotalWritebackDataPoint,
		"mapped_file":               r.mb.RecordContainerMemoryMappedFileDataPoint,
		"total_mapped_file":         r.mb.RecordContainerMemoryTotalMappedFileDataPoint,
		"pgpgin":                    r.mb.RecordContainerMemoryPgpginDataPoint,
		"total_pgpgin":              r.mb.RecordContainerMemoryTotalPgpginDataPoint,
		"pgpgout":                   r.mb.RecordContainerMemoryPgpgoutDataPoint,
		"total_pgpgout":             r.mb.RecordContainerMemoryTotalPgpgoutDataPoint,
		"swap":                      r.mb.RecordContainerMemorySwapDataPoint,
		"total_swap":                r.mb.RecordContainerMemoryTotalSwapDataPoint,
		"pgfault":                   r.mb.RecordContainerMemoryPgfaultDataPoint,
		"total_pgfault":             r.mb.RecordContainerMemoryTotalPgfaultDataPoint,
		"pgmajfault":                r.mb.RecordContainerMemoryPgmajfaultDataPoint,
		"total_pgmajfault":          r.mb.RecordContainerMemoryTotalPgmajfaultDataPoint,
		"inactive_anon":             r.mb.RecordContainerMemoryInactiveAnonDataPoint,
		"total_inactive_anon":       r.mb.RecordContainerMemoryTotalInactiveAnonDataPoint,
		"active_anon":               r.mb.RecordContainerMemoryActiveAnonDataPoint,
		"total_active_anon":         r.mb.RecordContainerMemoryTotalActiveAnonDataPoint,
		"inactive_file":             r.mb.RecordContainerMemoryInactiveFileDataPoint,
		"total_inactive_file":       r.mb.RecordContainerMemoryTotalInactiveFileDataPoint,
		"active_file":               r.mb.RecordContainerMemoryActiveFileDataPoint,
		"total_active_file":         r.mb.RecordContainerMemoryTotalActiveFileDataPoint,
		"unevictable":               r.mb.RecordContainerMemoryUnevictableDataPoint,
		"total_unevictable":         r.mb.RecordContainerMemoryTotalUnevictableDataPoint,
		"hierarchical_memory_limit": r.mb.RecordContainerMemoryHierarchicalMemoryLimitDataPoint,
		"hierarchical_memsw_limit":  r.mb.RecordContainerMemoryHierarchicalMemswLimitDataPoint,
	}

	for name, val := range memoryStats.Stats {
		if recorder, ok := recorders[name]; ok {
			recorder(now, int64(val))
		}
	}
}

type blkioRecorder func(now pcommon.Timestamp, val int64, devMaj string, devMin string)

type blkioMapper struct {
	opToRecorderMap map[string]blkioRecorder
	entries         []dtypes.BlkioStatEntry
}

func (r *receiver) recordBlkioMetrics(now pcommon.Timestamp, blkioStats *dtypes.BlkioStats) {
	// These maps can be avoided once the operation is changed to an attribute instead of being in the metric name
	for _, blkioRecorder := range []blkioMapper{
		{entries: blkioStats.IoMergedRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioIoMergedRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioIoMergedRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioIoMergedRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioIoMergedRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioIoMergedRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioIoMergedRecursiveTotalDataPoint,
		}},
		{entries: blkioStats.IoQueuedRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioIoQueuedRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioIoQueuedRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioIoQueuedRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioIoQueuedRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioIoQueuedRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioIoQueuedRecursiveTotalDataPoint,
		}},
		{entries: blkioStats.IoServiceBytesRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioIoServiceBytesRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioIoServiceBytesRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioIoServiceBytesRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioIoServiceBytesRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioIoServiceBytesRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioIoServiceBytesRecursiveTotalDataPoint,
		}},
		{entries: blkioStats.IoServiceTimeRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioIoServiceTimeRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioIoServiceTimeRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioIoServiceTimeRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioIoServiceTimeRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioIoServiceTimeRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioIoServiceTimeRecursiveTotalDataPoint,
		}},
		{entries: blkioStats.IoServicedRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioIoServicedRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioIoServicedRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioIoServicedRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioIoServicedRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioIoServicedRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioIoServicedRecursiveTotalDataPoint,
		}},
		{entries: blkioStats.IoTimeRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioIoTimeRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioIoTimeRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioIoTimeRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioIoTimeRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioIoTimeRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioIoTimeRecursiveTotalDataPoint,
		}},
		{entries: blkioStats.IoWaitTimeRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioIoWaitTimeRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioIoWaitTimeRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioIoWaitTimeRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioIoWaitTimeRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioIoWaitTimeRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioIoWaitTimeRecursiveTotalDataPoint,
		}},
		{entries: blkioStats.SectorsRecursive, opToRecorderMap: map[string]blkioRecorder{
			"read":    r.mb.RecordContainerBlockioSectorsRecursiveReadDataPoint,
			"write":   r.mb.RecordContainerBlockioSectorsRecursiveWriteDataPoint,
			"sync":    r.mb.RecordContainerBlockioSectorsRecursiveSyncDataPoint,
			"async":   r.mb.RecordContainerBlockioSectorsRecursiveAsyncDataPoint,
			"discard": r.mb.RecordContainerBlockioSectorsRecursiveDiscardDataPoint,
			"total":   r.mb.RecordContainerBlockioSectorsRecursiveTotalDataPoint,
		}},
	} {
		for _, entry := range blkioRecorder.entries {
			recorder, ok := blkioRecorder.opToRecorderMap[strings.ToLower(entry.Op)]
			if !ok {
				r.settings.Logger.Debug("Unknown operation in blockIO stats.", zap.String("operation", entry.Op))
				continue
			}
			recorder(now, int64(entry.Value), strconv.FormatUint(entry.Major, 10), strconv.FormatUint(entry.Minor, 10))
		}

	}
}

func (r *receiver) recordNetworkMetrics(now pcommon.Timestamp, networks *map[string]dtypes.NetworkStats) {
	if networks == nil || *networks == nil {
		return
	}

	for netInterface, stats := range *networks {
		r.mb.RecordContainerNetworkIoUsageRxBytesDataPoint(now, int64(stats.RxBytes), netInterface)
		r.mb.RecordContainerNetworkIoUsageTxBytesDataPoint(now, int64(stats.TxBytes), netInterface)
		r.mb.RecordContainerNetworkIoUsageRxDroppedDataPoint(now, int64(stats.RxDropped), netInterface)
		r.mb.RecordContainerNetworkIoUsageTxDroppedDataPoint(now, int64(stats.TxDropped), netInterface)
		r.mb.RecordContainerNetworkIoUsageRxPacketsDataPoint(now, int64(stats.RxPackets), netInterface)
		r.mb.RecordContainerNetworkIoUsageTxPacketsDataPoint(now, int64(stats.TxPackets), netInterface)
		r.mb.RecordContainerNetworkIoUsageRxErrorsDataPoint(now, int64(stats.RxErrors), netInterface)
		r.mb.RecordContainerNetworkIoUsageTxErrorsDataPoint(now, int64(stats.TxErrors), netInterface)
	}
}

func (r *receiver) recordCPUMetrics(now pcommon.Timestamp, cpuStats *dtypes.CPUStats, prevStats *dtypes.CPUStats) {
	r.mb.RecordContainerCPUUsageSystemDataPoint(now, int64(cpuStats.SystemUsage))
	r.mb.RecordContainerCPUUsageTotalDataPoint(now, int64(cpuStats.CPUUsage.TotalUsage))
	r.mb.RecordContainerCPUUsageKernelmodeDataPoint(now, int64(cpuStats.CPUUsage.UsageInKernelmode))
	r.mb.RecordContainerCPUUsageUsermodeDataPoint(now, int64(cpuStats.CPUUsage.UsageInUsermode))
	r.mb.RecordContainerCPUThrottlingDataThrottledPeriodsDataPoint(now, int64(cpuStats.ThrottlingData.ThrottledPeriods))
	r.mb.RecordContainerCPUThrottlingDataPeriodsDataPoint(now, int64(cpuStats.ThrottlingData.Periods))
	r.mb.RecordContainerCPUThrottlingDataThrottledTimeDataPoint(now, int64(cpuStats.ThrottlingData.ThrottledTime))
	r.mb.RecordContainerCPUPercentDataPoint(now, calculateCPUPercent(prevStats, cpuStats))

	for coreNum, v := range cpuStats.CPUUsage.PercpuUsage {
		r.mb.RecordContainerCPUUsagePercpuDataPoint(now, int64(v), fmt.Sprintf("cpu%s", strconv.Itoa(coreNum)))
	}
}
