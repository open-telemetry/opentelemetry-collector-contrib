// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/metadata"
)

// NewFactory creates a factory for the Azure Functions receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	serverConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig.WriteTimeout = 0
	serverConfig.ReadHeaderTimeout = 0
	serverConfig.IdleTimeout = 0
	serverConfig.KeepAlivesEnabled = false
	serverConfig.NetAddr = confignet.AddrConfig{Transport: confignet.TransportTypeTCP}
	return &Config{
		HTTP: &serverConfig,
	}
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	next consumer.Logs,
) (receiver.Logs, error) {
	rCfg := cfg.(*Config)
	shared := receivers.GetOrAdd(cfg, func() component.Component {
		return newFunctionsReceiver(rCfg, settings)
	})
	shared.Unwrap().(*functionsReceiver).nextLogs = next
	return shared, nil
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)
	shared := receivers.GetOrAdd(cfg, func() component.Component {
		return newFunctionsReceiver(rCfg, settings)
	})
	shared.Unwrap().(*functionsReceiver).nextMetrics = next
	return shared, nil
}

// receivers stores at most one underlying receiver per config. Logs and metrics pipelines each
// get their own receiver wrapper, but they share the same receiver configuration and listen address.
// GetOrAdd reuses one *functionsReceiver for that config so Start opens the HTTP listener only once.
var receivers = sharedcomponent.NewSharedComponents()
