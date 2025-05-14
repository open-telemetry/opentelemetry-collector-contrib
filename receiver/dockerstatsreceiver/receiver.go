// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	ctypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

// Matcher is an interface for matching container labels.
type Matcher interface {
	Matches(label string) bool
}

// StrictMatcher is a matcher that matches by exact match.
type StrictMatcher struct {
	Include string
}

// Matches returns true if the matcher matches the label.
func (sm *StrictMatcher) Matches(label string) bool {
	return label == sm.Include
}

// RegexMatcher is a matcher that matches by regular expression.
type RegexMatcher struct {
	re *regexp.Regexp
}

// Matches returns true if the matcher matches the label.
func (rm *RegexMatcher) Matches(label string) bool {
	return rm.re.MatchString(label)
}

// MatcherCorpus is a collection of matchers.
type MatcherCorpus struct {
	matchers []Matcher
}

// Add adds a matcher to the corpus.
func (mc *MatcherCorpus) Add(lm LabelMatcher) error {
	switch lm.MatchType {
	case strictMatchType:
		mc.matchers = append(mc.matchers, &StrictMatcher{Include: lm.Include})
	case regexpMatchType:
		re, err := regexp.Compile(lm.Include)
		if err != nil {
			return fmt.Errorf("failed to compile regex from include '%v': %w", lm.Include, err)
		}
		mc.matchers = append(mc.matchers, &RegexMatcher{re: re})
	default:
		return fmt.Errorf("unknown match type: %v", lm.MatchType)
	}
	return nil
}

// MatcherCorpusFromConfig creates a MatcherCorpus from a Config.
func MatcherCorpusFromConfig(config *Config) (*MatcherCorpus, error) {
	mc := &MatcherCorpus{}
	for _, lm := range config.ContainerLabelsToResourceAttributes {
		if err := mc.Add(lm); err != nil {
			return nil, err
		}
	}
	return mc, nil
}

// Matches returns true if any of the matchers in the corpus matches the label.
func (mc *MatcherCorpus) Matches(label string) bool {
	for _, matcher := range mc.matchers {
		if matcher.Matches(label) {
			return true
		}
	}
	return false
}

// IsEmpty returns true if the corpus is empty.
func (mc *MatcherCorpus) IsEmpty() bool {
	return mc == nil || len(mc.matchers) == 0
}

var (
	defaultDockerAPIVersion         = "1.25"
	minimumRequiredDockerAPIVersion = docker.MustNewAPIVersion(defaultDockerAPIVersion)
)

type resultV2 struct {
	stats     *ctypes.StatsResponse
	container *docker.Container
	err       error
}

type metricsReceiver struct {
	config   *Config
	settings receiver.Settings
	client   *docker.Client
	mb       *metadata.MetricsBuilder
	lm       *MatcherCorpus
	cancel   context.CancelFunc
}

func newMetricsReceiver(set receiver.Settings, config *Config) *metricsReceiver {
	lm, err := MatcherCorpusFromConfig(config)
	if err != nil {
		// This can practically never happen due to config validation.
		set.Logger.Warn("Failed to parse include regexes for labels from config.", zap.Error(err))
	}
	return &metricsReceiver{
		lm:       lm,
		config:   config,
		settings: set,
		mb:       metadata.NewMetricsBuilder(config.MetricsBuilderConfig, set),
	}
}

func (r *metricsReceiver) clientOptions() []client.Opt {
	var opts []client.Opt
	if r.config.Endpoint == "" {
		opts = append(opts, client.WithHostFromEnv())
	}
	return opts
}

func (r *metricsReceiver) start(ctx context.Context, _ component.Host) error {
	var err error
	r.client, err = docker.NewDockerClient(&r.config.Config, r.settings.Logger, r.clientOptions()...)
	if err != nil {
		return err
	}

	if err = r.client.LoadContainerList(ctx); err != nil {
		return err
	}

	cctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	go r.client.ContainerEventLoop(cctx)
	return nil
}

