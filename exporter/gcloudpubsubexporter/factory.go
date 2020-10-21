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

package gcloudpubsubexporter

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr        = "gcloudpubsub"
	defaultTimeout = 12 * time.Second
)

func NewFactory() component.ExporterFactory {
	return &PubsubExporterFactory{
		exporters: make(map[string]*pubsubExporter),
	}
}

type PubsubExporterFactory struct {
	exporters map[string]*pubsubExporter
}

func (factory *PubsubExporterFactory) ensureReceiver(params component.ExporterCreateParams, config configmodels.Receiver) *pubsubExporter {
	receiver := factory.exporters[config.Name()]
	if receiver != nil {
		return receiver
	}
	rconfig := config.(*Config)
	receiver = &pubsubExporter{
		instanceName: config.Name(),
		logger:       params.Logger,
		userAgent:    strings.ReplaceAll(rconfig.UserAgent, "{{version}}", params.ApplicationStartInfo.Version),
		config:       rconfig,
	}
	factory.exporters[config.Name()] = receiver
	return receiver
}

func (factory *PubsubExporterFactory) Type() configmodels.Type {
	return typeStr
}

// createDefaultConfig creates the default configuration for exporter.
func (factory *PubsubExporterFactory) CreateDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		UserAgent:       "opentelemetry-collector-contrib {{version}}",
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
	}
}

func (factory *PubsubExporterFactory) CreateTracesExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter) (component.TracesExporter, error) {

	exporter := factory.ensureReceiver(params, cfg)
	exporter.tracesTopicName = cfg.(*Config).TracesTopic
	return exporter, nil
}

func (factory *PubsubExporterFactory) CreateMetricsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter) (component.MetricsExporter, error) {

	exporter := factory.ensureReceiver(params, cfg)
	exporter.metricsTopicName = cfg.(*Config).MetricsTopic
	return exporter, nil
}

func (factory *PubsubExporterFactory) CreateLogsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter) (component.LogsExporter, error) {
	exporter := factory.ensureReceiver(params, cfg)
	exporter.logsTopicName = cfg.(*Config).LogsTopic
	return exporter, nil
}
