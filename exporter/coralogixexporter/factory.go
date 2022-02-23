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
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// NewFactory by Coralogix
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typestr,
		createDefaultConfig,
		component.WithTracesExporter(createTraceExporter),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings:   config.NewExporterSettings(config.NewComponentID(typestr)),
		QueueSettings:      exporterhelper.NewDefaultQueueSettings(),
		RetrySettings:      exporterhelper.NewDefaultRetrySettings(),
		GRPCClientSettings: configgrpc.GRPCClientSettings{Endpoint: "https://"},
		PrivateKey:         "",
		AppName:            "",
		SubSystem:          "",
	}
}

func createTraceExporter(_ context.Context, set component.ExporterCreateSettings, config config.Exporter) (component.TracesExporter, error) {
	cfg := config.(*Config)
	exporter := newCoralogixExporter(cfg, set)

	return exporterhelper.NewTracesExporter(
		config,
		set,
		exporter.tracesPusher,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(exporter.client.startConnection),
	)
}
