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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr           = "azuredataexplorer"
	managedingesttype = "managed"
	queuedingesttest  = "queued"
	unknown           = "unknown"
	MetricsType       = 1
)

// Creates a factory for the ADX Exporter
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
		ClientId:       unknown,
		ClientSecret:   unknown,
		TenantId:       unknown,
		Database:       unknown,
		RawMetricTable: unknown,
		IngestionType:  queuedingesttest,
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
	// In case the ingestion type is not set , it falls back to queued ingestion.
	// This form of ingestion is always available on all clusters
	if adxCfg.IngestionType == "" {
		set.Logger.Warn("Ingestion type is not set , will be defaulted to queued ingestion")
		adxCfg.IngestionType = queuedingesttest
	}
	// call the common exporter function in baseexporter. This ensures that the client and the ingest
	// are initialized and the metrics struct are available for operations
	amp, err := newExporter(adxCfg, set.Logger, MetricsType)

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
