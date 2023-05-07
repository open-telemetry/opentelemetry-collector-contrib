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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	metricCfg := metadata.DefaultMetricsBuilderConfig()
	metricCfg.Metrics.ProcessRuntimeMemstatsTotalAlloc.Enabled = true
	metricCfg.Metrics.ProcessRuntimeMemstatsMallocs.Enabled = false

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:       component.NewIDWithName(typeStr, "default"),
			expected: factory.CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(typeStr, "custom"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 30 * time.Second,
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8000/custom/path",
					Timeout:  time.Second * 5,
				},
				MetricsBuilderConfig: metricCfg,
			},
		},
		{
			id:           component.NewIDWithName(typeStr, "bad_schemeless_endpoint"),
			errorMessage: "scheme must be 'http' or 'https', but was 'localhost'",
		},
		{
			id:           component.NewIDWithName(typeStr, "bad_hostless_endpoint"),
			errorMessage: "host not found in HTTP endpoint",
		},
		{
			id:           component.NewIDWithName(typeStr, "bad_invalid_url"),
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			if diff := cmp.Diff(tt.expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{})); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}
