// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	testCases := []struct {
		name   string
		expect *Config
	}{
		{
			name:   "",
			expect: createDefaultConfig().(*Config),
		},
		{
			name: "custom",
			expect: &Config{
				HostIdentifiers:      []string{"k8s.node.name", "host.name", "host.id"},
				MetricsFlushInterval: 30 * time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			require.Equal(t, tc.expect, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name   string
		cfg    *Config
		errMsg string
	}{
		{
			name:   "valid",
			cfg:    createDefaultConfig().(*Config),
			errMsg: "",
		},
		{
			name:   "missing identifiers",
			cfg:    &Config{},
			errMsg: "at least one host identifier is required",
		},
		{
			name: "invalid flush interval",
			cfg: &Config{
				HostIdentifiers:      []string{"host.id"},
				MetricsFlushInterval: time.Hour,
			},
			errMsg: "\"1h0m0s\" is not a valid flush interval between 15s and 5m",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errMsg != "" {
				require.Equal(t, tc.errMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
