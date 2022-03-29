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

package resourcedetectionprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	p1 := cfg.Processors[config.NewComponentID(typeStr)]
	assert.Equal(t, p1, factory.CreateDefaultConfig())

	p2 := cfg.Processors[config.NewComponentIDWithName(typeStr, "gce")].(*Config)
	p2e := &Config{
		ProcessorSettings:  config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "gce")),
		Detectors:          []string{"env", "gce"},
		HTTPClientSettings: confighttp.HTTPClientSettings{Timeout: 2 * time.Second, MaxIdleConns: p2.MaxIdleConns, IdleConnTimeout: p2.IdleConnTimeout},
		Override:           false,
	}
	assert.Equal(t, p2, p2e)

	p3 := cfg.Processors[config.NewComponentIDWithName(typeStr, "ec2")]
	p3e := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "ec2")),
		Detectors:         []string{"env", "ec2"},
		DetectorConfig: DetectorConfig{
			EC2Config: ec2.Config{
				Tags: []string{"^tag1$", "^tag2$"},
			},
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{Timeout: 2 * time.Second, MaxIdleConns: p2.MaxIdleConns, IdleConnTimeout: p2.IdleConnTimeout},
		Override:           false,
	}
	assert.Equal(t, p3, p3e)

	p4 := cfg.Processors[config.NewComponentIDWithName(typeStr, "system")]
	p4e := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "system")),
		Detectors:         []string{"env", "system"},
		DetectorConfig: DetectorConfig{
			SystemConfig: system.Config{
				HostnameSources: []string{"os"},
			},
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{Timeout: 2 * time.Second, MaxIdleConns: p2.MaxIdleConns, IdleConnTimeout: p2.IdleConnTimeout},
		Override:           false,
		Attributes:         []string{"a", "b"},
	}
	assert.Equal(t, p4, p4e)
}

func TestLoadInvalidConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid_config.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)
}

func TestGetConfigFromType(t *testing.T) {
	tests := []struct {
		name                string
		detectorType        internal.DetectorType
		inputDetectorConfig DetectorConfig
		expectedConfig      internal.DetectorConfig
	}{
		{
			name:         "Get EC2 Config",
			detectorType: ec2.TypeStr,
			inputDetectorConfig: DetectorConfig{
				EC2Config: ec2.Config{
					Tags: []string{"tag1", "tag2"},
				},
			},
			expectedConfig: ec2.Config{
				Tags: []string{"tag1", "tag2"},
			},
		},
		{
			name:         "Get Nil Config",
			detectorType: internal.DetectorType("invalid input"),
			inputDetectorConfig: DetectorConfig{
				EC2Config: ec2.Config{
					Tags: []string{"tag1", "tag2"},
				},
			},
			expectedConfig: nil,
		},
		{
			name:         "Get System Config",
			detectorType: system.TypeStr,
			inputDetectorConfig: DetectorConfig{
				SystemConfig: system.Config{
					HostnameSources: []string{"os"},
				},
			},
			expectedConfig: system.Config{
				HostnameSources: []string{"os"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.inputDetectorConfig.GetConfigFromType(tt.detectorType)
			assert.Equal(t, output, tt.expectedConfig)
		})
	}
}
