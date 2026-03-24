// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id                 component.ID
		expected           component.Config
		expectedErrMessage string
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				HTTP: &confighttp.ServerConfig{NetAddr: confignet.AddrConfig{Endpoint: "test:123", Transport: confignet.TransportTypeTCP}},
				Auth: component.MustNewID("azureauth"),
				Logs: EncodingConfig{Encoding: component.MustNewID("azure_encoding")},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "no_auth"),
			expected: &Config{
				HTTP: &confighttp.ServerConfig{NetAddr: confignet.AddrConfig{Endpoint: "test:123", Transport: confignet.TransportTypeTCP}},
				Logs: EncodingConfig{Encoding: component.MustNewID("azure_encoding")},
			},
		},
		{
			id:                 component.NewIDWithName(metadata.Type, "no_http"),
			expectedErrMessage: "missing http server settings",
		},
		{
			id:                 component.NewIDWithName(metadata.Type, "missing_logs_encoding"),
			expectedErrMessage: "logs.encoding must be set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			assert.NoError(t, err)
			assert.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErrMessage != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErrMessage)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
