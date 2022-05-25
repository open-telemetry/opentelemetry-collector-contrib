package podmanreceiver

import (
	"context"

	"go.uber.org/zap"
)

type clientFactory func(logger *zap.Logger, cfg *Config) (PodmanClient, error)

type PodmanClient interface {
	ping(context.Context) error
	stats(context.Context) ([]containerStats, error)
}
