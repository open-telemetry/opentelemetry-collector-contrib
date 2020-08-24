// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lightstepexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "lightstep"
)

// NewFactory creates a factory for LightStep exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTraceExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		AccessToken:   "",
		SatelliteHost: "ingest.lightstep.com",
		SatellitePort: 443,
		ServiceName:   "opentelemetry-collector",
		PlainText:     false,
	}
}

func createTraceExporter(
	_ context.Context,
	_ component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.TraceExporter, error) {
	lsConfig := cfg.(*Config)
	return newLightStepTraceExporter(lsConfig)
}
