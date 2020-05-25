// Copyright 2019, OpenTelemetry Authors
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

package azuremonitorexporter

import (
	"errors"
	"time"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr         = "azuremonitor"
	defaultEndpoint = "https://dc.services.visualstudio.com/v2/track"
)

var (
	errUnexpectedConfigurationType = errors.New("failed to cast configuration to Azure Monitor Config")
)

// Factory for Azure Monitor exporter.
// Implements the interface from go.opentelemetry.io/collector/exporter/factory.go
type Factory struct {
	TransportChannel transportChannel
}

// Type gets the type of the Exporter config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CreateDefaultConfig creates the default configuration for exporter.
func (f *Factory) CreateDefaultConfig() configmodels.Exporter {

	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		Endpoint:         defaultEndpoint,
		MaxBatchSize:     1024,
		MaxBatchInterval: 10 * time.Second,
	}
}

// CreateTraceExporter creates a trace exporter based on this config.
func (f *Factory) CreateTraceExporter(logger *zap.Logger, config configmodels.Exporter) (component.TraceExporterOld, error) {
	exporterConfig, ok := config.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	tc := f.getTransportChannel(exporterConfig, logger)
	return newTraceExporter(exporterConfig, tc, logger)
}

// CreateMetricsExporter creates a metrics exporter based on this config.
func (f *Factory) CreateMetricsExporter(
	logger *zap.Logger,
	cfg configmodels.Exporter,
) (component.MetricsExporterOld, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

// Configures the transport channel.
// This method is not thread-safe
func (f *Factory) getTransportChannel(exporterConfig *Config, logger *zap.Logger) transportChannel {

	// The default transport channel uses the default send mechanism from the AppInsights telemetry client.
	// This default channel handles batching, appropriate retries, and is backed by memory.
	if f.TransportChannel == nil {
		telemetryConfiguration := appinsights.NewTelemetryConfiguration(exporterConfig.InstrumentationKey)
		telemetryConfiguration.EndpointUrl = exporterConfig.Endpoint
		telemetryConfiguration.MaxBatchSize = exporterConfig.MaxBatchSize
		telemetryConfiguration.MaxBatchInterval = exporterConfig.MaxBatchInterval
		telemetryClient := appinsights.NewTelemetryClientFromConfig(telemetryConfiguration)

		f.TransportChannel = telemetryClient.Channel()

		// Don't even bother enabling the AppInsights diagnostics listener unless debug logging is enabled
		if checkedEntry := logger.Check(zap.DebugLevel, ""); checkedEntry != nil {
			appinsights.NewDiagnosticsMessageListener(func(msg string) error {
				logger.Debug(msg)
				return nil
			})
		}
	}

	return f.TransportChannel
}
