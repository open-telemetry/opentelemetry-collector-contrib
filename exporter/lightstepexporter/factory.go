// Copyright 2019 OpenTelemetry Authors
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
	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"go.uber.org/zap"
)

const (
	typeStr = "lightstep"
)

// Factory is the factory for the LightStep exporter.
type Factory struct {
}

// Type gets the type of the exporter configuration created by this factory.
func (f *Factory) Type() string {
	return typeStr
}

// CreateDefaultConfig creates a default configuration for this exporter.
func (f *Factory) CreateDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		AccessToken:   "",
		SatelliteHost: "https://ingest.lightstep.com",
		SatellitePort: 443,
		ServiceName:   "opentelemetry-collector",
	}
}

// CreateTraceExporter creates a LightStep trace exporter for this configuration.
func (f *Factory) CreateTraceExporter(logger *zap.Logger, cfg configmodels.Exporter) (exporter.TraceExporter, error) {
	lsConfig := cfg.(*Config)
	return newLightStepTraceExporter(lsConfig)
}

// CreateMetricsExporter returns nil.
func (f *Factory) CreateMetricsExporter(logger *zap.Logger, cfg configmodels.Exporter) (exporter.MetricsExporter, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}
