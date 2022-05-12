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

package expvarreceiver

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
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 2, len(cfg.Receivers))

	// Validate default config
	expectedCfg := factory.CreateDefaultConfig().(*Config)
	expectedCfg.SetIDName("default")

	assert.Equal(t, expectedCfg, cfg.Receivers[config.NewComponentIDWithName(typeStr, "default")])

	// Validate custom config
	expectedCfg = factory.CreateDefaultConfig().(*Config)
	expectedCfg.SetIDName("custom")
	expectedCfg.CollectionInterval = time.Second * 30
	expectedCfg.HTTP = &confighttp.HTTPClientSettings{
		Endpoint: "http://localhost:8000/custom/path",
		Timeout:  time.Second * 5,
	}
	expectedCfg.MetricsConfig = []MetricConfig{
		{
			Name:    "example_metric.enabled",
			Enabled: true,
		},
		{
			Name:    "example_metric.disabled",
			Enabled: false,
		},
	}

	assert.Equal(t, expectedCfg, cfg.Receivers[config.NewComponentIDWithName(typeStr, "custom")])
}
