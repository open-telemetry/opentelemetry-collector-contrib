// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expvarreceiver

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	metricCfg := metadata.NewDefaultMetricsBuilderConfig()
	metricCfg.Metrics.ProcessRuntimeMemstatsTotalAlloc.Enabled = true
	metricCfg.Metrics.ProcessRuntimeMemstatsMallocs.Enabled = false
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "http://localhost:8000/custom/path"
	clientConfig.Timeout = time.Second * 5
	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:       component.NewIDWithName(metadata.Type, "default"),
			expected: factory.CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom"),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       time.Second,
					Timeout:            time.Second * 5,
				},
				ClientConfig:         clientConfig,
				MetricsBuilderConfig: metricCfg,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_schemeless_endpoint"),
			errorMessage: "scheme must be 'http' or 'https', but was 'localhost'",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_hostless_endpoint"),
			errorMessage: "host not found in HTTP endpoint",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_invalid_url"),
			errorMessage: "endpoint is not a valid URL: parse \"#$%^&*()_\": invalid URL escape \"%^&\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config", "config.yaml"))
			require.NoError(t, err)

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
			if diff := cmp.Diff(tt.expected, cfg,
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
