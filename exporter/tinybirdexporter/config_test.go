// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		subName      string
		expected     *Config
		errorMessage string
	}{
		{
			id:      component.NewIDWithName(component.MustNewType(typeStr), ""),
			subName: "tinybird",
			expected: &Config{
				Endpoint: "https://api.tinybird.co",
				Token:    "test-token",
				Metrics:  SignalConfig{Datasource: "metrics"},
				Traces:   SignalConfig{Datasource: "traces"},
				Logs:     SignalConfig{Datasource: "logs"},
			},
		},
		{
			id:      component.NewIDWithName(component.MustNewType(typeStr), "full"),
			subName: "tinybird/full",
			expected: &Config{
				Endpoint: "https://api.tinybird.co",
				Token:    "test-token",
				Metrics:  SignalConfig{Datasource: "metrics"},
				Traces:   SignalConfig{Datasource: "traces"},
				Logs:     SignalConfig{Datasource: "logs"},
			},
		},
		{
			id:      component.NewIDWithName(component.MustNewType(typeStr), "invalid_datasource"),
			subName: "tinybird/invalid_datasource",
			errorMessage: "metrics: invalid datasource \"metrics-with-dashes\": only letters, numbers, and underscores are allowed" + "\n" +
				"traces: invalid datasource \"traces-with-dashes\": only letters, numbers, and underscores are allowed" + "\n" +
				"logs: invalid datasource \"logs-with-dashes\": only letters, numbers, and underscores are allowed",
		},
		{
			id:           component.NewIDWithName(component.MustNewType(typeStr), "missing_token"),
			subName:      "tinybird/missing_token",
			errorMessage: "missing Tinybird API token",
		},
		{
			id:           component.NewIDWithName(component.MustNewType(typeStr), "missing_endpoint"),
			subName:      "tinybird/missing_endpoint",
			errorMessage: "missing Tinybird API endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
