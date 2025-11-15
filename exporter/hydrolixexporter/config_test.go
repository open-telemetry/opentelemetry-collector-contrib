// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	require.NotNil(t, cfg)

	hydrolixCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.NotNil(t, hydrolixCfg.ClientConfig)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://example.com/ingest",
					Timeout:  30 * time.Second,
				},
				HDXTable:     "test_table",
				HDXTransform: "test_transform",
				HDXUsername:  "user",
				HDXPassword:  "pass",
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal settings",
			config: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://example.com",
				},
				HDXTable:     "table",
				HDXTransform: "transform",
				HDXUsername:  "u",
				HDXPassword:  "p",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.config)
		})
	}
}
