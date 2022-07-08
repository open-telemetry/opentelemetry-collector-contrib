// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"context"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "googlemanagedprometheus"
	// The stability level of the exporter.
	stability      = component.StabilityLevelAlpha
	defaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

// NewFactory creates a factory for the googlemanagedprometheus exporter
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsExporterAndStabilityLevel(createMetricsExporter, stability),
	)
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	ctx context.Context,
	params component.ExporterCreateSettings,
	cfg config.Exporter) (component.MetricsExporter, error) {
	eCfg := cfg.(*Config)
	mExp, err := collector.NewGoogleCloudMetricsExporter(ctx, eCfg.GMPConfig.toCollectorConfig(), params.TelemetrySettings.Logger, params.BuildInfo.Version, eCfg.Timeout)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		cfg,
		params,
		mExp.PushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithRetry(eCfg.RetrySettings))
}
