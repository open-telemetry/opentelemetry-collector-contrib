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

package prometheusexecreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewIDWithName(typeStr, "test"),
			expected: &Config{
				ScrapeInterval: 60 * time.Second,
				ScrapeTimeout:  10 * time.Second,
				Port:           9104,
				SubprocessConfig: subprocessmanager.SubprocessConfig{
					Command: "mysqld_exporter",
					Env:     []subprocessmanager.EnvConfig{},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "test2"),
			expected: &Config{
				ScrapeInterval: 90 * time.Second,
				ScrapeTimeout:  10 * time.Second,
				SubprocessConfig: subprocessmanager.SubprocessConfig{
					Command: "postgres_exporter",
					Env:     []subprocessmanager.EnvConfig{},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "end_to_end_test/1"),
			expected: &Config{
				ScrapeInterval: 1 * time.Second,
				ScrapeTimeout:  1 * time.Second,
				Port:           9999,
				SubprocessConfig: subprocessmanager.SubprocessConfig{
					Command: "go run ./testdata/end_to_end_metrics_test/test_prometheus_exporter.go {{port}}",
					Env: []subprocessmanager.EnvConfig{
						{
							Name:  "DATA_SOURCE_NAME",
							Value: "user:password@(hostname:port)/dbname",
						},
						{
							Name:  "SECONDARY_PORT",
							Value: "1234",
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "end_to_end_test/2"),
			expected: &Config{
				ScrapeInterval: 1 * time.Second,
				ScrapeTimeout:  1 * time.Second,
				SubprocessConfig: subprocessmanager.SubprocessConfig{
					Command: "go run ./testdata/end_to_end_metrics_test/test_prometheus_exporter.go {{port}}",
					Env:     []subprocessmanager.EnvConfig{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
