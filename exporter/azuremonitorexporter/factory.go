// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"context"
	"errors"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "azuremonitor"
	// The stability level of the exporter.
	stability       = component.StabilityLevelBeta
	defaultEndpoint = "https://dc.services.visualstudio.com/v2/track"
)

var (
	errUnexpectedConfigurationType = errors.New("failed to cast configuration to Azure Monitor Config")
)

// NewFactory returns a factory for Azure Monitor exporter.
func NewFactory() exporter.Factory {
	f := &factory{}
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(f.createTracesExporter, stability),
		exporter.WithLogs(f.createLogsExporter, stability),
		exporter.WithMetrics(f.createMetricsExporter, stability))
}

// Implements the interface from go.opentelemetry.io/collector/exporter/factory.go
type factory struct {
	tChannel transportChannel
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint:          defaultEndpoint,
		MaxBatchSize:      1024,
		MaxBatchInterval:  10 * time.Second,
		SpanEventsEnabled: false,
	}
}

func (f *factory) createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	exporterConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	tc := f.getTransportChannel(exporterConfig, set.Logger)
	return newTracesExporter(exporterConfig, tc, set)
}

func (f *factory) createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	exporterConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	tc := f.getTransportChannel(exporterConfig, set.Logger)
	return newLogsExporter(exporterConfig, tc, set)
}

func (f *factory) createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	exporterConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	tc := f.getTransportChannel(exporterConfig, set.Logger)
	return newMetricsExporter(exporterConfig, tc, set)
}

// Configures the transport channel.
// This method is not thread-safe
func (f *factory) getTransportChannel(exporterConfig *Config, logger *zap.Logger) transportChannel {

	// The default transport channel uses the default send mechanism from the AppInsights telemetry client.
	// This default channel handles batching, appropriate retries, and is backed by memory.
	if f.tChannel == nil {
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

	return f.tChannel
}
