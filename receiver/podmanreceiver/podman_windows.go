// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"net/url"

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
}

func newContainerScraper(engineClient PodmanClient, logger *zap.Logger, config *Config) *ContainerScraper {
	return &ContainerScraper{}
}

func (pc *ContainerScraper) getContainers() []container {
	cs := make([]container, 0)
	return cs
}

func (pc *ContainerScraper) loadContainerList(ctx context.Context) error {
	return nil
}

func (pc *ContainerScraper) fetchContainerStats(ctx context.Context, c container) (containerStats, error) {
	return containerStats{}, nil
}

func (pc *ContainerScraper) persistContainer(c container) {
}

func (pc *ContainerScraper) containerEventLoop(ctx context.Context) {

}
