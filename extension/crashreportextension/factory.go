// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crashreportextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/crashreportextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension"
)

const (
	// The value of extension "type" in configuration.
	typeStr = "crashreport"
)

// NewFactory creates a factory for the crash reporter extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelDevelopment,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ExtensionSettings: config.NewExtensionSettings(component.NewID(typeStr)),
	}
}

func createExtension(_ context.Context, _ extension.CreateSettings, _ component.Config) (extension.Extension, error) {
	return &crashReportExtension{}, nil
}

type crashReportExtension struct {
}

func (c *crashReportExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *crashReportExtension) Shutdown(_ context.Context) error {
	return nil
}
