// Copyright 2022, OpenTelemetry Authors
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

package instanaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "instana"
	// The stability level of the exporter.
	stability = component.StabilityLevelInDevelopment
)

// NewFactory creates an Instana exporter factory
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTracesExporter, stability),
	)
}

// createDefaultConfig creates the default exporter configuration
func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:        "",
			Timeout:         30 * time.Second,
			Headers:         map[string]string{},
			WriteBufferSize: 512 * 1024,
		},
	}
}

// createTracesExporter creates a trace exporter based on this configuration
func createTracesExporter(ctx context.Context, set component.ExporterCreateSettings, config config.Exporter) (component.TracesExporter, error) {
	// TODO: Lines commented out until implementation is available
	// cfg := config.(*Config)

	ctx, cancel := context.WithCancel(ctx)

	// TODO: Lines commented out until implementation is available
	var pushConvertedTraces consumer.ConsumeTracesFunc
	/*instanaExporter, err := newInstanaExporter(cfg, set)
	if err != nil {
		cancel()
		return nil, err
	}*/

	//TODO: Lines commented out until implementation is available
	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		config,
		// instanaExporter.pushConvertedTraces,
		pushConvertedTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// exporterhelper.WithStart(instanaExporter.start),
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		exporterhelper.WithShutdown(func(context.Context) error {
			cancel()
			return nil
		}),
	)
}
