// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing targets",
			cfg: &Config{
				Targets:          []*confignet.TCPAddrConfig{},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errMissingTargets,
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Targets: []*confignet.TCPAddrConfig{
					{
						Endpoint: "endpoint:  12efg",
						DialerConfig: confignet.DialerConfig{
							Timeout: 12 * time.Second,
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidEndpoint, "provided port is not a number:   12efg"),
		},
		{
			desc: "invalid config with multiple targets",
			cfg: &Config{
				Targets: []*confignet.TCPAddrConfig{
					{
						Endpoint: "endpoint:  12efg",
					},
					{
						Endpoint: "https://example.com:80",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidEndpoint, `provided port is not a number:   12efg; endpoint contains a scheme, which is not allowed: https://example.com:80`),
		},
		{
			desc: "port out of range",
			cfg: &Config{
				Targets: []*confignet.TCPAddrConfig{
					{
						Endpoint: "www.opentelemetry.io:67000",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidEndpoint, `provided port is out of valid range (1-65535): 67000`),
		},
		{
			desc: "missing port",
			cfg: &Config{
				Targets: []*confignet.TCPAddrConfig{
					{
						Endpoint: "www.opentelemetry.io/docs",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidEndpoint, `address www.opentelemetry.io/docs: missing port in address`),
		},
		{
			desc: "valid config",
			cfg: &Config{
				Targets: []*confignet.TCPAddrConfig{
					{
						Endpoint: "opentelemetry.io:443",
						DialerConfig: confignet.DialerConfig{
							Timeout: 3 * time.Second,
						},
					},
					{
						Endpoint: "opentelemetry.io:8080",
						DialerConfig: confignet.DialerConfig{
							Timeout: 1 * time.Second,
						},
					},
					{
						Endpoint: "111.222.33.44:10000",
						DialerConfig: confignet.DialerConfig{
							Timeout: 5 * time.Second,
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
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

//''' others: configtcp  test??
//func TestLoadConfig(t *testing.T) {
//	// load test config
//	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
//	require.NoError(t, err)
//	rcvrs, err := cm.Sub("receivers")
//	require.NoError(t, err)
//	tcpconf, err := rcvrs.Sub("tcpcheck")
//	require.NoError(t, err)
//	// unmarshal to receiver config
//	actualConfig, ok := NewFactory().CreateDefaultConfig().(*Config)
//	require.True(t, ok)
//	require.NoError(t, tcpconf.Unmarshal(actualConfig))
//
//	// set expected config
//	expectedConfig, ok := NewFactory().CreateDefaultConfig().(*Config)
//	require.True(t, ok)
//
//	expectedConfig.ControllerConfig = scraperhelper.ControllerConfig{
//		InitialDelay:       time.Second,
//		CollectionInterval: 60 * time.Second,
//	}
//	expectedConfig.TCPClientSettings = configtcp.TCPClientSettings{
//		Endpoint: "localhost:80",
//		Timeout:  10 * time.Second,
//	}
//	require.Equal(t, expectedConfig, actualConfig)
//}
//''''
