// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"context"
	"errors"
	"sync"
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
	f := &factory{
		mu:            sync.RWMutex{},
		hasInitLogger: false,
		tChannels:     make(map[component.ID]transportChannel),
	}
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(f.createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(f.createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(f.createMetricsExporter, metadata.MetricsStability))
}

// Implements the interface from go.opentelemetry.io/collector/exporter/factory.go
type factory struct {
	mu            sync.RWMutex
	hasInitLogger bool
	tChannels     map[component.ID]transportChannel
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

	f.initLogger(set.Logger)
	tc, errInstrumentationKeyOrConnectionString := f.getTransportChannel(set.ID, exporterConfig)
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

	f.initLogger(set.Logger)
	tc, errInstrumentationKeyOrConnectionString := f.getTransportChannel(set.ID, exporterConfig)
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

	f.initLogger(set.Logger)
	tc, errInstrumentationKeyOrConnectionString := f.getTransportChannel(set.ID, exporterConfig)
	if errInstrumentationKeyOrConnectionString != nil {
		return nil, errInstrumentationKeyOrConnectionString
	}

	return newMetricsExporter(exporterConfig, tc, set)
}

func (f *factory) initLogger(logger *zap.Logger) {
	if f.hasInitLogger {
		return
	}
	if checkedEntry := logger.Check(zap.DebugLevel, ""); checkedEntry != nil {
		appinsights.NewDiagnosticsMessageListener(func(msg string) error {
			logger.Debug(msg)
			return nil
		})
	}
	f.hasInitLogger = true
}

// Configures the transport channel.
// This method is not thread-safe
func (f *factory) getTransportChannel(id component.ID, exporterConfig *Config) (transportChannel, error) {
	f.mu.RLock()
	if channel, exists := f.tChannels[id]; exists {
		f.mu.RUnlock()
		return channel, nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check locking to prevent redundant channel creation
	if channel, exists := f.tChannels[id]; exists {
		return channel, nil
	}

	connectionVars, err := parseConnectionString(exporterConfig)
	if err != nil {
		return nil, err
	}

	exporterConfig.InstrumentationKey = configopaque.String(connectionVars.InstrumentationKey)
	exporterConfig.Endpoint = connectionVars.IngestionURL
	telemetryConfiguration := appinsights.NewTelemetryConfiguration(connectionVars.InstrumentationKey)
	telemetryConfiguration.EndpointUrl = connectionVars.IngestionURL
	telemetryConfiguration.MaxBatchSize = exporterConfig.MaxBatchSize
	telemetryConfiguration.MaxBatchInterval = exporterConfig.MaxBatchInterval

	telemetryClient := appinsights.NewTelemetryClientFromConfig(telemetryConfiguration)
	tChannel := telemetryClient.Channel()

	if f.tChannels == nil {
		f.tChannels = make(map[component.ID]transportChannel)
	}
	f.tChannels[id] = tChannel

	return tChannel, nil
}
