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

package datasetexporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

type SuiteFactory struct{}

func (s *SuiteFactory) PreTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteFactory) PostTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteFactory) Destroy(t *td.T) error {
	os.Clearenv()
	return nil
}

func TestSuiteFactory(t *testing.T) {
	td.NewT(t)
	tdsuite.Run(t, &SuiteFactory{})
}

func (s *SuiteFactory) TestCreateDefaultConfig(assert, require *td.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Cmp(cfg, &Config{
		MaxDelayMs:      MaxDelayMs,
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}, cfg, "failed to create default config")

	assert.Nil(componenttest.CheckConfigStruct(cfg))
}

func (s *SuiteFactory) TestLoadConfig(assert, require *td.T) {

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	assert.Nil(err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(CfgTypeStr, "minimal"),
			expected: &Config{
				DatasetURL:      "https://app.scalyr.com",
				APIKey:          "key-minimal",
				MaxDelayMs:      MaxDelayMs,
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
		},
		{
			id: component.NewIDWithName(CfgTypeStr, "lib"),
			expected: &Config{
				DatasetURL:      "https://app.eu.scalyr.com",
				APIKey:          "key-lib",
				MaxDelayMs:      "12345",
				GroupBy:         []string{"attributes.container_id", "attributes.log.file.path"},
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
		},
		{
			id: component.NewIDWithName(CfgTypeStr, "full"),
			expected: &Config{
				DatasetURL: "https://app.scalyr.com",
				APIKey:     "key-full",
				MaxDelayMs: "3456",
				GroupBy:    []string{"body.map.kubernetes.pod_id", "body.map.kubernetes.docker_id", "body.map.stream"},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             true,
					InitialInterval:     11 * time.Nanosecond,
					RandomizationFactor: 11.3,
					Multiplier:          11.6,
					MaxInterval:         12 * time.Nanosecond,
					MaxElapsedTime:      13 * time.Nanosecond,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 14,
					QueueSize:    15,
				},
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 16 * time.Nanosecond,
				},
			},
		},
	}

	for _, tt := range tests {
		require.Run(tt.id.Name(), func(*td.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.Nil(err)
			require.Nil(component.UnmarshalConfig(sub, cfg))

			if assert.Nil(component.ValidateConfig(cfg)) {
				assert.Cmp(cfg, tt.expected)
			}
		})
	}
}

type CreateTest struct {
	name          string
	config        component.Config
	expectedError error
}

func createExporterTests() []CreateTest {
	return []CreateTest{
		{
			name: "valid",
			config: Config{
				DatasetURL:      "https://app.eu.scalyr.com",
				APIKey:          "key-lib",
				MaxDelayMs:      "12345",
				GroupBy:         []string{"attributes.container_id"},
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
			expectedError: nil,
		},
	}
}

func (s *SuiteFactory) TestCreateLogsExporter(assert, require *td.T) {
	ctx := context.Background()
	createSettings := exportertest.NewNopCreateSettings()
	tests := createExporterTests()

	for _, tt := range tests {
		require.Run(tt.name, func(*td.T) {
			exporterInstance = nil
			logs, err := createLogsExporter(ctx, createSettings, tt.config)

			if err == nil {
				assert.Nil(tt.expectedError)
			} else {
				assert.Cmp(err.Error(), tt.expectedError.Error())
				assert.Nil(logs)
			}
		})
	}
}

func (s *SuiteFactory) TestCreateMetricsExporter(assert, require *td.T) {
	ctx := context.Background()
	createSettings := exportertest.NewNopCreateSettings()
	tests := createExporterTests()

	for _, tt := range tests {
		require.Run(tt.name, func(*td.T) {
			exporterInstance = nil
			logs, err := createMetricsExporter(ctx, createSettings, tt.config)

			if err == nil {
				assert.Nil(tt.expectedError)
			} else {
				assert.Cmp(err.Error(), tt.expectedError.Error())
				assert.Nil(logs)
			}
		})
	}
}

func (s *SuiteFactory) TestCreateTracesExporter(assert, require *td.T) {
	ctx := context.Background()
	createSettings := exportertest.NewNopCreateSettings()
	tests := createExporterTests()

	for _, tt := range tests {
		require.Run(tt.name, func(*td.T) {
			exporterInstance = nil
			logs, err := createTracesExporter(ctx, createSettings, tt.config)

			if err == nil {
				assert.Nil(tt.expectedError)
			} else {
				assert.Cmp(err.Error(), tt.expectedError.Error())
				assert.Nil(logs)
			}
		})
	}
}
