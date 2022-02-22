// Copyright  The OpenTelemetry Authors
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

package ecstaskobserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Extensions), 3)

	dflt := cfg.Extensions[config.NewComponentID(typeStr)]
	assert.Equal(t, dflt, factory.CreateDefaultConfig())

	withEndpoint := cfg.Extensions[config.NewComponentIDWithName(typeStr, "with-endpoint")].(*Config)
	assert.Equal(t, &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "with-endpoint")),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://a.valid.url:1234/path",
		},
		PortLabels:      []string{"ECS_TASK_OBSERVER_PORT"},
		RefreshInterval: 100 * time.Second,
	}, withEndpoint)

	withPortLabels := cfg.Extensions[config.NewComponentIDWithName(typeStr, "with-port-labels")].(*Config)
	assert.Equal(t, &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "with-port-labels")),
		PortLabels:        []string{"A_PORT_LABEL", "ANOTHER_PORT_LABEL"},
		RefreshInterval:   30 * time.Second,
	}, withPortLabels)
}

func TestValidateConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_endpoint_config.yaml"), factories)
	require.Error(t, err)
	require.EqualError(t, err, `extension "ecs_task_observer/with-invalid-endpoint" has invalid configuration: failed to parse ecs task metadata endpoint "_:invalid": parse "_:invalid": first path segment in URL cannot contain colon`)
}
