// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"context"
	"errors"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter/internal/metadata"
)

var errUnexpectedConfigurationType = errors.New("failed to cast configuration to Azure Monitor Config")

// NewFactory returns a factory for Azure Monitor exporter.
func NewFactory() exporter.Factory {
	f := &factory{}
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(f.createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(f.createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(f.createMetricsExporter, metadata.MetricsStability))
}

// Implements the interface from go.opentelemetry.io/collector/exporter/factory.go
type factory struct {
	tChannel transportChannel
}

func createDefaultConfig() component.Config {
	return &Config{
		MaxBatchSize:      1024,
		MaxBatchInterval:  10 * time.Second,
		SpanEventsEnabled: false,
		QueueSettings:     exporterhelper.NewDefaultQueueConfig(),
	}
}

func (f *factory) createTracesExporter(
	_ context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	exporterConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	tc, errInstrumentationKeyOrConnectionString := f.getTransportChannel(exporterConfig, set.Logger)
	if errInstrumentationKeyOrConnectionString != nil {
		return nil, errInstrumentationKeyOrConnectionString
	}

	return newTracesExporter(exporterConfig, tc, set)
}

func (f *factory) createLogsExporter(
	_ context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	exporterConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	tc, errInstrumentationKeyOrConnectionString := f.getTransportChannel(exporterConfig, set.Logger)
	if errInstrumentationKeyOrConnectionString != nil {
		return nil, errInstrumentationKeyOrConnectionString
	}

	return newLogsExporter(exporterConfig, tc, set)
}

func (f *factory) createMetricsExporter(
	_ context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	exporterConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	tc, errInstrumentationKeyOrConnectionString := f.getTransportChannel(exporterConfig, set.Logger)
	if errInstrumentationKeyOrConnectionString != nil {
		return nil, errInstrumentationKeyOrConnectionString
	}

	return newMetricsExporter(exporterConfig, tc, set)
}

// Configures the transport channel.
// This method is not thread-safe
func (f *factory) getTransportChannel(exporterConfig *Config, logger *zap.Logger) (transportChannel, error) {
	// The default transport channel uses the default send mechanism from the AppInsights telemetry client.
	// This default channel handles batching, appropriate retries, and is backed by memory.
	if f.tChannel == nil {
		connectionVars, err := parseConnectionString(exporterConfig)
		if err != nil {
			return nil, err
		}

		exporterConfig.InstrumentationKey = configopaque.String(connectionVars.InstrumentationKey)
		exporterConfig.Endpoint = connectionVars.IngestionURL
		telemetryConfiguration := appinsights.NewTelemetryConfiguration(string(exporterConfig.InstrumentationKey))
		telemetryConfiguration.EndpointUrl = exporterConfig.Endpoint
		telemetryConfiguration.MaxBatchSize = exporterConfig.MaxBatchSize
		telemetryConfiguration.MaxBatchInterval = exporterConfig.MaxBatchInterval
		telemetryClient := appinsights.NewTelemetryClientFromConfig(telemetryConfiguration)

		f.tChannel = telemetryClient.Channel()

		// Don't even bother enabling the AppInsights diagnostics listener unless debug logging is enabled
		if checkedEntry := logger.Check(zap.DebugLevel, ""); checkedEntry != nil {
			appinsights.NewDiagnosticsMessageListener(func(msg string) error {
				logger.Debug(msg)
				return nil
			})
		}
	}

	return f.tChannel, nil
}
