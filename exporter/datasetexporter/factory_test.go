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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type SuiteFactory struct {
	suite.Suite
}

func (s *SuiteFactory) SetupTest() {
	os.Clearenv()
}

func TestSuiteFactory(t *testing.T) {
	suite.Run(t, new(SuiteFactory))
}

func (s *SuiteFactory) TestCreateDefaultConfig() {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	s.Equal(&Config{
		BufferSettings:  newDefaultBufferSettings(),
		TracesSettings:  newDefaultTracesSettings(),
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}, cfg, "failed to create default config")

	s.Nil(componenttest.CheckConfigStruct(cfg))
}

func (s *SuiteFactory) TestLoadConfig() {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	s.Nil(err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(CfgTypeStr, "minimal"),
			expected: &Config{
				DatasetURL:      "https://app.scalyr.com",
				APIKey:          "key-minimal",
				BufferSettings:  newDefaultBufferSettings(),
				TracesSettings:  newDefaultTracesSettings(),
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
		},
		{
			id: component.NewIDWithName(CfgTypeStr, "lib"),
			expected: &Config{
				DatasetURL: "https://app.eu.scalyr.com",
				APIKey:     "key-lib",
				BufferSettings: BufferSettings{
					MaxLifetime:          345 * time.Millisecond,
					GroupBy:              []string{"attributes.container_id", "attributes.log.file.path"},
					RetryInitialInterval: bufferRetryInitialInterval,
					RetryMaxInterval:     bufferRetryMaxInterval,
					RetryMaxElapsedTime:  bufferRetryMaxElapsedTime,
				},
				TracesSettings:  newDefaultTracesSettings(),
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
				BufferSettings: BufferSettings{
					MaxLifetime:          3456 * time.Millisecond,
					GroupBy:              []string{"body.map.kubernetes.pod_id", "body.map.kubernetes.docker_id", "body.map.stream"},
					RetryInitialInterval: 21 * time.Second,
					RetryMaxInterval:     22 * time.Second,
					RetryMaxElapsedTime:  23 * time.Second,
				},
				TracesSettings: TracesSettings{
					MaxWait: 3 * time.Second,
				},
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
		s.T().Run(tt.id.Name(), func(*testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			s.Require().Nil(err)
			s.Require().Nil(component.UnmarshalConfig(sub, cfg))
			if s.Nil(component.ValidateConfig(cfg)) {
				s.Equal(tt.expected, cfg)
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
			name:          "broken",
			config:        &Config{},
			expectedError: fmt.Errorf("cannot get DataSetExpoter: cannot convert config: DatasetURL: ; BufferSettings: {MaxLifetime:0s GroupBy:[] RetryInitialInterval:0s RetryMaxInterval:0s RetryMaxElapsedTime:0s}; TracesSettings: {Aggregate:false MaxWait:0s}; RetrySettings: {Enabled:false InitialInterval:0s RandomizationFactor:0 Multiplier:0 MaxInterval:0s MaxElapsedTime:0s}; QueueSettings: {Enabled:false NumConsumers:0 QueueSize:0 StorageID:<nil>}; TimeoutSettings: {Timeout:0s}; config is not valid: api_key is required"),
		},
		{
			name: "valid",
			config: &Config{
				DatasetURL: "https://app.eu.scalyr.com",
				APIKey:     "key-lib",
				BufferSettings: BufferSettings{
					MaxLifetime: 12345,
					GroupBy:     []string{"attributes.container_id"},
				},
				TracesSettings: TracesSettings{
					Aggregate: true,
					MaxWait:   5 * time.Second,
				},
				RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
				QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
				TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
			},
			expectedError: nil,
		},
	}
}
