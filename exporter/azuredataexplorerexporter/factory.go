// Copyright 2020, OpenTelemetry Authors
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

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr            = "azuredataexplorer"
	defaultMaxIdleCons = 100
	defaultHTTPTimeout = 10 * time.Second
)

// NewFactory creates a factory for Splunk HEC exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTracesExporter),
		component.WithMetricsExporter(createMetricsExporter),
		component.WithLogsExporter(createLogsExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ClusterName:    "https://CLUSTER.kusto.windows.net",
		ClientId:       "unknown",
		ClientSecret:   "unknown",
		TenantId:       "unknown",
		Database:       "not-configured",
		RawMetricTable: "not-configured",
	}
}

func createMetricsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	adxCfg := config.(*Config)

	// call the common exporter function in baseexporter
	amp, err := newMetricsExporter(adxCfg, set.Logger)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		adxCfg,
		set,
		amp.metricsDataPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(amp.Close))

	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.TracesExporter, error) {
	return nil, nil
}

func createLogsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (exporter component.LogsExporter, err error) {
	return nil, nil
}
