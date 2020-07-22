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

package awsemfexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
)

const (
	// The value of "type" key in configuration.
	typeStr = "awsemf"
)

// Factory is the factory for AWS EMF exporter.
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
		LogGroupName:          "",
		LogStreamName:         "",
		Endpoint:              "",
		RequestTimeoutSeconds: 30,
		MaxRetries:            2,
		NoVerifySSL:           false,
		ProxyAddress:          "",
		Region:                "",
		LocalMode:             false,
		ResourceARN:           "",
		RoleARN:               "",
		ForceFlushInterval:    defaultForceFlushIntervalInSeconds,
	}
}

// CreateTraceExporter creates a trace exporter based on this config.
func (f *Factory) CreateTraceExporter(_ context.Context,
	_ component.ExporterCreateParams,
	config configmodels.Exporter) (component.TraceExporter, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsExporter creates a metrics exporter based on this config.
func (f *Factory) CreateMetricsExporter(_ context.Context,
	params component.ExporterCreateParams,
	config configmodels.Exporter) (component.MetricsExporter, error) {

	expCfg := config.(*Config)

	exp, err := New(expCfg, params)

	if err != nil {
		return nil, err
	}

	return exp, nil
}