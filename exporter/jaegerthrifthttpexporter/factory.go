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

package jaegerthrifthttpexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "jaeger_thrift"
)

var once sync.Once

// NewFactory creates a factory for Jaeger Thrift over HTTP exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelDeprecated))
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: exporterhelper.NewDefaultTimeoutSettings().Timeout,
		},
	}
}

func logDeprecation(logger *zap.Logger) {
	once.Do(func() {
		logger.Warn("jaeger_thrift exporter is deprecated and will be removed in July 2023. See https://github.com/open-telemetry/opentelemetry-specification/pull/2858 for more details.")
	})
}

func createTracesExporter(
	_ context.Context,
	set exporter.CreateSettings,
	config component.Config,
) (exporter.Traces, error) {
	logDeprecation(set.Logger)
	expCfg := config.(*Config)
	return newTracesExporter(expCfg, set)
}
