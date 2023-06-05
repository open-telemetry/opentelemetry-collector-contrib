// Copyright The OpenTelemetry Authors
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

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	dtypes "github.com/docker/docker/api/types"
	devents "github.com/docker/docker/api/types/events"
	dfilters "github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"go.uber.org/zap"
)

const (
	minimalRequiredDockerAPIVersion = 1.22
	userAgent                       = "OpenTelemetry-Collector Docker Stats Receiver/v0.0.1"
)

// Container is client.ContainerInspect() response container
// stats and translated environment string map for potential labels.
type Container struct {
	*dtypes.ContainerJSON
	EnvMap map[string]string
}

// Client provides the core metric gathering functionality from the Docker Daemon.
// It retrieves container information in two forms to produce metric data: dtypes.ContainerJSON
// from client.ContainerInspect() for container information (id, name, hostname, labels, and env)
// and dtypes.StatsJSON from client.ContainerStats() for metric values.
type Client struct {
	client               *docker.Client
	config               *Config
	containers           map[string]Container
	containersLock       sync.Mutex
	excludedImageMatcher *stringMatcher
	logger               *zap.Logger
}

func NewDockerClient(config *Config, logger *zap.Logger, opts ...docker.Opt) (*Client, error) {
	client, err := docker.NewClientWithOpts(
		append([]docker.Opt{
			docker.WithHost(config.Endpoint),
			docker.WithVersion(fmt.Sprintf("v%v", config.DockerAPIVersion)),
			docker.WithHTTPHeaders(map[string]string{"User-Agent": userAgent}),
		}, opts...)...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create docker client: %w", err)
	}

	excludedImageMatcher, err := newStringMatcher(config.ExcludedImages)
	if err != nil {
		return nil, fmt.Errorf("could not determine docker client excluded images: %w", err)
	}

	dc := &Client{
		client:               client,
		config:               config,
		logger:               logger,
		containers:           make(map[string]Container),
		containersLock:       sync.Mutex{},
		excludedImageMatcher: excludedImageMatcher,
	}

	return dc, nil
}

// Containers provides a slice of Container to use for individual FetchContainerStats calls.
func (dc *Client) Containers() []Container {
	dc.containersLock.Lock()
	defer dc.containersLock.Unlock()
	containers := make([]Container, 0, len(dc.containers))
	for _, container := range dc.containers {
		containers = append(containers, container)
	}
	return containers
}

// LoadContainerList will load the initial running container maps for
// inspection and establishing which containers warrant stat gathering calls
// by the receiver.
func (dc *Client) LoadContainerList(ctx context.Context) error {
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
				dc.InspectAndPersistContainer(ctx, container.ID)
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

// FetchContainerStatsAsJSON will query the desired container stats
// and return them as StatsJSON
func (dc *Client) FetchContainerStatsAsJSON(
	ctx context.Context,
	container Container,
) (*dtypes.StatsJSON, error) {
	containerStats, err := dc.FetchContainerStats(ctx, container)
	if err != nil {
		return nil, err
	}

	statsJSON, err := dc.toStatsJSON(containerStats, &container)
	if err != nil {
		return nil, err
	}

	return statsJSON, nil
}

// FetchContainerStats will query the desired container stats
// and return them as ContainerStats
func (dc *Client) FetchContainerStats(
	ctx context.Context,
	container Container,
) (dtypes.ContainerStats, error) {
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
			dc.RemoveContainer(container.ID)
		} else {
			dc.logger.Warn(
				"Could not fetch docker containerStats for container",
				zap.String("id", container.ID),
				zap.Error(err),
			)
		}
	}

	return containerStats, err
}

