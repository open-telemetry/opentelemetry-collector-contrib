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
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://vcsa.host.localnet"
	expected.Username = "otelu"
	expected.Password = "${env:VCENTER_PASSWORD}"
	expected.MaxQueryMetrics = 128
	expected.MetricsBuilderConfig = metadata.NewDefaultMetricsBuilderConfig()
	expected.Metrics.VcenterHostCPUUtilization.Enabled = false
	expected.CollectionInterval = 5 * time.Minute

	if diff := cmp.Diff(expected, cfg,
		cmpopts.IgnoreFields(metadata.MetricsBuilderConfig{}, "Metrics"),
		cmpopts.IgnoreUnexported(metadata.ResourceAttributeConfig{}),
	); diff != "" {
		t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
	}

	require.False(t, cfg.(*Config).Metrics.VcenterHostCPUUtilization.Enabled)
}

func TestUIDFromURL(t *testing.T) {
	vc := &vcenterClient{}

	testCases := []struct {
		name        string
		url         string
		expectedUID string
		expectError bool
	}{
		{
			name:        "Valid URL with trailing slash",
			url:         "ds:///vmfs/volumes/634d4a87-9657ca9c-0517-4cd98f9a3b65/",
			expectedUID: "634d4a87-9657ca9c-0517-4cd98f9a3b65",
			expectError: false,
		},
		{
			name:        "Valid URL without trailing slash",
			url:         "ds:///vmfs/volumes/fca66b03-2ed734ff",
			expectedUID: "fca66b03-2ed734ff",
			expectError: false,
		},
		{
			name:        "Invalid URL with no parts",
			url:         "",
			expectedUID: "",
			expectError: true,
		},
		{
			name:        "URL with multiple trailing slashes",
			url:         "ds:///vmfs/volumes/fca66b03-2ed734ff//",
			expectedUID: "fca66b03-2ed734ff",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uid, err := vc.IDFromURL(tc.url)

			if tc.expectError {
				require.Error(t, err)
				require.Equal(t, tc.expectedUID, uid)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedUID, uid)
			}
		})
	}
}
