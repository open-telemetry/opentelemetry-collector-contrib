// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	// Endpoint doesn't have a default value so set it directly.
	defaultCfg := createDefaultConfig().(*Config)
	defaultCfg.Endpoint = "cn-hangzhou.log.aliyuncs.com"

	tests := []struct {
		id       component.ID
		expected component.Config
	}{

		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				Endpoint:        "cn-hangzhou.log.aliyuncs.com",
				Project:         "demo-project",
				Logstore:        "demo-logstore",
				AccessKeyID:     "test-id",
				AccessKeySecret: "test-secret",
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
