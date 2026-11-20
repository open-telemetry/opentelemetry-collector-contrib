// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				Region:    "ap-beijing",
				LogSet:    "demo-logset",
				Topic:     "demo-topic",
				SecretID:  "demo-secret-id",
				SecretKey: "demo-secret-key",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				Endpoint:   "ap-beijing.cls.tencentcs.com",
				LogSet:     "demo-logset",
				Topic:      "demo-topic",
				SecretID:   "demo-secret-id",
				SecretKey:  "demo-secret-key",
				IsIntranet: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name:        "missing logset and topic",
			config:      &Config{Region: "ap-beijing"},
			expectedErr: "missing tencentcloudlogservice params: LogSet, Topic",
		},
		{
			name:        "missing region and endpoint",
			config:      &Config{LogSet: "demo-logset", Topic: "demo-topic"},
			expectedErr: "missing tencentcloudlogservice param: Region or Endpoint",
		},
		{
			name:   "region only",
			config: &Config{Region: "ap-beijing", LogSet: "demo-logset", Topic: "demo-topic"},
		},
		{
			name:   "endpoint only",
			config: &Config{Endpoint: "cls.example.com", LogSet: "demo-logset", Topic: "demo-topic"},
		},
		{
			name: "endpoint with intranet",
			config: &Config{
				Endpoint:   "ap-beijing.cls.tencentyun.com",
				LogSet:     "demo-logset",
				Topic:      "demo-topic",
				IsIntranet: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.expectedErr)
		})
	}
}
