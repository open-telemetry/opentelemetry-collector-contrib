// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"os"
	"testing"

	"github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"gopkg.in/yaml.v3"
)

var scrapeFileTargetPage = `
# A simple counter
# TYPE foo counter
foo1 1
`

// TestScrapeConfigFiles validates that scrape configs provided via scrape_config_files are
// also considered and added to the applied configuration

func TestScrapeConfigFiles(t *testing.T) {
	setMetricsTimestamp()
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: scrapeFileTargetPage},
			},
			validateFunc: verifyScrapeConfigFiles,
		},
	}

	testComponent(t, targets, func(cfg *Config) {
		// take the generated scrape config and move it into a file instead
		marshalledScrapeConfigs, err := yaml.Marshal(cfg.PrometheusConfig.ScrapeConfigs)
		require.NoError(t, err)
		tmpDir := t.TempDir()
		cfgFileName := tmpDir + "/test-scrape-config.yaml"
		scrapeConfigFileContent := "scrape_configs:\n" + string(marshalledScrapeConfigs)
		err = os.WriteFile(cfgFileName, []byte(scrapeConfigFileContent), 0o400)
		require.NoError(t, err)
		cfg.PrometheusConfig.ScrapeConfigs = []*config.ScrapeConfig{}
		cfg.PrometheusConfig.ScrapeConfigFiles = []string{cfgFileName}
	})
}

func verifyScrapeConfigFiles(t *testing.T, _ *testData, result []pmetric.ResourceMetrics) {
	require.Len(t, result, 1)
	serviceName, ok := result[0].Resource().Attributes().Get(string(semconv.ServiceNameKey))
	assert.True(t, ok)
	assert.Equal(t, "target1", serviceName.Str())
	assert.Equal(t, 6, result[0].ScopeMetrics().At(0).Metrics().Len())
	metricFound := false

	for i := 0; i < result[0].ScopeMetrics().At(0).Metrics().Len(); i++ {
		if result[0].ScopeMetrics().At(0).Metrics().At(i).Name() == "foo1" {
			metricFound = true
			break
		}
	}
	assert.True(t, metricFound)
}
