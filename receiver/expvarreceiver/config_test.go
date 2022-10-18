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

package expvarreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	metricCfg := metadata.DefaultMetricsSettings()
	metricCfg.ProcessRuntimeMemstatsTotalAlloc.Enabled = true
	metricCfg.ProcessRuntimeMemstatsMallocs.Enabled = false

	tests := []struct {
		id           config.ComponentID
		expected     config.Receiver
		errorMessage string
	}{
		{
			id:       config.NewComponentIDWithName(typeStr, "default"),
			expected: factory.CreateDefaultConfig(),
		},
		{
			id: config.NewComponentIDWithName(typeStr, "custom"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
					CollectionInterval: 30 * time.Second,
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8000/custom/path",
					Timeout:  time.Second * 5,
				},
				MetricsConfig: metricCfg,
			},
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "bad_schemeless_endpoint"),
			errorMessage: "scheme must be 'http' or 'https', but was 'localhost'",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "bad_hostless_endpoint"),
			errorMessage: "host not found in HTTP endpoint",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "bad_invalid_url"),
			errorMessage: "endpoint is not a valid URL: parse \"#$%^&*()_\": invalid URL escape \"%^&\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, cfg.Validate(), tt.errorMessage)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
