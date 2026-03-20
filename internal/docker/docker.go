// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	dtypes "github.com/docker/docker/api/types"
	ctypes "github.com/docker/docker/api/types/container"
	etypes "github.com/docker/docker/api/types/events"
	dfilters "github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const userAgent = "OpenTelemetry-Collector Docker Stats Receiver/v0.0.1"

// Container is client.ContainerInspect() response container
// stats and translated environment string map for potential labels.
type Container struct {
	*dtypes.ContainerJSON
	EnvMap map[string]string
}

// Client provides the core metric gathering functionality from the Docker Daemon.
// It retrieves container information in two forms to produce metric data: dtypes.ContainerJSON
// from client.ContainerInspect() for container information (id, name, hostname, labels, and env)
// and ctypes.StatsResponse from client.ContainerStats() for metric values.
// A persistent streaming connection is maintained per container so that the latest stats are
// always available without opening a new connection on every scrape.
type Client struct {
	client                     *docker.Client
	config                     *Config
	containers                 map[string]Container
	containersLock             sync.Mutex
	excludedImageMatcher       *stringMatcher
	logger                     *zap.Logger
	streamLatestStats          sync.Map
	streamContainerCancels     map[string]context.CancelFunc
	streamContainerCancelsLock sync.Mutex
	streamErrorGroup           *errgroup.Group
	streamClientCtx            context.Context
	streamClientCancel         context.CancelFunc
}

func NewDockerClient(config *Config, logger *zap.Logger, opts ...docker.Opt) (*Client, error) {
	clientOpts := []docker.Opt{
		docker.WithHost(config.Endpoint),
		docker.WithHTTPHeaders(map[string]string{"User-Agent": userAgent}),
	}

	if config.DockerAPIVersion != "" {
		version, err := NewAPIVersion(config.DockerAPIVersion)
		if err != nil {
			return nil, err
		}
		clientOpts = append(clientOpts, docker.WithVersion(version))
		logger.Debug("Using explicitly configured Docker API version", zap.String("version", version))
	} else {
		clientOpts = append(clientOpts, docker.WithAPIVersionNegotiation())
		logger.Debug("Docker API version not specified, using automatic version negotiation")
	}

	// Configure TLS transport when a TLS config is provided.
	if config.TLS.HasValue() && !config.TLS.Get().Insecure {
		tlsCfg, err := config.TLS.Get().LoadTLSConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("could not load docker client TLS config: %w", err)
		}
		transport := &http.Transport{TLSClientConfig: tlsCfg}
		clientOpts = append(clientOpts, docker.WithHTTPClient(&http.Client{Transport: transport}))
	} else if isTCPEndpoint(config.Endpoint) {
		// Unauthenticated TCP connections to the Docker daemon were deprecated in Docker v26.0
		// and enforcement began in v27.0. Configure the 'tls' block to secure this connection.
		// See: https://docs.docker.com/engine/deprecated/#unauthenticated-tcp-connections
		logger.Warn(
			"Unauthenticated TCP connection to Docker daemon is deprecated since Docker v26.0. "+
				"Configure the 'tls' option to use a secure connection.",
			zap.String("endpoint", config.Endpoint),
		)
	}

	// Append any additional opts passed by caller
	clientOpts = append(clientOpts, opts...)

	client, err := docker.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not create docker client: %w", err)
	}

	excludedImageMatcher, err := newStringMatcher(config.ExcludedImages)
	if err != nil {
		return nil, fmt.Errorf("could not determine docker client excluded images: %w", err)
	}

	dc := &Client{
		client:                 client,
		config:                 config,
		logger:                 logger,
		containers:             make(map[string]Container),
		containersLock:         sync.Mutex{},
		excludedImageMatcher:   excludedImageMatcher,
		streamContainerCancels: make(map[string]context.CancelFunc),
	}

	dc.streamErrorGroup = &errgroup.Group{}
	dc.streamClientCtx, dc.streamClientCancel = context.WithCancel(context.Background())

	return dc, nil
}

