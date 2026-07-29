// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectorydsreceiver

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	overriddenMetricsBuilderConfig := metadata.NewDefaultMetricsBuilderConfig()
	overriddenMetricsBuilderConfig.Metrics.ActiveDirectoryDsReplicationObjectRate.Enabled = false
	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, "defaults"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 2 * time.Minute,
					InitialDelay:       time.Second,
				},
				MetricsBuilderConfig: overriddenMetricsBuilderConfig,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			if diff := cmp.Diff(
				tt.expected,
				cfg,
				// mdatagen gives metric and resource attribute configs an unexported enabledSetByUser,
				// set from parser.IsSet("enabled"), so it is only true on the unmarshaled side:
				// https://github.com/open-telemetry/opentelemetry-collector/blob/e4e58cda0aa6d5d4d275ff12072ae418410e6ae7/cmd/mdatagen/internal/templates/config.go.tmpl#L42-L44
				cmp.FilterPath(
					func(fp cmp.Path) bool {
						return fp.Last().String() == ".enabledSetByUser"
					},
					cmp.Ignore(),
				),
				// Allow go-cmp to read unexported fields instead of panicking on them, so new
				// upstream fields can't break this (https://pkg.go.dev/github.com/google/go-cmp/cmp#Exporter).
				cmp.Exporter(func(reflect.Type) bool { return true }),
			); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}
