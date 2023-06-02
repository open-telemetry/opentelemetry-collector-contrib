// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/heroku"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cfg := confighttp.NewDefaultHTTPClientSettings()
	cfg.Timeout = 2 * time.Second

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "openshift"),
			expected: &Config{
				Detectors: []string{"openshift"},
				DetectorConfig: DetectorConfig{
					OpenShiftConfig: openshift.Config{
						Address: "127.0.0.1:4444",
						Token:   "some_token",
						TLSSettings: configtls.TLSClientSetting{
							Insecure: true,
						},
					},
				},
				HTTPClientSettings: cfg,
				Override:           false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "gcp"),
			expected: &Config{
				Detectors:          []string{"env", "gcp"},
				HTTPClientSettings: cfg,
				Override:           false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "ec2"),
			expected: &Config{
				Detectors: []string{"env", "ec2"},
				DetectorConfig: DetectorConfig{
					EC2Config: ec2.Config{
						Tags: []string{"^tag1$", "^tag2$"},
					},
				},
				HTTPClientSettings: cfg,
				Override:           false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "system"),
			expected: &Config{
				Detectors: []string{"env", "system"},
				DetectorConfig: DetectorConfig{
					SystemConfig: system.Config{
						HostnameSources: []string{"os"},
					},
				},
				HTTPClientSettings: cfg,
				Override:           false,
				Attributes:         []string{"a", "b"},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "heroku"),
			expected: &Config{
				Detectors:          []string{"env", "heroku"},
				HTTPClientSettings: cfg,
				Override:           false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "lambda"),
			expected: &Config{
				Detectors:          []string{"env", "lambda"},
				HTTPClientSettings: cfg,
				Override:           false,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid"),
			errorMessage: "hostname_sources contains invalid value: \"invalid_source\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
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
			assert.Equal(t, tt.expected, cfg)
		})
	}
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
		{
			name:                "Get Heroku Config",
			detectorType:        heroku.TypeStr,
			inputDetectorConfig: DetectorConfig{},
			expectedConfig:      nil,
		},
		{
			name:                "Get AWS Lambda Config",
			detectorType:        lambda.TypeStr,
			inputDetectorConfig: DetectorConfig{},
			expectedConfig:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.inputDetectorConfig.GetConfigFromType(tt.detectorType)
			assert.Equal(t, output, tt.expectedConfig)
		})
	}
}
