// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

// NewFactory creates a factory for Sentry exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 30 * time.Second

	return &Config{
		ClientConfig:  httpClientConfig,
		TimeoutConfig: exporterhelper.NewDefaultTimeoutConfig(),
		QueueConfig:   configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Traces, error) {
	sc, state, err := getOrCreateEndpointState(config, set)
	if err != nil {
		return nil, err
	}
	cfg := config.(*Config)
	se := newSentryExporter(state, set.Logger)
	return exporterhelper.NewTraces(
		ctx,
		set,
		config,
		se.pushTraceData,
		exporterhelper.WithStart(sc.Start),
		exporterhelper.WithShutdown(sc.Shutdown),
		exporterhelper.WithTimeout(cfg.TimeoutConfig),
		exporterhelper.WithQueue(cfg.QueueConfig),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Logs, error) {
	sc, state, err := getOrCreateEndpointState(config, set)
	if err != nil {
		return nil, err
	}
	cfg := config.(*Config)
	se := newSentryExporter(state, set.Logger)
	return exporterhelper.NewLogs(
		ctx,
		set,
		config,
		se.pushLogData,
		exporterhelper.WithStart(sc.Start),
		exporterhelper.WithShutdown(sc.Shutdown),
		exporterhelper.WithTimeout(cfg.TimeoutConfig),
		exporterhelper.WithQueue(cfg.QueueConfig),
	)
}

// getOrCreateEndpointState creates an endpointState and caches it for a particular configuration.
func getOrCreateEndpointState(cfg component.Config, set exporter.Settings) (*sharedcomponent.SharedComponent, *endpointState, error) {
	sc := states.GetOrAdd(cfg, func() component.Component {
		sentryConfig := cfg.(*Config)
		se, err := newEndpointState(sentryConfig, set)
		if err != nil {
			return nil
		}
		return se
	})

	unwrapped := sc.Unwrap()
	if unwrapped == nil {
		return nil, nil, errors.New("failed to create sentry exporter")
	}
	return sc, unwrapped.(*endpointState), nil
}

var states = sharedcomponent.NewSharedComponents()