func (dc *Client) toStatsJSON(
	containerStats dtypes.ContainerStats,
	container *Container,
) (*dtypes.StatsJSON, error) {
	var statsJSON dtypes.StatsJSON
	err := json.NewDecoder(containerStats.Body).Decode(&statsJSON)
	containerStats.Body.Close()
	if err != nil {
		// EOF means there aren't any containerStats, perhaps because the container has been removed.
		if errors.Is(err, io.EOF) {
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

// Events exposes the underlying Docker clients Events channel.
// Caller should close the events channel by canceling the context.
// If an error occurs, processing stops and caller must reinvoke this method.
func (dc *Client) Events(ctx context.Context, options dtypes.EventsOptions) (<-chan devents.Message, <-chan error) {
	return dc.client.Events(ctx, options)
}

func (dc *Client) ContainerEventLoop(ctx context.Context) {
	filters := dfilters.NewArgs([]dfilters.KeyValuePair{
		{Key: "type", Value: "container"},
		{Key: "event", Value: "destroy"},
		{Key: "event", Value: "die"},
		{Key: "event", Value: "pause"},
		{Key: "event", Value: "rename"},
		{Key: "event", Value: "stop"},
		{Key: "event", Value: "start"},
		{Key: "event", Value: "unpause"},
		{Key: "event", Value: "update"},
	}...)
	lastTime := time.Now()

EVENT_LOOP:
	for {
		options := dtypes.EventsOptions{
			Filters: filters,
			Since:   lastTime.Format(time.RFC3339Nano),
		}
		eventCh, errCh := dc.Events(ctx, options)

		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventCh:
				switch event.Action {
				case "destroy":
					dc.logger.Debug("Docker container was destroyed:", zap.String("id", event.ID))
					dc.RemoveContainer(event.ID)
				default:
					dc.logger.Debug(
						"Docker container update:",
						zap.String("id", event.ID),
						zap.String("action", event.Action),
					)

					dc.InspectAndPersistContainer(ctx, event.ID)
				}

				if event.TimeNano > lastTime.UnixNano() {
					lastTime = time.Unix(0, event.TimeNano)
				}

			case err := <-errCh:
				// We are only interested when the context hasn't been canceled since requests made
				// with a closed context are guaranteed to fail.
				if ctx.Err() == nil {
					dc.logger.Error("Error watching docker container events", zap.Error(err))
					// Either decoding or connection error has occurred, so we should resume the event loop after
					// waiting a moment.  In cases of extended daemon unavailability this will retry until
					// collector teardown or background context is closed.
					select {
					case <-time.After(3 * time.Second):
						continue EVENT_LOOP
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

// InspectAndPersistContainer queries inspect api and returns *ContainerJSON and true when container should be queried for stats,
// nil and false otherwise. Persists the container in the cache if container is
// running and not excluded.
func (dc *Client) InspectAndPersistContainer(ctx context.Context, cid string) (*dtypes.ContainerJSON, bool) {
	if container, ok := dc.inspectedContainerIsOfInterest(ctx, cid); ok {
		dc.persistContainer(container)
		return container, ok
	}
	return nil, false
}

// Queries inspect api and returns *ContainerJSON and true when container should be queried for stats,
// nil and false otherwise.
func (dc *Client) inspectedContainerIsOfInterest(ctx context.Context, cid string) (*dtypes.ContainerJSON, bool) {
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

func (dc *Client) persistContainer(containerJSON *dtypes.ContainerJSON) {
	if containerJSON == nil {
		return
	}

	cid := containerJSON.ID
	if !containerJSON.State.Running || containerJSON.State.Paused {
		dc.logger.Debug("Docker container not running.  Will not persist.", zap.String("id", cid))
		dc.RemoveContainer(cid)
		return
	}

	dc.logger.Debug("Monitoring Docker container", zap.String("id", cid))
	dc.containersLock.Lock()
	defer dc.containersLock.Unlock()
	dc.containers[cid] = Container{
		ContainerJSON: containerJSON,
		EnvMap:        ContainerEnvToMap(containerJSON.Config.Env),
	}
}

func (dc *Client) RemoveContainer(cid string) {
	dc.containersLock.Lock()
	defer dc.containersLock.Unlock()
	delete(dc.containers, cid)
	dc.logger.Debug("Removed container from stores.", zap.String("id", cid))
}

func (dc *Client) shouldBeExcluded(image string) bool {
	return dc.excludedImageMatcher != nil && dc.excludedImageMatcher.matches(image)
}

func ContainerEnvToMap(env []string) map[string]string {
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
