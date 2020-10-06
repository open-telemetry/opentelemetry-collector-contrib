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
package datadogexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},

		// These are filled when loading using the helper methods
		TagsConfig: TagsConfig{
			Hostname: "${DD_HOST}",
			Env:      "${DD_ENV}",
			Service:  "${DD_SERVICE}",
			Version:  "${DD_VERSION}",
		},

		API: APIConfig{Site: "datadoghq.com"},
		Traces: TracesConfig{
			SampleRate: 1,
		},
	}, cfg, "failed to create default config")

	assert.NoError(t, configcheck.ValidateConfig(cfg))
}
