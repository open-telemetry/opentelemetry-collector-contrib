// Copyright 2022, OpenTelemetry Authors
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

package instanaexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/service/servicetest"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:        "",
			Timeout:         30 * time.Second,
			Headers:         map[string]string{},
			WriteBufferSize: 512 * 1024,
		},
	}, cfg, "failed to create default config")

	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "config.yml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	t.Run("valid config", func(t *testing.T) {
		validConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "valid")].(*Config)
		err = validConfig.Validate()

		require.NoError(t, err)
		assert.Equal(t, &Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "valid")),
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint:        "http://example.com/api/",
				Timeout:         30 * time.Second,
				Headers:         map[string]string{},
				WriteBufferSize: 512 * 1024,
			},
			Endpoint: "http://example.com/api/",
			AgentKey: "key1",
		}, validConfig)
	})

	t.Run("bad endpoint", func(t *testing.T) {
		badEndpointConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "bad_endpoint")].(*Config)
		err = badEndpointConfig.Validate()
		require.Error(t, err)
	})

	t.Run("missing agent key", func(t *testing.T) {
		missingAgentConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "missing_agent_key")].(*Config)
		err = missingAgentConfig.Validate()
		require.Error(t, err)
	})
}
