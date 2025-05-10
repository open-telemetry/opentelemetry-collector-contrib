// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package lokireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver/internal/metadata"
)

const (
	defaultGRPCPort         = 3600
	defaultHTTPPort         = 3500
	defaultGRPCBindEndpoint = "0.0.0.0:3600"
	defaultHTTPBindEndpoint = "0.0.0.0:3500"
)

// NewFactory return a new receiver.Factory for loki receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		Protocols: Protocols{
			GRPC: &configgrpc.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Endpoint:  testutil.EndpointForPort(defaultGRPCPort),
					Transport: confignet.TransportTypeTCP,
				},
			},
			HTTP: &confighttp.ServerConfig{
				Endpoint: testutil.EndpointForPort(defaultHTTPPort),
			},
		},
	}
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {

	rCfg := cfg.(*Config)
	return newLokiReceiver(rCfg, consumer, settings)
}
