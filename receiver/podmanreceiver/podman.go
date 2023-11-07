// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

type clientFactory func(logger *zap.Logger, cfg *Config) (PodmanClient, error)

type PodmanClient interface {
	ping(context.Context) error
	stats(context.Context, url.Values) ([]containerStats, error)
	list(context.Context, url.Values) ([]container, error)
	events(context.Context, url.Values) (<-chan event, <-chan error)
}

type ContainerScraper struct {
	client         PodmanClient
	containers     map[string]container
	containersLock sync.Mutex
	logger         *zap.Logger
	config         *Config
}

func newContainerScraper(engineClient PodmanClient, logger *zap.Logger, config *Config) *ContainerScraper {
	return &ContainerScraper{
		client:     engineClient,
		containers: make(map[string]container),
		logger:     logger,
		config:     config,
	}
}

// containers provides a slice of container to use for individual fetchContainerStats calls.
func (pc *ContainerScraper) getContainers() []container {
	pc.containersLock.Lock()
	defer pc.containersLock.Unlock()
	containers := make([]container, 0, len(pc.containers))
	for _, container := range pc.containers {
		containers = append(containers, container)
	}
	return containers
}

// loadContainerList will load the initial running container maps for
// inspection and establishing which containers warrant stat gathering calls
// by the receiver.
func (pc *ContainerScraper) loadContainerList(ctx context.Context) error {
	params := url.Values{}
	runningFilter := map[string][]string{
		"status": {"running"},
	}
	jsonFilter, err := json.Marshal(runningFilter)
	if err != nil {
		return nil
	}
	params.Add("filters", string(jsonFilter))

	listCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	containerList, err := pc.client.list(listCtx, params)
	if err != nil {
		return err
	}

	for _, c := range containerList {
		pc.persistContainer(c)
	}
	return nil
}

func (pc *ContainerScraper) events(ctx context.Context, options url.Values) (<-chan event, <-chan error) {
	return pc.client.events(ctx, options)
}

func (pc *ContainerScraper) containerEventLoop(ctx context.Context) {
	filters := url.Values{}
	cidFilter := map[string][]string{
		"status": {"died", "start"},
		"type":   {"container"},
	}
	jsonFilter, err := json.Marshal(cidFilter)
	if err != nil {
		return
	}
	filters.Add("filters", string(jsonFilter))
EVENT_LOOP:
	for {
		eventCh, errCh := pc.events(ctx, filters)
		for {

			select {
			case <-ctx.Done():
				return
			case podmanEvent := <-eventCh:
				pc.logger.Info("Event received", zap.String("status", podmanEvent.Status))
				switch podmanEvent.Status {
				case "died":
					pc.logger.Debug("Podman container died:", zap.String("id", podmanEvent.ID))
					pc.removeContainer(podmanEvent.ID)
				case "start":
					pc.logger.Debug(
						"Podman container started:",
						zap.String("id", podmanEvent.ID),
						zap.String("status", podmanEvent.Status),
					)
					pc.inspectAndPersistContainer(ctx, podmanEvent.ID)
				}
			case err := <-errCh:
				// We are only interested when the context hasn't been canceled since requests made
				// with a closed context are guaranteed to fail.
				if ctx.Err() == nil {
					pc.logger.Error("Error watching podman container events", zap.Error(err))
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

// inspectAndPersistContainer queries inspect api and returns *container and true when container should be queried for stats,
// nil and false otherwise. Persists the container in the cache if container is
// running and not excluded.
func (pc *ContainerScraper) inspectAndPersistContainer(ctx context.Context, cid string) (*container, bool) {
	params := url.Values{}
	cidFilter := map[string][]string{
		"id": {cid},
	}
	jsonFilter, err := json.Marshal(cidFilter)
	if err != nil {
		return nil, false
	}
	params.Add("filters", string(jsonFilter))
	inspectCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	container, err := pc.client.list(inspectCtx, params)
	if len(container) == 1 && err == nil {
		pc.persistContainer(container[0])
		return &container[0], true
	}
	pc.logger.Error(
		"Could not inspect updated container",
		zap.String("id", cid),
		zap.Error(err),
	)
	return nil, false
}

// fetchContainerStats will query the desired container stats
func (pc *ContainerScraper) fetchContainerStats(ctx context.Context, c container) (containerStats, error) {
	params := url.Values{}
	params.Add("stream", "false")
	params.Add("containers", c.ID)

	stats, err := pc.client.stats(ctx, params)
	if err != nil || len(stats) < 1 {
		return containerStats{}, err
	}
	return stats[0], nil
}

func (pc *ContainerScraper) persistContainer(c container) {
	pc.logger.Debug("Monitoring Podman container", zap.String("id", c.ID))
	pc.containersLock.Lock()
	defer pc.containersLock.Unlock()
	pc.containers[c.ID] = c
}

func (pc *ContainerScraper) removeContainer(cid string) {
	pc.containersLock.Lock()
	defer pc.containersLock.Unlock()
	delete(pc.containers, cid)
	pc.logger.Debug("Removed container from stores.", zap.String("id", cid))
}
