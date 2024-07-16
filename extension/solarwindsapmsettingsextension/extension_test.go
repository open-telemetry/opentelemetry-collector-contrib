// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

func TestCreateExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "default",
			cfg: &Config{
				Endpoint: DefaultEndpoint,
				Interval: DefaultInterval,
			},
		},
		{
			name: "anything",
			cfg: &Config{
				Endpoint: "apm.collector.na-02.cloud.solarwinds.com:443",
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := createAnExtension(tt.cfg, t)
			require.NoError(t, ex.Shutdown(context.TODO()))
		})
	}
}

// create extension
func createAnExtension(c *Config, t *testing.T) extension.Extension {
	logger, err := zap.NewProduction()
	require.NoError(t, err)
	ex, err := newSolarwindsApmSettingsExtension(c, logger)
	require.NoError(t, err)
	require.NoError(t, ex.Start(context.TODO(), nil))
	return ex
}
