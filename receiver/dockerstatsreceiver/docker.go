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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	dtypes "github.com/docker/docker/api/types"
	dfilters "github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

const (
	dockerAPIVersion = "v1.22"
	userAgent        = "OpenTelemetry-Collector Docker Stats Receiver/v0.0.1"
)

// dockerClient provides the core metric gathering functionality from the Docker Daemon.
// It retrieves container information in two forms to produce metric data: dtypes.ContainerJSON
// from client.ContainerInspect() for container information (id, name, hostname, labels, and env)
// and dtypes.StatsJSON from client.ContainerStats() for metric values.
type dockerClient struct {
	client               *docker.Client
	config               *Config
	containers           map[string]DockerContainer
	containersLock       sync.Mutex
	excludedImageMatcher *StringMatcher
	logger               *zap.Logger
}

func newDockerClient(config *Config, logger *zap.Logger) (*dockerClient, error) {
	client, err := docker.NewClientWithOpts(
		docker.WithHost(config.Endpoint),
		docker.WithVersion(dockerAPIVersion),
		docker.WithHTTPHeaders(map[string]string{"User-Agent": userAgent}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create docker client: %w", err)
	}

	excludedImageMatcher, err := NewStringMatcher(config.ExcludedImages)
	if err != nil {
		return nil, fmt.Errorf("could not determine docker client excluded images: %w", err)
	}

	dc := &dockerClient{
		client:               client,
		config:               config,
		logger:               logger,
		containers:           make(map[string]DockerContainer),
		containersLock:       sync.Mutex{},
		excludedImageMatcher: excludedImageMatcher,
	}

	return dc, nil
}

// Provides a slice of DockerContainers to use for individual FetchContainerStats calls.
func (dc *dockerClient) Containers() []DockerContainer {
	dc.containersLock.Lock()
	defer dc.containersLock.Unlock()
	containers := make([]DockerContainer, 0, len(dc.containers))
	for _, container := range dc.containers {
		containers = append(containers, container)
	}
	return containers
}

// LoadContainerList will load the initial running container maps for
// inspection and establishing which containers warrant stat gathering calls
// by the receiver.
func (dc *dockerClient) LoadContainerList(ctx context.Context) error {
	// Build initial container maps before starting loop
	filters := dfilters.NewArgs()
	filters.Add("status", "running")
	options := dtypes.ContainerListOptions{
		Filters: filters,
	}

	listCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	containerList, err := dc.client.ContainerList(listCtx, options)
	defer cancel()
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for _, c := range containerList {
		wg.Add(1)
		go func(container dtypes.Container) {
			if !dc.shouldBeExcluded(container.Image) {
				if cnt, ok := dc.inspectedContainerIsOfInterest(ctx, container.ID); ok {
					dc.persistContainer(cnt)
				}
			} else {
				dc.logger.Debug(
					"Not monitoring container per ExcludedImages",
					zap.String("image", container.Image),
					zap.String("id", container.ID),
				)
			}
			wg.Done()
		}(c)
	}
	wg.Wait()
	return nil
}

// FetchContainerStatsAndConvertToMetrics will query the desired container stats and send
// converted metrics to the results channel, since this is intended to be run in a goroutine.
func (dc *dockerClient) FetchContainerStatsAndConvertToMetrics(
	ctx context.Context,
	container DockerContainer,
) (*consumerdata.MetricsData, error) {
	dc.logger.Debug("Fetching container stats.", zap.String("id", container.ID))
	statsCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	containerStats, err := dc.client.ContainerStats(statsCtx, container.ID, false)
	defer cancel()
	if err != nil {
		if docker.IsErrNotFound(err) {
			dc.logger.Debug(
				"Daemon reported container doesn't exist. Will no longer monitor.",
				zap.String("id", container.ID),
			)
			dc.removeContainer(container.ID)
		} else {
			dc.logger.Warn(
				"Could not fetch docker containerStats for container",
				zap.String("id", container.ID),
				zap.Error(err),
			)
		}

		return nil, err
	}

	statsJSON, err := dc.toStatsJSON(containerStats, &container)
	if err != nil { // results have been sent in converter
		return nil, err
	}

	md, err := ContainerStatsToMetrics(statsJSON, &container, dc.config)
	if err != nil {
		dc.logger.Error(
			"Could not convert docker containerStats for container id",
			zap.String("id", container.ID),
			zap.Error(err),
		)
		return nil, err
	}

	return md, nil
}

func (dc *dockerClient) toStatsJSON(
	containerStats dtypes.ContainerStats,
	container *DockerContainer,
) (*dtypes.StatsJSON, error) {
	var statsJSON dtypes.StatsJSON
	err := json.NewDecoder(containerStats.Body).Decode(&statsJSON)
	containerStats.Body.Close()
	if err != nil {
		// EOF means there aren't any containerStats, perhaps because the container has been removed.
		if err == io.EOF {
			// It isn't indicative of actual error.
			return nil, err
		}
		dc.logger.Error(
			"Could not parse docker containerStats for container id",
			zap.String("id", container.ID),
			zap.Error(err),
		)
		return nil, err
	}
	return &statsJSON, nil
}

// Queries inspect api and returns *ContainerJSON and true when container should be queried for stats,
// nil and false otherwise.
func (dc *dockerClient) inspectedContainerIsOfInterest(ctx context.Context, cid string) (*dtypes.ContainerJSON, bool) {
	inspectCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	container, err := dc.client.ContainerInspect(inspectCtx, cid)
	defer cancel()
	if err != nil {
		dc.logger.Error(
			"Could not inspect updated container",
			zap.String("id", cid),
			zap.Error(err),
		)
	} else if !dc.shouldBeExcluded(container.Config.Image) {
		return &container, true
	}
	return nil, false
}

func (dc *dockerClient) persistContainer(containerJSON *dtypes.ContainerJSON) {
	if containerJSON == nil {
		return
	}
	cid := containerJSON.ID
	dc.logger.Debug("Monitoring Docker container", zap.String("id", cid))
	dc.containers[cid] = DockerContainer{
		ContainerJSON: containerJSON,
		EnvMap:        containerEnvToMap(containerJSON.Config.Env),
	}
}

func (dc *dockerClient) removeContainer(cid string) {
	dc.containersLock.Lock()
	defer dc.containersLock.Unlock()
	delete(dc.containers, cid)
	dc.logger.Debug("Removed container from stores.", zap.String("id", cid))
}

func (dc *dockerClient) shouldBeExcluded(image string) bool {
	return dc.excludedImageMatcher != nil && dc.excludedImageMatcher.Matches(image)
}

func containerEnvToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, v := range env {
		parts := strings.Split(v, "=")
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}
