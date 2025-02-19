// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t,
		&Config{
			ControllerConfig: scraperhelper.ControllerConfig{
				CollectionInterval: 120 * time.Second,
				InitialDelay:       1 * time.Second,
			},
			ProjectID: "my-project-id",
			MetricsList: []MetricConfig{
				{
					MetricName: "compute.googleapis.com/instance/cpu/usage_time",
				},
				{
					MetricName: "connectors.googleapis.com/flex/instance/cpu/usage_time",
				},
				{
					MetricDescriptorFilter: "metric.type = starts_with(\"compute.googleapis.com\")",
				},
			},
		},
		cfg,
	)
}

func TestValidateService(t *testing.T) {
	testCases := map[string]struct {
		metric       MetricConfig
		requireError bool
	}{
		"Valid Service": {
			MetricConfig{
				MetricName: "metric_name",
			}, false,
		},
		"Empty MetricName": {
			MetricConfig{
				MetricName: "",
			}, true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			err := testCase.metric.Validate()
			if testCase.requireError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	validMetric := MetricConfig{
		MetricName: "metric_name",
	}

	testCases := map[string]struct {
		metricsList        []MetricConfig
		collectionInterval time.Duration
		requireError       bool
	}{
		"Valid Config":                {[]MetricConfig{validMetric}, 300 * time.Second, false},
		"Empty Services":              {nil, 300 * time.Second, true},
		"Invalid Service in Services": {[]MetricConfig{{}}, 300 * time.Second, true},
		"Invalid Collection Interval": {[]MetricConfig{validMetric}, 0 * time.Second, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: testCase.collectionInterval,
				},
				MetricsList: testCase.metricsList,
			}

			err := cfg.Validate()
			if testCase.requireError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
