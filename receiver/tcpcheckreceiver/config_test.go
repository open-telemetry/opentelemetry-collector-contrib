// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"
)

// check that OTel Collector patterns are implemented
func TestCheckConfig(t *testing.T) {
	t.Parallel()
	if err := componenttest.CheckConfigStruct(&Config{}); err != nil {
		t.Fatal(err)
	}
}

// test the validate function for config
func TestValidate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing endpoint",
			cfg: &Config{
				TCPClientSettings: configtcp.TCPClientSettings{
					Endpoint: "",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(errMissingEndpoint),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				TCPClientSettings: configtcp.TCPClientSettings{
					Endpoint: "badendpoint . cuz spaces:443",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(errInvalidEndpoint),
		},
		{
			desc: "no error",
			cfg: &Config{
				TCPClientSettings: configtcp.TCPClientSettings{
					Endpoint: "localhost:8080",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: error(nil),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.EqualError(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// load test config
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	rcvrs, err := cm.Sub("receivers")
	require.NoError(t, err)
	tcpconf, err := rcvrs.Sub("tcpcheck")
	require.NoError(t, err)
	// unmarshal to receiver config
	actualConfig, ok := NewFactory().CreateDefaultConfig().(*Config)
	require.True(t, ok)
	require.NoError(t, tcpconf.Unmarshal(actualConfig))

	// set expected config
	expectedConfig, ok := NewFactory().CreateDefaultConfig().(*Config)
	require.True(t, ok)

	expectedConfig.ControllerConfig = scraperhelper.ControllerConfig{
		InitialDelay:       time.Second,
		CollectionInterval: 60 * time.Second,
	}
	expectedConfig.TCPClientSettings = configtcp.TCPClientSettings{
		Endpoint: "localhost:80",
		Timeout:  10 * time.Second,
	}
	require.Equal(t, expectedConfig, actualConfig)
}
