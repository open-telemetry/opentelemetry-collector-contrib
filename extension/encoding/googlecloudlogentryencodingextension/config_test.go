// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/metadata"
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
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				HandleJSONPayloadAs:  "json",
				HandleProtoPayloadAs: "json",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "default"),
			expected: &Config{
				HandleJSONPayloadAs:  "json",
				HandleProtoPayloadAs: "json",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "json_as_text"),
			expected: &Config{
				HandleJSONPayloadAs:  "text",
				HandleProtoPayloadAs: "json",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "proto_as_text"),
			expected: &Config{
				HandleJSONPayloadAs:  "json",
				HandleProtoPayloadAs: "text",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "proto_as_protobuf"),
			expected: &Config{
				HandleJSONPayloadAs:  "json",
				HandleProtoPayloadAs: "protobuf",
			},
		},
	}

	for _, tt := range tests {
		name := strings.ReplaceAll(tt.id.String(), "/", "_")
		t.Run(name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.HandleJSONPayloadAs = "invalid"
	assert.Error(t, xconfmap.Validate(cfg))

	cfg = factory.CreateDefaultConfig().(*Config)
	cfg.HandleProtoPayloadAs = "invalid"
	assert.Error(t, xconfmap.Validate(cfg))
}
