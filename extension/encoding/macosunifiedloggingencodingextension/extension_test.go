// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "macos_unified_logging_encoding", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg)

	config := cfg.(*Config)
	assert.False(t, config.ParsePrivateLogs)
	assert.True(t, config.IncludeSignpostEvents)
	assert.True(t, config.IncludeActivityEvents)
	assert.Equal(t, 65536, config.MaxLogSize)
}

func TestCreateExtension(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	ext, err := factory.Create(context.Background(), extensiontest.NewNopSettings(factory.Type()), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)

	err = ext.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = ext.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				MaxLogSize: 65536,
			},
			wantErr: false,
		},
		{
			name: "zero max log size",
			config: &Config{
				MaxLogSize: 0,
			},
			wantErr: true,
		},
		{
			name: "negative max log size",
			config: &Config{
				MaxLogSize: -1,
			},
			wantErr: true,
		},
		{
			name: "too large max log size",
			config: &Config{
				MaxLogSize: 2 * 1024 * 1024, // 2MB
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
