// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	var multipleErrors error

	multipleErrors = multierr.Combine(multipleErrors, errBadOrMissingEndpoint, errMissingUsername, errMissingPassword)

	tests := []struct {
		desc   string
		expect error
		conf   Config
	}{
		{
			desc:   "Missing password",
			expect: errMissingPassword,
			conf: Config{
				Username: "admin",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://localhost:8089",
				},
			},
		},
		{
			desc:   "Missing username",
			expect: errMissingUsername,
			conf: Config{
				Password: "securityFirst",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://localhost:8089",
				},
			},
		},
		{
			desc:   "Bad scheme (none http/s)",
			expect: errBadScheme,
			conf: Config{
				Password: "securityFirst",
				Username: "admin",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:8089",
				},
			},
		},
		{
			desc:   "Missing endpoint",
			expect: errBadOrMissingEndpoint,
			conf: Config{
				Username: "admin",
				Password: "securityFirst",
			},
		},
		{
			desc:   "Missing multiple",
			expect: multipleErrors,
			conf:   Config{},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			err := test.conf.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), test.expect.Error())
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	id := component.NewID(metadata.Type)
	cmSub, err := cm.Sub(id.String())
	require.NoError(t, err)

	testmetrics := metadata.DefaultMetricsBuilderConfig()
	testmetrics.Metrics.SplunkLicenseIndexUsage.Enabled = true
	testmetrics.Metrics.SplunkServerIntrospectionIndexerThroughput.Enabled = false

	expected := &Config{
		Username:          "admin",
		Password:          "securityFirst",
		MaxSearchWaitTime: 11 * time.Second,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "https://localhost:8089",
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
		},
		MetricsBuilderConfig: testmetrics,
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	require.NoError(t, component.UnmarshalConfig(cmSub, cfg))
	require.NoError(t, component.ValidateConfig(cfg))

	diff := cmp.Diff(expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{}))
	if diff != "" {
		t.Errorf("config mismatch (-expected / +actual)\n%s", diff)
	}
}
