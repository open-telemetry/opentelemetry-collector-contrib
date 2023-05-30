// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver/internal/metadata"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	var multierror error

	multierror = multierr.Append(multierror, errMissingPassword)
	multierror = multierr.Append(multierror, errMissingWarehouse)

	tests := []struct {
		desc   string
		expect error
		conf   Config
	}{
		{
			desc:   "Missing username all else present",
			expect: errMissingUsername,
			conf: Config{
				Username:  "",
				Password:  "password",
				Account:   "account",
				Warehouse: "warehouse",
			},
		},
		{
			desc:   "Missing password all else present",
			expect: errMissingPassword,
			conf: Config{
				Username:  "username",
				Password:  "",
				Account:   "account",
				Warehouse: "warehouse",
			},
		},
		{
			desc:   "Missing account all else present",
			expect: errMissingAccount,
			conf: Config{
				Username:  "username",
				Password:  "password",
				Account:   "",
				Warehouse: "warehouse",
			},
		},
		{
			desc:   "Missing warehouse all else present",
			expect: errMissingWarehouse,
			conf: Config{
				Username:  "username",
				Password:  "password",
				Account:   "account",
				Warehouse: "",
			},
		},
		{
			desc:   "Missing multiple check multierror",
			expect: multierror,
			conf: Config{
				Username:  "username",
				Password:  "",
				Account:   "account",
				Warehouse: "",
			},
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
	// LoadConf includes the TypeStr which NewFactory does not set
	id := component.NewIDWithName(metadata.Type, "")
	cmNoStr, err := cm.Sub(id.String())
	require.NoError(t, err)

	testMetrics := metadata.DefaultMetricsBuilderConfig()
	testMetrics.Metrics.SnowflakeDatabaseBytesScannedAvg.Enabled = true
	testMetrics.Metrics.SnowflakeQueryBytesDeletedAvg.Enabled = false

	expected := &Config{
		Username:  "snowflakeuser",
		Password:  "securepassword",
		Account:   "bigbusinessaccount",
		Warehouse: "metricWarehouse",
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 18 * time.Minute,
		},
		Role:                 "customMonitoringRole",
		Database:             "SNOWFLAKE",
		Schema:               "ACCOUNT_USAGE",
		MetricsBuilderConfig: testMetrics,
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	require.NoError(t, component.UnmarshalConfig(cmNoStr, cfg))
	assert.NoError(t, component.ValidateConfig(cfg))

	diff := cmp.Diff(expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{}))
	if diff != "" {
		t.Errorf("config mismatch (-expected / +actual)\n%s", diff)
	}
}