func (r *metricsReceiver) shutdown(context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

func (r *metricsReceiver) scrapeV2(ctx context.Context) (pmetric.Metrics, error) {
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
				err:       nil,
			}
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

func (r *metricsReceiver) recordContainerStats(now pcommon.Timestamp, containerStats *ctypes.StatsResponse, container *docker.Container) error {
	var errs error
	r.recordCPUMetrics(now, &containerStats.CPUStats, &containerStats.PreCPUStats)
	r.recordMemoryMetrics(now, &containerStats.MemoryStats)
	r.recordBlkioMetrics(now, &containerStats.BlkioStats)
	r.recordNetworkMetrics(now, &containerStats.Networks)
	r.recordPidsMetrics(now, &containerStats.PidsStats)
	if err := r.recordBaseMetrics(now, container.ContainerJSONBase); err != nil {
		errs = multierr.Append(errs, err)
	}
	if err := r.recordHostConfigMetrics(now, container.ContainerJSON); err != nil {
		errs = multierr.Append(errs, err)
	}
	r.mb.RecordContainerRestartsDataPoint(now, int64(container.RestartCount))

	// Always-present resource attrs + the user-configured resource attrs
	rb := r.mb.NewResourceBuilder()
	rb.SetContainerRuntime("docker")
	rb.SetContainerHostname(container.Config.Hostname)
	rb.SetContainerID(container.ID)
	rb.SetContainerImageName(container.Config.Image)
	rb.SetContainerName(strings.TrimPrefix(container.Name, "/"))
	rb.SetContainerImageID(container.Image)
	rb.SetContainerCommandLine(strings.Join(container.Config.Cmd, " "))

	// Copy container labels into the resource attributes if they match the configured regexes.
	var l map[string]any
	if !r.lm.IsEmpty() {
		l = make(map[string]any, len(container.Config.Labels))
		for k, v := range container.Config.Labels {
			if r.lm.Matches(k) {
				l[k] = v
			}
		}
	}
	rb.SetContainerLabels(l)
	resource := rb.Emit()

	for k, label := range r.config.EnvVarsToMetricLabels {
		if v := container.EnvMap[k]; v != "" {
			resource.Attributes().PutStr(label, v)
		}
	}
	for k, label := range r.config.ContainerLabelsToMetricLabels {
		if v := container.Config.Labels[k]; v != "" {
			resource.Attributes().PutStr(label, v)
		}
	}

	r.mb.EmitForResource(metadata.WithResource(resource))
	return errs
}

func (r *metricsReceiver) recordMemoryMetrics(now pcommon.Timestamp, memoryStats *ctypes.MemoryStats) {
	totalUsage := calculateMemUsageNoCache(memoryStats)
	r.mb.RecordContainerMemoryUsageTotalDataPoint(now, int64(totalUsage))

	r.mb.RecordContainerMemoryUsageLimitDataPoint(now, int64(memoryStats.Limit))

	r.mb.RecordContainerMemoryPercentDataPoint(now, calculateMemoryPercent(memoryStats.Limit, totalUsage))

	r.mb.RecordContainerMemoryUsageMaxDataPoint(now, int64(memoryStats.MaxUsage))

	r.mb.RecordContainerMemoryFailsDataPoint(now, int64(memoryStats.Failcnt))

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
		"anon":                      r.mb.RecordContainerMemoryAnonDataPoint,
		"file":                      r.mb.RecordContainerMemoryFileDataPoint,
	}

	for name, val := range memoryStats.Stats {
		if recorder, ok := recorders[name]; ok {
			recorder(now, int64(val))
		}
	}
}

type blkioRecorder func(now pcommon.Timestamp, val int64, devMaj string, devMin string, operation string)

