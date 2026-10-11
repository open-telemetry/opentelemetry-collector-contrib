// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		desc        string
		cfg         Config
		expectedErr error
	}{
		{
			desc: "empty endpoint",
			cfg: Config{
				Endpoint:         "",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("no endpoint was provided"),
		},
		{
			desc: "with endpoint",
			cfg: Config{
				Endpoint:         "http://vcsa.some-host",
				Username:         "otelu",
				Password:         "otelp",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			desc: "not http or https",
			cfg: Config{
				Endpoint:         "ws://vcsa.some-host",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparsable URL",
			cfg: Config{
				Endpoint:         "h" + string(rune(0x7f)),
				ClientConfig:     configtls.ClientConfig{},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("unable to parse url"),
		},
		{
			desc: "no username",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Password:         "otelp",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("username not provided"),
		},
		{
			desc: "no password",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("password not provided"),
		},
		{
			desc: "socks5 proxy_url",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				Password:         "otelp",
				ProxyURL:         "socks5://proxy.some-host:1080",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			desc: "http proxy_url",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				Password:         "otelp",
				ProxyURL:         "http://proxy.some-host:8080",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			desc: "proxy_url with unsupported scheme",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				Password:         "otelp",
				ProxyURL:         "sock5://proxy.some-host:1080",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("proxy_url scheme must be http, https, socks5 or socks5h"),
		},
		{
			desc: "proxy_url without scheme",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				Password:         "otelp",
				ProxyURL:         "proxy.some-host:1080",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("proxy_url scheme must be http, https, socks5 or socks5h"),
		},
		{
			desc: "proxy_url without host",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				Password:         "otelp",
				ProxyURL:         "socks5://",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("proxy_url must include a host"),
		},
		{
			desc: "unparsable proxy_url",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				Password:         "otelp",
				ProxyURL:         "h" + string(rune(0x7f)),
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("unable to parse proxy_url"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
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
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://vcsa.host.localnet"
	expected.Username = "otelu"
	expected.Password = "${env:VCENTER_PASSWORD}"
	expected.ProxyURL = "socks5://proxy.host.localnet:1080"
	expected.MaxQueryMetrics = 128
	expected.MetricsBuilderConfig = metadata.NewDefaultMetricsBuilderConfig()
	expected.MetricsBuilderConfig.Metrics.VcenterHostCPUUtilization.Enabled = false
	expected.ControllerConfig.CollectionInterval = 5 * time.Minute

	if diff := cmp.Diff(expected, cfg,
		cmpopts.IgnoreFields(metadata.MetricsBuilderConfig{}, "Metrics"),
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

	require.False(t, cfg.(*Config).MetricsBuilderConfig.Metrics.VcenterHostCPUUtilization.Enabled)
}
