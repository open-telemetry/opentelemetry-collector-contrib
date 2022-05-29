// Copyright 2022 OpenTelemetry Authors
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

package podmanreceiver

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
	stats(context.Context, url.Values) ([]ContainerStats, error)
	list(context.Context, url.Values) ([]Container, error)
	events(context.Context, url.Values) (<-chan Event, <-chan error)
}

type ContainerScraper struct {
	client         PodmanClient
	containers     map[string]Container
	containersLock sync.Mutex
	logger         *zap.Logger
	config         *Config
}

func NewContainerScraper(engineClient PodmanClient, logger *zap.Logger, config *Config) *ContainerScraper {
	return &ContainerScraper{
		client:     engineClient,
		containers: make(map[string]Container),
		logger:     logger,
		config:     config,
	}
}

// Containers provides a slice of Container to use for individual FetchContainerStats calls.
func (pc *ContainerScraper) Containers() []Container {
	pc.containersLock.Lock()
	defer pc.containersLock.Unlock()
	containers := make([]Container, 0, len(pc.containers))
	for _, container := range pc.containers {
		containers = append(containers, container)
	}
	return containers
}

// LoadContainerList will load the initial running container maps for
// inspection and establishing which containers warrant stat gathering calls
// by the receiver.
func (pc *ContainerScraper) LoadContainerList(ctx context.Context) error {
	params := url.Values{}
	runningFilter := map[string][]string{
		"status": {"running"},
	}
	jsonFilter, err := json.Marshal(runningFilter)
	if err != nil {
		return nil
	}
	params.Add("filters", string(jsonFilter[:]))

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

func (pc *ContainerScraper) Events(ctx context.Context, options url.Values) (<-chan Event, <-chan error) {
	return pc.client.events(ctx, options)
}

func (pc *ContainerScraper) ContainerEventLoop(ctx context.Context) {
	filters := url.Values{}
	cidFilter := map[string][]string{
		"status": {"died", "start"},
		"type":   {"container"},
	}
	jsonFilter, err := json.Marshal(cidFilter)
	if err != nil {
		return
	}
	filters.Add("filters", string(jsonFilter[:]))
EVENT_LOOP:
	for {
		eventCh, errCh := pc.Events(ctx, filters)
		for {

			select {
			case <-ctx.Done():
				return
			case event := <-eventCh:
				switch event.Status {
				case "died":
					pc.logger.Debug("Podman container died:", zap.String("id", event.ID))
					pc.removeContainer(event.ID)
				case "start":
					pc.logger.Debug(
						"Podman container started:",
						zap.String("id", event.ID),
						zap.String("status", event.Status),
					)
					pc.InspectAndPersistContainer(ctx, event.ID)
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

// InspectAndPersistContainer queries inspect api and returns *ContainerStats and true when container should be queried for stats,
// nil and false otherwise. Persists the container in the cache if container is
// running and not excluded.
func (pc *ContainerScraper) InspectAndPersistContainer(ctx context.Context, cid string) (*Container, bool) {
	params := url.Values{}
	cidFilter := map[string][]string{
		"id": {cid},
	}
	jsonFilter, err := json.Marshal(cidFilter)
	if err != nil {
		return nil, false
	}
	params.Add("filters", string(jsonFilter[:]))
	listCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	cStats, err := pc.client.list(listCtx, params)
	if len(cStats) == 1 && err == nil {
		pc.persistContainer(cStats[0])
		return &cStats[0], true
	}
	pc.logger.Error(
		"Could not inspect updated container",
		zap.String("id", cid),
		zap.Error(err),
	)
	return nil, false
}

// FetchContainerStats will query the desired container stats
func (pc *ContainerScraper) FetchContainerStats(ctx context.Context, c Container) (ContainerStats, error) {
	params := url.Values{}
	params.Add("stream", "false")
	params.Add("containers", c.ID)

	statsCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	stats, err := pc.client.stats(statsCtx, params)
	if err != nil || len(stats) < 1 {
		return ContainerStats{}, err
	}
	return stats[0], nil
}

func (pc *ContainerScraper) persistContainer(c Container) {
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