func (r *metricsReceiver) recordBlkioMetrics(now pcommon.Timestamp, blkioStats *ctypes.BlkioStats) {
	recordSingleBlkioStat(now, blkioStats.IoMergedRecursive, r.mb.RecordContainerBlockioIoMergedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoQueuedRecursive, r.mb.RecordContainerBlockioIoQueuedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServiceBytesRecursive, r.mb.RecordContainerBlockioIoServiceBytesRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServiceTimeRecursive, r.mb.RecordContainerBlockioIoServiceTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoServicedRecursive, r.mb.RecordContainerBlockioIoServicedRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoTimeRecursive, r.mb.RecordContainerBlockioIoTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.IoWaitTimeRecursive, r.mb.RecordContainerBlockioIoWaitTimeRecursiveDataPoint)
	recordSingleBlkioStat(now, blkioStats.SectorsRecursive, r.mb.RecordContainerBlockioSectorsRecursiveDataPoint)
}

func recordSingleBlkioStat(now pcommon.Timestamp, statEntries []ctypes.BlkioStatEntry, recorder blkioRecorder) {
	for _, stat := range statEntries {
		recorder(
			now,
			int64(stat.Value),
			strconv.FormatUint(stat.Major, 10),
			strconv.FormatUint(stat.Minor, 10),
			strings.ToLower(stat.Op))
	}
}

func (r *metricsReceiver) recordNetworkMetrics(now pcommon.Timestamp, networks *map[string]ctypes.NetworkStats) {
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

func (r *metricsReceiver) recordCPUMetrics(now pcommon.Timestamp, cpuStats *ctypes.CPUStats, prevStats *ctypes.CPUStats) {
	r.mb.RecordContainerCPUUsageSystemDataPoint(now, int64(cpuStats.SystemUsage))
	r.mb.RecordContainerCPUUsageTotalDataPoint(now, int64(cpuStats.CPUUsage.TotalUsage))
	r.mb.RecordContainerCPUUsageKernelmodeDataPoint(now, int64(cpuStats.CPUUsage.UsageInKernelmode))
	r.mb.RecordContainerCPUUsageUsermodeDataPoint(now, int64(cpuStats.CPUUsage.UsageInUsermode))
	r.mb.RecordContainerCPUThrottlingDataThrottledPeriodsDataPoint(now, int64(cpuStats.ThrottlingData.ThrottledPeriods))
	r.mb.RecordContainerCPUThrottlingDataPeriodsDataPoint(now, int64(cpuStats.ThrottlingData.Periods))
	r.mb.RecordContainerCPUThrottlingDataThrottledTimeDataPoint(now, int64(cpuStats.ThrottlingData.ThrottledTime))
	r.mb.RecordContainerCPUUtilizationDataPoint(now, calculateCPUPercent(prevStats, cpuStats))
	r.mb.RecordContainerCPULogicalCountDataPoint(now, int64(cpuStats.OnlineCPUs))

	for coreNum, v := range cpuStats.CPUUsage.PercpuUsage {
		r.mb.RecordContainerCPUUsagePercpuDataPoint(now, int64(v), "cpu"+strconv.Itoa(coreNum))
	}
}

func (r *metricsReceiver) recordPidsMetrics(now pcommon.Timestamp, pidsStats *ctypes.PidsStats) {
	// pidsStats are available when kernel version is >= 4.3 and pids_cgroup is supported, it is empty otherwise.
	if pidsStats.Current != 0 {
		r.mb.RecordContainerPidsCountDataPoint(now, int64(pidsStats.Current))
		if pidsStats.Limit != 0 {
			r.mb.RecordContainerPidsLimitDataPoint(now, int64(pidsStats.Limit))
		}
	}
}

func (r *metricsReceiver) recordBaseMetrics(now pcommon.Timestamp, base *ctypes.ContainerJSONBase) error {
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

func (r *metricsReceiver) recordHostConfigMetrics(now pcommon.Timestamp, containerJSON *ctypes.InspectResponse) error {
	r.mb.RecordContainerCPUSharesDataPoint(now, containerJSON.HostConfig.CPUShares)

	cpuLimit, err := calculateCPULimit(containerJSON.HostConfig)
	if err != nil {
		return scrapererror.NewPartialScrapeError(fmt.Errorf("error retrieving container.cpu.limit: %w", err), 1)
	}
	if cpuLimit > 0 {
		r.mb.RecordContainerCPULimitDataPoint(now, cpuLimit)
	}
	return nil
}
