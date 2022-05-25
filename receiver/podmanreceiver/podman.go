package podmanreceiver

import (
	"context"
	"net/url"

	"go.uber.org/zap"
)

type clientFactory func(logger *zap.Logger, cfg *Config) (PodmanClient, error)

type PodmanClient interface {
	ping(context.Context) error
	stats(context.Context, url.Values) ([]containerStats, error)
	list(context.Context, url.Values) ([]Container, error)
	events(context.Context, url.Values) (<-chan Event, <-chan error)
}

type ContainerScraper struct {
	client PodmanClient

	logger *zap.Logger
	config *Config
}

func NewContainerScraper(engineClient PodmanClient, logger *zap.Logger, config *Config) *ContainerScraper {
	return &ContainerScraper{
		client: engineClient,
		logger: logger,
		config: config,
	}
}

// FetchContainerStats will query all desired container stats
func (pc *ContainerScraper) FetchContainerStats(ctx context.Context) ([]containerStats, error) {
	params := url.Values{}
	params.Add("stream", "false")

	statsCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	return pc.client.stats(statsCtx, params)
}
