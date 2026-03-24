// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsefareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	require.NotNil(t, cfg)

	efaCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Equal(t, scraperhelper.NewDefaultControllerConfig(), efaCfg.ControllerConfig)
	assert.Equal(t, metadata.DefaultMetricsBuilderConfig(), efaCfg.MetricsBuilderConfig)
	assert.Empty(t, efaCfg.HostPath)
}

func TestCreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig()
	settings := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	recv, err := createMetricsReceiver(t.Context(), settings, cfg, consumer)
	require.NoError(t, err)
	require.NotNil(t, recv)
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)
	assert.Equal(t, metadata.Type, f.Type())
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		hostPath string
		wantErr  string
		wantPath string
	}{
		{"empty", "", "", ""},
		{"absolute", "/host", "", "/host"},
		{"relative", "relative/path", "must be an absolute path", ""},
		{"trailing_slash", "/host/", "", "/host/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.HostPath = tt.hostPath
			err := cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				if tt.wantPath != "" {
					assert.Equal(t, tt.wantPath, cfg.HostPath)
				}
			}
		})
	}
}
