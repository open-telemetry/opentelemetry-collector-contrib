// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver/internal/metadata"
)

// NewFactory creates a factory for the StatsD receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	grpcCfg := configgrpc.NewDefaultServerConfig()
	grpcCfg.NetAddr = confignet.NewDefaultAddrConfig()
	grpcCfg.NetAddr.Endpoint = "localhost:4320"
	grpcCfg.NetAddr.Transport = confignet.TransportTypeTCP
	grpcCfg.ReadBufferSize = 512 * 1024

	return &Config{
		ServerConfig: grpcCfg,
		AckInterval:  10 * time.Millisecond,
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	oCfg := cfg.(*Config)
	return &stefReceiver{
		cfg:                 oCfg,
		nextMetricsConsumer: nextConsumer,
		settings:            set,
	}, nil
}
