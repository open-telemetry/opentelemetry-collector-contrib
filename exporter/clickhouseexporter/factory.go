// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metadata"
)

// Deprecated: Use the `json` config option instead. This feature gate will be removed in a future version.
var featureGateJSON = featuregate.GlobalRegistry().MustRegister(
	"clickhouse.json",
	featuregate.StageDeprecated,
	featuregate.WithRegisterDescription("Deprecated: Use the `json` config option instead."),
	featuregate.WithRegisterToVersion("v0.149.0"),
)

// NewFactory creates a factory for the ClickHouse exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricExporter, metadata.MetricsStability),
	)
}

func warnIfAsyncInsertIneffective(logger *zap.Logger, c *Config) {
	if c.isNativeProtocol() && c.isAsyncInsertEnabled() {
		logger.Warn(
			"async_insert is enabled but may be ineffective with native protocol. "+
				"The clickhouse-go driver sends data using FORMAT Native, which ClickHouse processes "+
				"synchronously, bypassing async_insert buffering. This can cause excessive part creation "+
				"and merge pressure. Consider using HTTP protocol (http://host:8123) or increasing the "+
				"batch processor timeout as a workaround. "+
				"See: https://clickhouse.com/docs/optimize/asynchronous-inserts",
		)
	}
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	c := cfg.(*Config)
	c.collectorVersion = set.BuildInfo.Version
	warnIfAsyncInsertIneffective(set.Logger, c)

	var exp anyLogsExporter
	if useJSON(set.Logger, c) {
		exp = newLogsJSONExporter(set.Logger, c)
	} else {
		exp = newLogsExporter(set.Logger, c)
	}

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exp.pushLogsData,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	c := cfg.(*Config)
	c.collectorVersion = set.BuildInfo.Version
	warnIfAsyncInsertIneffective(set.Logger, c)

	var exp anyTracesExporter
	if useJSON(set.Logger, c) {
		exp = newTracesJSONExporter(set.Logger, c)
	} else {
		exp = newTracesExporter(set.Logger, c)
	}

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		exp.pushTraceData,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func useJSON(logger *zap.Logger, c *Config) bool {
	if featureGateJSON.IsEnabled() {
		logger.Warn("The clickhouse.json feature gate is deprecated. Use the `json` config option instead.")
		return true
	}
	return c.JSON
}

func createMetricExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	c := cfg.(*Config)
	c.collectorVersion = set.BuildInfo.Version
	warnIfAsyncInsertIneffective(set.Logger, c)
	exp := newMetricsExporter(set.Logger, c)

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exp.pushMetricsData,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}
