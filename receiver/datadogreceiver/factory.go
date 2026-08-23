// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/metadata"
)

// NewFactory creates a factory for DataDog receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	netAddr := confignet.NewDefaultAddrConfig()
	netAddr.Transport = confignet.TransportTypeTCP
	netAddr.Endpoint = "localhost:8126"

	serverConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig.WriteTimeout = 0
	serverConfig.ReadHeaderTimeout = 0
	serverConfig.IdleTimeout = 0
	serverConfig.KeepAlivesEnabled = false
	serverConfig.NetAddr = netAddr

	return &Config{
		ServerConfig:              serverConfig,
		ReadTimeout:               60 * time.Second,
		IdleSeriesTimeout:         0,
		IdleSeriesCleanupInterval: 5 * time.Minute,
		TraceIDCacheSize:          100,
		Logs:                      LogsConfig{DecodeJSONMessage: true},
		Intake: IntakeConfig{
			Behavior: defaultConfigIntakeBehavior,
			Proxy: ProxyConfig{
				API: datadogconfig.APIConfig{
					Site: defaultConfigIntakeProxyAPISite,
				},
			},
		},
	}
}

func createTracesReceiver(ctx context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Traces) (receiver.Traces, error) {
	var err error
	rcfg := cfg.(*Config)
	r := receivers.GetOrAdd(rcfg, func() (dd component.Component) {
		dd, err = newDataDogReceiver(ctx, rcfg, params)
		return dd
	})
	if err != nil {
		return nil, err
	}

	r.Unwrap().(*datadogReceiver).nextTracesConsumer = consumer
	return r, nil
}

func createMetricsReceiver(ctx context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	var err error
	rcfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() (dd component.Component) {
		dd, err = newDataDogReceiver(ctx, rcfg, params)
		return dd
	})
	if err != nil {
		return nil, err
	}

	r.Unwrap().(*datadogReceiver).nextMetricsConsumer = consumer
	return r, nil
}

func createLogsReceiver(ctx context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	var err error
	rcfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() (dd component.Component) {
		dd, err = newDataDogReceiver(ctx, rcfg, params)
		return dd
	})
	if err != nil {
		return nil, err
	}

	r.Unwrap().(*datadogReceiver).nextLogsConsumer = consumer
	return r, nil
}

var receivers = sharedcomponent.NewSharedComponents()
