// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
			id:       component.NewIDWithName(metadata.Type, "default"),
			expected: factory.CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
					Timeout:            5 * time.Second,
				},
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8000/custom/path",
					Timeout:  5 * time.Second,
				},
				MetricsBuilderConfig: metricCfg,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_schemeless_endpoint"),
			errorMessage: "scheme must be 'http' or 'https', but was 'localhost'",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_hostless_endpoint"),
			errorMessage: "host not found in HTTP endpoint",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_invalid_url"),
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
