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

package stackdriverexporter

import (
	"context"
	"sync"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr        = "stackdriver"
	defaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

var once sync.Once

// NewFactory creates a factory for the stackdriver exporter
func NewFactory() component.ExporterFactory {
	// register view for self-observability
	once.Do(func() {
		view.Register(viewPointCount)
	})

	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter),
		exporterhelper.WithMetrics(createMetricsExporter),
	)
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
		UserAgent:       "opentelemetry-collector-contrib {{version}}",
	}
}

// createTraceExporter creates a trace exporter based on this config.
func createTraceExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter) (component.TracesExporter, error) {
	eCfg := cfg.(*Config)
	return newStackdriverTraceExporter(eCfg, params)
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter) (component.MetricsExporter, error) {
	eCfg := cfg.(*Config)
	return newStackdriverMetricsExporter(eCfg, params)
}
