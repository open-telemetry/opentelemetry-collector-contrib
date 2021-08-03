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

package dynatraceexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	dtconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
)

const (
	// typeStr is the type of the exporter
	typeStr = "dynatrace"
)

// NewFactory creates a Dynatrace exporter factory
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithMetrics(createMetricsExporter),
	)
}

// createDefaultConfig creates the default exporter configuration
func createDefaultConfig() config.Exporter {
	return &dtconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),
		ResourceToTelemetrySettings: exporterhelper.ResourceToTelemetrySettings{
			Enabled: false,
		},

		APIToken:           "",
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ""},

		Tags:              []string{},
		DefaultDimensions: make(map[string]string),
	}
}

// createMetricsExporter creates a metrics exporter based on this
func createMetricsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	c config.Exporter,
) (component.MetricsExporter, error) {

	cfg := c.(*dtconfig.Config)

	if err := cfg.ValidateAndConfigureHTTPClientSettings(); err != nil {
		return nil, err
	}

	exp := newMetricsExporter(set, cfg)

	return exporterhelper.NewMetricsExporter(
		cfg,
		set,
		exp.PushMetricsData,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithResourceToTelemetryConversion(cfg.ResourceToTelemetrySettings),
	)
}
