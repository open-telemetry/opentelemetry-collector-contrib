// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

// TestConfig ensures a config created with the factory is the same as one created manually with
// the exported Config struct.
func TestConfig(t *testing.T) {
	factory := Factory{}
	defaultConfig := factory.CreateDefaultConfig()

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 15 * time.Second

	expectedConfig := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		ClientConfig:         clientConfig,
		ConcurrencyLimit:     50,
		MergedPRLookbackDays: 30,
	}

	assert.Equal(t, expectedConfig, defaultConfig)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with concurrency limit",
			config: Config{
				ConcurrencyLimit: 50,
			},
			wantErr: false,
		},
		{
			name: "valid config with zero concurrency (unlimited)",
			config: Config{
				ConcurrencyLimit: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid config with negative concurrency",
			config: Config{
				ConcurrencyLimit: -1,
			},
			wantErr: true,
		},
		{
			name: "valid config with lookback days",
			config: Config{
				ConcurrencyLimit:     50,
				MergedPRLookbackDays: 30,
			},
			wantErr: false,
		},
		{
			name: "valid config with zero lookback (unlimited)",
			config: Config{
				ConcurrencyLimit:     50,
				MergedPRLookbackDays: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid config with negative lookback",
			config: Config{
				ConcurrencyLimit:     50,
				MergedPRLookbackDays: -5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
