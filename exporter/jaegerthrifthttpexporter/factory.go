// Copyright 2019, OpenTelemetry Authors
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

package jaegerthrifthttpexporter

import (
	"context"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "jaeger_thrift"
)

// NewFactory creates a factory for Jaeger Thrift over HTTP exporter.
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
		Timeout: defaultHTTPTimeout,
	}
}

func createTraceExporter(
	_ context.Context,
	_ component.ExporterCreateParams,
	config configmodels.Exporter,
) (component.TracesExporter, error) {

	expCfg := config.(*Config)
	_, err := url.ParseRequestURI(expCfg.URL)
	if err != nil {
		// TODO: Improve error message, see #215
		err = fmt.Errorf(
			"%q config requires a valid \"url\": %v",
			expCfg.Name(),
			err)
		return nil, err
	}

	if expCfg.Timeout <= 0 {
		err := fmt.Errorf(
			"%q config requires a positive value for \"timeout\"",
			expCfg.Name())
		return nil, err
	}

	return newTraceExporter(config, component.ExporterCreateParams{Logger: zap.NewNop()}, expCfg.URL, expCfg.Headers, expCfg.Timeout)
}
