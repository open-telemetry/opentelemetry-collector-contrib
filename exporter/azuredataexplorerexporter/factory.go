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

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr            = "azuredataexplorer"
	managedIngestType  = "managed"
	queuedIngestTest   = "queued"
	otelDb             = "oteldb"
	defaultMetricTable = "OTELMetrics"
	defaultLogTable    = "OTELLogs"
	defaultTraceTable  = "OTELTraces"
	metricsType        = 1
	logsType           = 2
	tracesType         = 3
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// Creates a factory for the ADX Exporter
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithMetrics(createMetricsExporter, stability),
		exporter.WithLogs(createLogsExporter, stability),
	)
}

// Create default configurations
func createDefaultConfig() component.Config {
	return &Config{
		Database:      otelDb,
		MetricTable:   defaultMetricTable,
		LogTable:      defaultLogTable,
		TraceTable:    defaultTraceTable,
		IngestionType: queuedIngestTest,
	}
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	config component.Config,
) (exporter.Metrics, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	adxCfg := config.(*Config)
	setDefaultIngestionType(adxCfg, set.Logger)
	version := set.BuildInfo.Version
	// call the common exporter function in baseexporter. This ensures that the client and the ingest
	// are initialized and the metrics struct are available for operations
	adp, err := newExporter(adxCfg, set.Logger, metricsType, version)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		adxCfg,
		adp.metricsDataPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(adp.Close))

	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	config component.Config,
) (exporter.Traces, error) {
	adxCfg := config.(*Config)
	setDefaultIngestionType(adxCfg, set.Logger)
	version := set.BuildInfo.Version
	// call the common exporter function in baseexporter. This ensures that the client and the ingest
	// are initialized and the metrics struct are available for operations
	adp, err := newExporter(adxCfg, set.Logger, tracesType, version)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewTracesExporter(
		ctx,
		set,
		adxCfg,
		adp.tracesDataPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(adp.Close))

	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	config component.Config,
) (exp exporter.Logs, err error) {
	adxCfg := config.(*Config)
	setDefaultIngestionType(adxCfg, set.Logger)
	version := set.BuildInfo.Version
	// call the common exporter function in baseexporter. This ensures that the client and the ingest
	// are initialized and the metrics struct are available for operations
	adp, err := newExporter(adxCfg, set.Logger, logsType, version)

	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewLogsExporter(
		ctx,
		set,
		adxCfg,
		adp.logsDataPusher,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithShutdown(adp.Close))

	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func setDefaultIngestionType(config *Config, logger *zap.Logger) {
	// If ingestion type is not set , it falls back to queued ingestion.
	// This form of ingestion is always available on all clusters
	if config.IngestionType == "" {
		logger.Warn("Ingestion type is not set , will be defaulted to queued ingestion")
		config.IngestionType = queuedIngestTest
	}
}
