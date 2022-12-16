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

package humioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/humioexporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The key used to refer to this exporter
	typeStr = "humio"
	// The stability level of the exporter.
	stability = component.StabilityLevelDeprecated
)

// NewFactory creates an exporter factory for Humio
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
	)
}

// Provides a struct with default values for all relevant configuration settings
func createDefaultConfig() component.Config {
	return &Config{

		// Default settings inherited from exporter helper
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),

		HTTPClientSettings: confighttp.HTTPClientSettings{
			Headers: map[string]configopaque.String{},
		},

		// Settings specific to the Humio exporter
		DisableCompression: false,
		Tag:                TagNone,
		Traces: TracesConfig{
			UnixTimestamps: false,
		},
	}
}

// Creates a new trace exporter for Humio
func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	config component.Config,
) (exporter.Traces, error) {
	if config == nil {
		return nil, errors.New("missing config")
	}
	cfg := config.(*Config)

	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	// We only require the trace ingest token when the trace exporter is enabled
	if cfg.Traces.IngestToken == "" {
		return nil, errors.New("an ingest token for traces is required when enabling the Humio trace exporter")
	}

	exporter := newTracesExporter(cfg, set.TelemetrySettings)

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
	)
}
