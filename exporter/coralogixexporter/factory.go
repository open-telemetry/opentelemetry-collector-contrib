// Copyright 2021, OpenTelemetry Authors
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

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// NewFactory by Coralogix
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTraceExporter),
		component.WithMetricsExporter(createMetricsExporter),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		TimeoutSettings:  exporterhelper.NewDefaultTimeoutSettings(),
		// Traces GRPC client
		GRPCClientSettings: configgrpc.GRPCClientSettings{Endpoint: "https://"},
		Metrics: configgrpc.GRPCClientSettings{
			Endpoint: "https://",
			Headers:  map[string]string{},
			// Default to gzip compression
			Compression:     configcompression.Gzip,
			WriteBufferSize: 512 * 1024,
		},
		PrivateKey: "",
		AppName:    "",
	}
}

func createTraceExporter(_ context.Context, set component.ExporterCreateSettings, config config.Exporter) (component.TracesExporter, error) {
	cfg := config.(*Config)
	exporter, err := newCoralogixExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		config,
		set,
		exporter.tracesPusher,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithStart(exporter.client.startConnection),
	)
}

func createMetricsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.MetricsExporter, error) {
	oce, err := newMetricsExporter(cfg, set)
	if err != nil {
		return nil, err
	}
	oCfg := cfg.(*Config)
	return exporterhelper.NewMetricsExporter(
		cfg,
		set,
		oce.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oCfg.TimeoutSettings),
		exporterhelper.WithRetry(oCfg.RetrySettings),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	)
}
