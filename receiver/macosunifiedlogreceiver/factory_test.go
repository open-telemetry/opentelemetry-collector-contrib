// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedlogreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedlogreceiver/internal/metadata"
)

func TestFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg)

	config := cfg.(*Config)
	assert.Empty(t, config.Encoding)
	assert.NotNil(t, config.Config)
	assert.NotNil(t, config.BaseConfig)
}

func TestCreateLogsReceiver(t *testing.T) {
	cfg := &Config{
		Config: fileconsumer.Config{
			Criteria: matcher.Criteria{
				Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
			},
		},
		Encoding: "macosunifiedlogencoding", // Correct encoding name
	}

	// Should succeed in creation, but Start will fail without proper extension
	_, err := createLogsReceiver(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	assert.NoError(t, err) // Creation succeeds, but Start will fail without extension
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
				},
				Encoding: "macosunifiedlogencoding", // Correct encoding name
			},
			expectErr: false,
		},
		{
			name: "missing encoding",
			config: &Config{
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
				},
				Encoding: "",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
