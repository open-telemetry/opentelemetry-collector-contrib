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
	"go.opentelemetry.io/collector/exporter"
)

const (
	typeStr = "loki"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the legacy Loki exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultLegacyConfig,
		exporter.WithLogs(createLogsExporter, stability),
	)
}

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Logs, error) {
	expCfg := config.(*Config)

	if expCfg.isLegacy() {
		return createLegacyLogsExporter(ctx, set, expCfg)
	}

	return createNextLogsExporter(ctx, set, expCfg)
}
