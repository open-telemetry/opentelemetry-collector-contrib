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

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	dtypes "github.com/docker/docker/api/types"
	dclient "github.com/docker/docker/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

const (
	defaultDockerAPIVersion         = 1.22
	minimalRequiredDockerAPIVersion = 1.22
)

type receiver struct {
	config   *Config
	settings component.ReceiverCreateSettings
	client   *docker.Client
	mb       *metadata.MetricsBuilder
}

func newReceiver(set component.ReceiverCreateSettings, config *Config) *receiver {
	return &receiver{
		config:   config,
		settings: set,
		mb:       metadata.NewMetricsBuilder(config.MetricsConfig, set.BuildInfo),
	}
}

func (r *receiver) start(ctx context.Context, _ component.Host) error {
	dConfig, err := docker.NewConfig(r.config.Endpoint, r.config.Timeout, r.config.ExcludedImages, r.config.DockerAPIVersion)
	if err != nil {
		return err
	}

	// Restore the default dialer due to bug in v0.4.1 of go-connections/sockets.go
	// which overrides the proto, and doesn't reset it when a new scheme is applied
	var opts []dclient.Opt
	if strings.HasPrefix(dConfig.Endpoint, "http") {
		dialer := &net.Dialer{
			Timeout: r.config.Timeout,
		}
		opts = append(opts, dclient.WithDialContext(dialer.DialContext))
	}
	r.client, err = docker.NewDockerClient(dConfig, r.settings.Logger, opts...)
	if err != nil {
		return err
	}

	if err = r.client.LoadContainerList(ctx); err != nil {
		return err
	}

	go r.client.ContainerEventLoop(ctx)
	return nil
}

type result struct {
	md  pmetric.Metrics
	err error
}

func (r *receiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	containers := r.client.Containers()
	results := make(chan result, len(containers))

	wg := &sync.WaitGroup{}
	wg.Add(len(containers))
	for _, container := range containers {
		go func(c docker.Container) {
			defer wg.Done()
			statsJSON, err := r.client.FetchContainerStatsAsJSON(ctx, c)
			if err != nil {
				results <- result{md: pmetric.Metrics{}, err: err}
				return
			}

			results <- result{
				md:  ContainerStatsToMetrics(pcommon.NewTimestampFromTime(time.Now()), statsJSON, c, r.config),
				err: nil}
		}(container)
	}

	wg.Wait()
	close(results)

	var errs error
	md := pmetric.NewMetrics()
	for res := range results {
		if res.err != nil {
			// Don't know the number of failed stats, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			continue
		}
		res.md.ResourceMetrics().CopyTo(md.ResourceMetrics())
	}

	return md, errs
}

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

		// todo verify this with multiple containers
		r.recordContainerStats(now, res.stats, res.container).ResourceMetrics().CopyTo(md.ResourceMetrics())
	}

	return md, errs
}

func (r *receiver) recordContainerStats(now pcommon.Timestamp, containerStats *dtypes.StatsJSON, container *docker.Container) pmetric.Metrics {

	r.recordMemoryMetrics(now, &containerStats.MemoryStats)
	r.recordBlkioMetrics(now, &containerStats.BlkioStats)

	// Five always-present resource attrs + the user-configured resource attrs
	resourceCapacity := 5 + len(r.config.EnvVarsToMetricLabels) + len(r.config.ContainerLabelsToMetricLabels)
	resourceMetricsOptions := make([]metadata.ResourceMetricsOption, 0, resourceCapacity)
	resourceMetricsOptions = append(resourceMetricsOptions,
		metadata.WithContainerRuntime("docker"),
		metadata.WithContainerHostname(container.Config.Hostname),
		metadata.WithContainerID(container.ID),
		metadata.WithContainerImageName(container.Config.Image),
		metadata.WithContainerName(strings.TrimPrefix(container.Name, "/")))

	for k, label := range r.config.EnvVarsToMetricLabels {
		if v := container.EnvMap[k]; v != "" {
			label := label
			v := v
			resourceMetricsOptions = append(resourceMetricsOptions, func(rm pmetric.ResourceMetrics) {
				rm.Resource().Attributes().UpsertString(label, v)
			})
		}
	}
	for k, label := range r.config.ContainerLabelsToMetricLabels {
		if v := container.Config.Labels[k]; v != "" {
			label := label
			v := v
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
	r.mb.RecordContainerMemoryMaxDataPoint(now, int64(memoryStats.MaxUsage))
	r.mb.RecordContainerMemoryPercentDataPoint(now, calculateMemoryPercent(memoryStats))
	r.mb.RecordContainerMemoryUsageTotalDataPoint(now, int64(totalUsage))
	r.mb.RecordContainerMemoryUsageTotalCacheDataPoint(now, int64(totalCache))
	r.mb.RecordContainerMemoryUsageLimitDataPoint(now, int64(memoryStats.Limit))

	// todo more metrics here
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
			if recorder, ok := blkioRecorder.opToRecorderMap[strings.ToLower(entry.Op)]; ok {
				recorder(now, int64(entry.Value), strconv.FormatUint(entry.Major, 10), strconv.FormatUint(entry.Minor, 10))
			}
		}
	}
}
