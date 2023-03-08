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

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
)

const (
	typeStr = "sentry"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for Sentry exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesExporter(
	_ context.Context,
	params exporter.CreateSettings,
	config component.Config,
) (exporter.Traces, error) {
	sentryConfig, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unexpected config type: %T", config)
	}

	// Create exporter based on sentry config.
	exp, err := CreateSentryExporter(sentryConfig, params)
	return exp, err
}
