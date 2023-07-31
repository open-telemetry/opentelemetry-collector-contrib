// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

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
				Endpoint:                  "",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: errors.New("no endpoint was provided"),
		},
		{
			desc: "with endpoint",
			cfg: Config{
				Endpoint:                  "http://vcsa.some-host",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
		},
		{
			desc: "not http or https",
			cfg: Config{
				Endpoint:                  "ws://vcsa.some-host",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparseable URL",
			cfg: Config{
				Endpoint:                  "h" + string(rune(0x7f)),
				TLSClientSetting:          configtls.TLSClientSetting{},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: errors.New("unable to parse url"),
		},
		{
			desc: "no username",
			cfg: Config{
				Endpoint:                  "https://vcsa.some-host",
				Password:                  "otelp",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: errors.New("username not provided"),
		},
		{
			desc: "no password",
			cfg: Config{
				Endpoint:                  "https://vcsa.some-host",
				Username:                  "otelu",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: errors.New("password not provided"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://vcsa.host.localnet"
	expected.Username = "otelu"
	expected.Password = "${env:VCENTER_PASSWORD}"
	expected.MetricsBuilderConfig = metadata.DefaultMetricsBuilderConfig()
	expected.MetricsBuilderConfig.Metrics.VcenterHostCPUUtilization.Enabled = false
	expected.CollectionInterval = 5 * time.Minute

	if diff := cmp.Diff(expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{})); diff != "" {
		t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
	}

}
