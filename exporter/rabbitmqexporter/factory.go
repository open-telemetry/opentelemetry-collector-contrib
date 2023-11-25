// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type rabbitmqExporterFactory struct {
}

// One connection per exporter definition since the same definition may be used for different telemetry types, resulting in different factory instances
// Channels need to be thread safe

func NewFactory() exporter.Factory {
	f := &rabbitmqExporterFactory{}
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(f.createLogsExporter, metadata.LogsStability),
	)
}

func (f *rabbitmqExporterFactory) createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	customConfig := *(cfg.(*config))
	exp, err := newLogsExporter(customConfig, set, newAmqpClient())
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exp.logsDataPusher,
		exporterhelper.WithRetry(customConfig.retrySettings))
}
