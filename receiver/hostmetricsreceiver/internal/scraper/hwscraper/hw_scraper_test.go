// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("hw scraper only supported on linux")
	}

	type testCase struct {
		name                string
		config              *Config
		expectedMetricCount int
		expectedErr         string
	}

	testCases := []testCase{
		{
			name: "Standard",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				HwmonPath:            createTestHwmonData(t),
				Temperature: &TemperatureConfig{
					Include: MatchConfig{
						Config:  filterset.Config{MatchType: filterset.Regexp},
						Sensors: []string{".*"},
					},
				},
			},
			expectedMetricCount: 2,
		},
		{
			name: "No hwmon path",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				HwmonPath:            "/nonexistent/path",
				Temperature: &TemperatureConfig{
					Include: MatchConfig{
						Config:  filterset.Config{MatchType: filterset.Regexp},
						Sensors: []string{".*"},
					},
				},
			},
			expectedMetricCount: 0,
		},
		{
			name: "No temperature config",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				HwmonPath:            createTestHwmonData(t),
				Temperature:          nil,
			},
			expectedMetricCount: 0,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newHwScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), test.config)
			require.NotNil(t, scraper)

			err := scraper.start(t.Context(), componenttest.NewNopHost())
			if test.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
				return
			}
			assert.NoError(t, err)

			metrics, err := scraper.scrape(t.Context())
			assert.NoError(t, err)
			validateMetrics(t, metrics, test.expectedMetricCount)
		})
	}
}

func TestScrapeOnNonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Testing non-Linux behavior")
	}

	config := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		HwmonPath:            "/sys/class/hwmon",
		Temperature: &TemperatureConfig{
			Include: MatchConfig{
				Sensors: []string{".*"},
			},
		},
	}

	scraper := newHwScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), config)
	require.NotNil(t, scraper)

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hwmon not available")
}

func validateMetrics(t *testing.T, metrics pmetric.Metrics, expectedCount int) {
	assert.Equal(t, expectedCount, metrics.MetricCount())

	if expectedCount == 0 {
		return
	}

	resourceMetrics := metrics.ResourceMetrics()
	require.Positive(t, resourceMetrics.Len())

	scopeMetrics := resourceMetrics.At(0).ScopeMetrics()
	require.Positive(t, scopeMetrics.Len())

	hwMetrics := scopeMetrics.At(0).Metrics()
	assert.Equal(t, expectedCount, hwMetrics.Len())

	// Validate temperature metrics
	for i := 0; i < hwMetrics.Len(); i++ {
		metric := hwMetrics.At(i)
		assert.Contains(t, []string{"hw.temperature", "hw.temperature.limit", "hw.status"}, metric.Name())
		switch metric.Name() {
		case "hw.temperature", "hw.temperature.limit":
			assert.Positive(t, metric.Gauge().DataPoints().Len())
		case "hw.status":
			assert.Positive(t, metric.Sum().DataPoints().Len())
		}
	}
}

// createTestHwmonData creates a temporary directory structure mimicking hwmon
func createTestHwmonData(t *testing.T) string {
	tempDir := t.TempDir()
	hwmonDir := filepath.Join(tempDir, "hwmon0")

	err := os.MkdirAll(hwmonDir, 0o755)
	require.NoError(t, err)

	// Create device name file
	nameFile := filepath.Join(hwmonDir, "name")
	err = os.WriteFile(nameFile, []byte("coretemp"), 0o600)
	require.NoError(t, err)

	// Create temperature input file
	tempFile := filepath.Join(hwmonDir, "temp1_input")
	err = os.WriteFile(tempFile, []byte("45000"), 0o600) // 45°C in millicelsius
	require.NoError(t, err)

	// Create temperature label file
	labelFile := filepath.Join(hwmonDir, "temp1_label")
	err = os.WriteFile(labelFile, []byte("temp1"), 0o600)
	require.NoError(t, err)

	// Create temperature max file
	maxFile := filepath.Join(hwmonDir, "temp1_max")
	err = os.WriteFile(maxFile, []byte("100000"), 0o600) // 100°C in millicelsius
	require.NoError(t, err)

	return tempDir
}