// Containers provides a snapshot of the currently monitored containers.
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
	options := ctypes.ListOptions{
		Filters: filters,
	}

	listCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	containerList, err := dc.client.ContainerList(listCtx, options)
	defer cancel()
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for i := range containerList {
		c := &containerList[i]
		wg.Add(1)
		go func(container *ctypes.Summary) {
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
) (*ctypes.StatsResponse, error) {
	statsCtx, cancel := context.WithTimeout(ctx, dc.config.Timeout)
	defer cancel()

	containerStats, err := dc.FetchContainerStats(statsCtx, container)
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
) (ctypes.StatsResponseReader, error) {
	dc.logger.Debug("Fetching container stats.", zap.String("id", container.ID))
	containerStats, err := dc.client.ContainerStats(ctx, container.ID, false)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
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
	containerStats ctypes.StatsResponseReader,
	container *Container,
) (*ctypes.StatsResponse, error) {
	var statsJSON ctypes.StatsResponse
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

// cachedStats pairs a stats snapshot with the time it was received.
type cachedStats struct {
	stats    *ctypes.StatsResponse
	recorded time.Time
}

// LatestContainerStats returns the most recently received stats for the given container.
// Returns false if no stats have been received yet (stream still starting up) or if the
// cached entry is older than maxAge (pass 0 to disable expiry).
// Only populated when stream_stats is enabled.
func (dc *Client) LatestContainerStats(containerID string, maxAge time.Duration) (*ctypes.StatsResponse, bool) {
	val, ok := dc.streamLatestStats.Load(containerID)
	if !ok {
		return nil, false
	}
	cs := val.(*cachedStats)
	if maxAge > 0 && time.Since(cs.recorded) > maxAge {
		return nil, false
	}
	return cs.stats, true
}

// startContainerStream opens a persistent streaming stats connection for the container
// and continuously updates the cached latest stats in the background.
// Cancels any existing stream for that container first.
func (dc *Client) startContainerStream(containerID string) {
	dc.streamContainerCancelsLock.Lock()
	if cancel, ok := dc.streamContainerCancels[containerID]; ok {
		cancel()
	}
	streamCtx, cancel := context.WithCancel(dc.streamClientCtx)
	dc.streamContainerCancels[containerID] = cancel
	dc.streamContainerCancelsLock.Unlock()

	dc.streamErrorGroup.Go(func() error {
		return dc.runStatsStream(streamCtx, containerID)
	})
}

// runStatsStream opens a Docker stats stream and keeps the latest stats updated.
// Reconnects automatically on transient errors. Stops when ctx is cancelled.
func (dc *Client) runStatsStream(ctx context.Context, containerID string) error {
	for {
		resp, err := dc.client.ContainerStats(ctx, containerID, true)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			if cerrdefs.IsNotFound(err) {
				dc.logger.Debug(
					"Daemon reported container doesn't exist. Stopping stats stream.",
					zap.String("id", containerID),
				)
				dc.RemoveContainer(containerID)
				return nil
			}
			dc.logger.Warn(
				"Could not open stats stream for container, retrying",
				zap.String("id", containerID),
				zap.Error(err),
			)
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return nil
			}
			continue
		}

		decoder := json.NewDecoder(resp.Body)
		for {
			var stats ctypes.StatsResponse
			if err := decoder.Decode(&stats); err != nil {
				resp.Body.Close()

				if !errors.Is(err, io.EOF) {
					dc.logger.Warn(
						"Error reading stats stream, reconnecting",
						zap.String("id", containerID),
						zap.Error(err),
					)
				}
				break
			}
			dc.streamLatestStats.Store(containerID, &cachedStats{stats: &stats, recorded: time.Now()})
		}
	}
}

// Events exposes the underlying Docker clients Events channel.
// Caller should close the events channel by canceling the context.
// If an error occurs, processing stops and caller must reinvoke this method.
func (dc *Client) Events(ctx context.Context, options etypes.ListOptions) (<-chan etypes.Message, <-chan error) {
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
		options := etypes.ListOptions{
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
					dc.logger.Debug("Docker container was destroyed:", zap.String("id", event.Actor.ID))
					dc.RemoveContainer(event.Actor.ID)
				default:
					dc.logger.Debug(
						"Docker container update:",
						zap.String("id", event.Actor.ID),
						zap.Any("action", event.Action),
					)

					dc.InspectAndPersistContainer(ctx, event.Actor.ID)
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
func (dc *Client) InspectAndPersistContainer(ctx context.Context, cid string) (*ctypes.InspectResponse, bool) {
	if container, ok := dc.inspectedContainerIsOfInterest(ctx, cid); ok {
		dc.persistContainer(container)
		return container, ok
	}
	return nil, false
}

// Queries inspect api and returns *ContainerJSON and true when container should be queried for stats,
// nil and false otherwise.
func (dc *Client) inspectedContainerIsOfInterest(ctx context.Context, cid string) (*ctypes.InspectResponse, bool) {
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

func (dc *Client) persistContainer(containerJSON *ctypes.InspectResponse) {
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
	dc.containers[cid] = Container{
		ContainerJSON: containerJSON,
		EnvMap:        ContainerEnvToMap(containerJSON.Config.Env),
	}
	dc.containersLock.Unlock()

	if dc.config.StreamStats {
		dc.startContainerStream(cid)
	}
}

func (dc *Client) RemoveContainer(cid string) {
	if dc.config.StreamStats {
		dc.streamContainerCancelsLock.Lock()
		if cancel, ok := dc.streamContainerCancels[cid]; ok {
			cancel()
			delete(dc.streamContainerCancels, cid)
		}
		dc.streamContainerCancelsLock.Unlock()

		dc.streamLatestStats.Delete(cid)
	}

	dc.containersLock.Lock()
	delete(dc.containers, cid)
	dc.containersLock.Unlock()

	dc.logger.Debug("Removed container from stores.", zap.String("id", cid))
}

// Close shuts down the client and waits for all background streams to finish.
func (dc *Client) Close() error {
	dc.streamClientCancel()
	return dc.streamErrorGroup.Wait()
}

func (dc *Client) shouldBeExcluded(image string) bool {
	return dc.excludedImageMatcher != nil && dc.excludedImageMatcher.matches(image)
}

// isTCPEndpoint reports whether the endpoint uses a plain TCP or HTTP scheme,
// meaning the connection is not secured by a Unix socket, named pipe, or TLS.
func isTCPEndpoint(endpoint string) bool {
	lower := strings.ToLower(endpoint)
	return strings.HasPrefix(lower, "tcp://") || strings.HasPrefix(lower, "http://")
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
