// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	dtypes "github.com/docker/docker/api/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

const defaultResourcesLen = 5

const (
	defaultDockerAPIVersion         = 1.25
	minimalRequiredDockerAPIVersion = 1.25
)

type resultV2 struct {
	stats     *dtypes.StatsJSON
	container *docker.Container
	err       error
}

type receiver struct {
	config   *Config
	settings rcvr.CreateSettings
	client   *docker.Client
	mb       *metadata.MetricsBuilder
}

func newReceiver(set rcvr.CreateSettings, config *Config) *receiver {
	return &receiver{
		config:   config,
		settings: set,
		mb:       metadata.NewMetricsBuilder(config.MetricsBuilderConfig, set),
	}
}

func (r *receiver) start(ctx context.Context, _ component.Host) error {
	dConfig, err := docker.NewConfig(r.config.Endpoint, r.config.Timeout, r.config.ExcludedImages, r.config.DockerAPIVersion)
	if err != nil {
		return err
	}

	r.client, err = docker.NewDockerClient(dConfig, r.settings.Logger)
	if err != nil {
		return err
	}

	if err = r.client.LoadContainerList(ctx); err != nil {
		return err
	}

	go r.client.ContainerEventLoop(ctx)
	return nil
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
	for res := range results {
		if res.err != nil {
			// Don't know the number of failed stats, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			continue
		}
		if err := r.recordContainerStats(now, res.stats, res.container); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return r.mb.Emit(), errs
}

func (r *receiver) recordContainerStats(now pcommon.Timestamp, containerStats *dtypes.StatsJSON, container *docker.Container) error {
	var errs error
	r.recordCPUMetrics(now, &containerStats.CPUStats, &containerStats.PreCPUStats)
	r.recordMemoryMetrics(now, &containerStats.MemoryStats)
	r.recordBlkioMetrics(now, &containerStats.BlkioStats)
	r.recordNetworkMetrics(now, &containerStats.Networks)
	r.recordPidsMetrics(now, &containerStats.PidsStats)
	if err := r.recordBaseMetrics(now, container.ContainerJSONBase); err != nil {
		errs = multierr.Append(errs, err)
	}
	r.recordHostConfigMetrics(now, container.ContainerJSON)
	r.mb.RecordContainerRestartsDataPoint(now, int64(container.RestartCount))

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
		label := label
		if v := container.EnvMap[k]; v != "" {
			resourceMetricsOptions = append(resourceMetricsOptions, func(ras metadata.ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
				rm.Resource().Attributes().PutStr(label, v)
			})
		}
	}
	for k, label := range r.config.ContainerLabelsToMetricLabels {
		label := label
		if v := container.Config.Labels[k]; v != "" {
			resourceMetricsOptions = append(resourceMetricsOptions, func(ras metadata.ResourceAttributesConfig, rm pmetric.ResourceMetrics) {
				rm.Resource().Attributes().PutStr(label, v)
			})
		}
	}

	r.mb.EmitForResource(resourceMetricsOptions...)
	return errs
}

func (r *receiver) recordMemoryMetrics(now pcommon.Timestamp, memoryStats *dtypes.MemoryStats) {
	totalUsage := calculateMemUsageNoCache(memoryStats)
	r.mb.RecordContainerMemoryUsageTotalDataPoint(now, int64(totalUsage))

	r.mb.RecordContainerMemoryUsageLimitDataPoint(now, int64(memoryStats.Limit))

	r.mb.RecordContainerMemoryPercentDataPoint(now, calculateMemoryPercent(memoryStats.Limit, totalUsage))

	r.mb.RecordContainerMemoryUsageMaxDataPoint(now, int64(memoryStats.MaxUsage))

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

type blkioRecorder func(now pcommon.Timestamp, val int64, devMaj string, devMin string, operation string)

func (r *receiver) recordBlkioMetrics(now pcommon.Timestamp, blkioStats *dtypes.BlkioStats) {
	recordSingleBlkioStat(now, blkioStats.IoMergedRecursive, r.mb.RecordContainerBlockioIoMergedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoQueuedRecursive, r.mb.RecordContainerBlockioIoQueuedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServiceBytesRecursive, r.mb.RecordContainerBlockioIoServiceBytesRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServiceTimeRecursive, r.mb.RecordContainerBlockioIoServiceTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServicedRecursive, r.mb.RecordContainerBlockioIoServicedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoTimeRecursive, r.mb.RecordContainerBlockioIoTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoWaitTimeRecursive, r.mb.RecordContainerBlockioIoWaitTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.SectorsRecursive, r.mb.RecordContainerBlockioSectorsRecursiveDataPoint)
}

func recordSingleBlkioStat(now pcommon.Timestamp, statEntries []dtypes.BlkioStatEntry, recorder blkioRecorder) {
	for _, stat := range statEntries {
		recorder(
			now,
			int64(stat.Value),
			strconv.FormatUint(stat.Major, 10),
			strconv.FormatUint(stat.Minor, 10),
			strings.ToLower(stat.Op))
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
	r.mb.RecordContainerCPUUtilizationDataPoint(now, calculateCPUPercent(prevStats, cpuStats))
	r.mb.RecordContainerCPUPercentDataPoint(now, calculateCPUPercent(prevStats, cpuStats))

	for coreNum, v := range cpuStats.CPUUsage.PercpuUsage {
		r.mb.RecordContainerCPUUsagePercpuDataPoint(now, int64(v), fmt.Sprintf("cpu%s", strconv.Itoa(coreNum)))
	}
}

func (r *receiver) recordPidsMetrics(now pcommon.Timestamp, pidsStats *dtypes.PidsStats) {
	// pidsStats are available when kernel version is >= 4.3 and pids_cgroup is supported, it is empty otherwise.
	if pidsStats.Current != 0 {
		r.mb.RecordContainerPidsCountDataPoint(now, int64(pidsStats.Current))
		if pidsStats.Limit != 0 {
			r.mb.RecordContainerPidsLimitDataPoint(now, int64(pidsStats.Limit))
		}
	}
}

func (r *receiver) recordBaseMetrics(now pcommon.Timestamp, base *types.ContainerJSONBase) error {
	t, err := time.Parse(time.RFC3339, base.State.StartedAt)
	if err != nil {
		// value not available or invalid
		return scrapererror.NewPartialScrapeError(fmt.Errorf("error retrieving container.uptime from Container.State.StartedAt: %w", err), 1)
	}
	if v := now.AsTime().Sub(t); v > 0 {
		r.mb.RecordContainerUptimeDataPoint(now, v.Seconds())
	}
	return nil
}

func (r *receiver) recordHostConfigMetrics(now pcommon.Timestamp, containerJSON *dtypes.ContainerJSON) {
	r.mb.RecordContainerCPULimitDataPoint(now, calculateCPULimit(containerJSON.HostConfig))
	r.mb.RecordContainerCPUSharesDataPoint(now, containerJSON.HostConfig.CPUShares)
}
