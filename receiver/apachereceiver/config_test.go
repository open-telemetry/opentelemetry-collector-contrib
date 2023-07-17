// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachereceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		endpoint    string
		errExpected bool
		errText     string
	}{
		{
			desc:        "default_endpoint",
			endpoint:    "http://localhost:8080/server-status?auto",
			errExpected: false,
		},
		{
			desc:        "custom_host",
			endpoint:    "http://123.123.123.123:8080/server-status?auto",
			errExpected: false,
		},
		{
			desc:        "custom_port",
			endpoint:    "http://123.123.123.123:9090/server-status?auto",
			errExpected: false,
		},
		{
			desc:        "custom_path",
			endpoint:    "http://localhost:8080/my-status?auto",
			errExpected: false,
		},
		{
			desc:        "empty_path",
			endpoint:    "",
			errExpected: true,
			errText:     "missing hostname: ''",
		},
		{
			desc:        "missing_hostname",
			endpoint:    "http://:8080/server-status?auto",
			errExpected: true,
			errText:     "missing hostname: 'http://:8080/server-status?auto'",
		},
		{
			desc:        "missing_query",
			endpoint:    "http://localhost:8080/server-status",
			errExpected: true,
			errText:     "query must be 'auto': 'http://localhost:8080/server-status'",
		},
		{
			desc:        "invalid_query",
			endpoint:    "http://localhost:8080/server-status?nonsense",
			errExpected: true,
			errText:     "query must be 'auto': 'http://localhost:8080/server-status?nonsense'",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.Endpoint = tc.endpoint
			err := component.ValidateConfig(cfg)
			if tc.errExpected {
				require.EqualError(t, err, tc.errText)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://localhost:8080/server-status?auto"
	expected.CollectionInterval = 10 * time.Second

	require.Equal(t, expected, cfg)
}
