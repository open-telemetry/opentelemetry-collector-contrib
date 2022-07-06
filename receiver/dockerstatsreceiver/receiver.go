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
	for res := range results {
		if res.err != nil {
			// Don't know the number of failed stats, but one container fetch is a partial error.
			errs = multierr.Append(errs, scrapererror.NewPartialScrapeError(res.err, 0))
			continue
		}
		//res.md.ResourceMetrics().CopyTo(md.ResourceMetrics())

		r.recordContainerStats(now, res.stats, res.container)
	}

	return r.mb.Emit(), errs
}

func (r *receiver) recordContainerStats(now pcommon.Timestamp, containerStats *dtypes.StatsJSON, container *docker.Container) {

	// Record memory metrics
	memoryStats := &containerStats.MemoryStats
	totalCache := memoryStats.Stats["total_cache"]
	totalUsage := memoryStats.Usage - totalCache
	r.mb.RecordMemoryMaxDataPoint(now, int64(memoryStats.MaxUsage))
	r.mb.RecordMemoryPercentDataPoint(now, calculateMemoryPercent(memoryStats))
	r.mb.RecordMemoryUsageTotalDataPoint(now, int64(totalUsage))
	r.mb.RecordMemoryUsageTotalCacheDataPoint(now, int64(totalCache))
	r.mb.RecordMemoryUsageLimitDataPoint(now, int64(memoryStats.Limit))

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

	r.mb.Emit(resourceMetricsOptions...)
}
