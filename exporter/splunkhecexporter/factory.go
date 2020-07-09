// Copyright 2020, OpenTelemetry Authors
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

package splunkhecexporter

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "splunk_hec"
	// default values
	defaultNumWorkers  uint = 8
	defaultMaxIdleCons      = 100
	defaultHTTPTimeout      = 10 * time.Second
)

// Factory is the factory for Splunk HEC exporter.
type Factory struct {
}

// Type gets the type of the Exporter config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CreateDefaultConfig creates the default configuration for exporter.
func (f *Factory) CreateDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		Timeout:            defaultHTTPTimeout,
		DisableCompression: false,
		MaxConnections:     defaultMaxIdleCons,
	}
}

// CreateTraceExporter creates a trace exporter based on this config.
func (f *Factory) CreateTraceExporter(
	logger *zap.Logger,
	config configmodels.Exporter,
) (component.TraceExporterOld, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	expCfg := config.(*Config)

	exp, err := createExporter(expCfg, logger)

	if err != nil {
		return nil, err
	}

	return exp, nil
}

// CreateMetricsExporter creates a metrics exporter based on this config.
func (f *Factory) CreateMetricsExporter(
	logger *zap.Logger,
	config configmodels.Exporter,
) (component.MetricsExporterOld, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}
	expCfg := config.(*Config)

	exp, err := createExporter(expCfg, logger)

	if err != nil {
		return nil, err
	}

	return exp, nil
}
