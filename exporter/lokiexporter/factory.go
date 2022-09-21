// Copyright The OpenTelemetry Authors
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

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
)

const (
	typeStr = "loki"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the legacy Loki exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultLegacyConfig,
		component.WithLogsExporter(createLogsExporter, stability),
	)
}

func createLogsExporter(ctx context.Context, set component.ExporterCreateSettings, config config.Exporter) (component.LogsExporter, error) {
	expCfg := config.(*Config)

	// this should go away once the legacy code is removed, as the config validation happens during the loading
	// of the config already, it should not be called explicitly here
	if err := expCfg.Validate(); err != nil {
		return nil, err
	}

	if expCfg.isLegacy() {
		return createLegacyLogsExporter(ctx, set, expCfg)
	}

	return createNextLogsExporter(ctx, set, expCfg)
}
