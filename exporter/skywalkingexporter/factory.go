// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skywalkingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/skywalkingexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "skywalking"
)

// NewFactory creates a factory for Skywalking exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsExporter(createLogsExporter),
		component.WithMetricsExporter(createMetricsExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings:  exporterhelper.NewDefaultTimeoutSettings(),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Headers: map[string]string{},
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
		},
		NumStreams: 2,
	}
}

func createLogsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.LogsExporter, error) {
	oCfg := cfg.(*Config)
	oce := newLogsExporter(ctx, oCfg, set.TelemetrySettings)
	return exporterhelper.NewLogsExporter(
		cfg,
		set,
		oce.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(oCfg.RetrySettings),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithTimeout(oCfg.TimeoutSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	)
}

func createMetricsExporter(ctx context.Context, set component.ExporterCreateSettings, cfg config.Exporter) (component.MetricsExporter, error) {
	oCfg := cfg.(*Config)
	oce := newMetricsExporter(ctx, oCfg, set.TelemetrySettings)
	return exporterhelper.NewMetricsExporter(
		cfg,
		set,
		oce.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(oCfg.RetrySettings),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithTimeout(oCfg.TimeoutSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown))
}
