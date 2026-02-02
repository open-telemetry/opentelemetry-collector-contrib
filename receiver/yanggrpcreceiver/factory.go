// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	config := &Config{
		ServerConfig: configgrpc.NewDefaultServerConfig(),
		Security: SecurityConfig{
			RateLimiting: RateLimitingConfig{
				Enabled:           false,
				RequestsPerSecond: 100.0,
				BurstSize:         10,
				CleanupInterval:   time.Minute,
			},
			ConnectionTimeout: 30 * time.Second,
			EnableMetrics:     true,
		},
		YANG: YANGConfig{
			EnableRFCParser: true,
			CacheModules:    true,
			MaxModules:      1000,
		},
	}
	config.NetAddr.Transport = "tcp"
	config.NetAddr.Endpoint = "localhost:57500"
	config.MaxRecvMsgSizeMiB = 4
	config.MaxConcurrentStreams = 100
	config.Keepalive.GetOrInsertDefault().ServerParameters.GetOrInsertDefault().Time = 30 * time.Second
	config.Keepalive.GetOrInsertDefault().ServerParameters.GetOrInsertDefault().Timeout = 10 * time.Second

	return config
}
