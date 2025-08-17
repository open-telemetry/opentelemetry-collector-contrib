// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hwscraper

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scrapertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

func TestHwTemperatureScraperStart_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	type testCase struct {
		name        string
		hwmonPath   string
		config      *TemperatureConfig
		expectedErr string
		setupDir    bool
	}

	testCases := []testCase{
		{
			name:      "Standard",
			setupDir:  true,
			hwmonPath: createTestHwmonDir(t),
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
			},
		},
		{
			name:        "Invalid hwmon path",
			hwmonPath:   "/nonexistent/path",
			expectedErr: "hwmon not available",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
			},
		},
		{
			name:        "Invalid include filter",
			setupDir:    true,
			hwmonPath:   createTestHwmonDir(t),
			expectedErr: "failed to create include filter",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"["}, // Invalid regex
				},
			},
		},
		{
			name:        "Invalid exclude filter",
			setupDir:    true,
			hwmonPath:   createTestHwmonDir(t),
			expectedErr: "failed to create exclude filter",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
				Exclude: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"["}, // Invalid regex
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := &hwTemperatureScraper{
				logger:               zap.NewNop(),
				config:               test.config,
				hwmonPath:            test.hwmonPath,
				metricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			}

			err := scraper.start(t.Context())
			if test.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHwTemperatureScraperScrape_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	type testCase struct {
		name                string
		config              *TemperatureConfig
		metricsConfig       metadata.MetricsBuilderConfig
		expectedMetricCount int
		setupCompleteDir    bool
	}

	// Disabled temperature metric
	disabledTempMetric := metadata.DefaultMetricsBuilderConfig()
	disabledTempMetric.Metrics.HwTemperature.Enabled = false

	// Disabled temperature limit metric
	disabledLimitMetric := metadata.DefaultMetricsBuilderConfig()
	disabledLimitMetric.Metrics.HwTemperatureLimit.Enabled = false

	// Disabled status metric
	disabledStatusMetric := metadata.DefaultMetricsBuilderConfig()
	disabledStatusMetric.Metrics.HwStatus.Enabled = false

	// All metrics enabled
	allEnabledMetrics := metadata.DefaultMetricsBuilderConfig()
	allEnabledMetrics.Metrics.HwTemperatureLimit.Enabled = true

	testCases := []testCase{
		{
			name: "Standard with default metrics",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
			},
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			expectedMetricCount: 2, // hw.temperature and hw.status (default enabled)
			setupCompleteDir:    true,
		},
		{
			name: "All metrics enabled",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
			},
			metricsConfig:       allEnabledMetrics,
			expectedMetricCount: 3, // hw.temperature, hw.temperature.limit, hw.status
			setupCompleteDir:    true,
		},
		{
			name: "Temperature metric disabled",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
			},
			metricsConfig:       disabledTempMetric,
			expectedMetricCount: 1, // hw.status only
			setupCompleteDir:    true,
		},
		{
			name: "Temperature limit metric disabled",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
			},
			metricsConfig:       disabledLimitMetric,
			expectedMetricCount: 2, // hw.temperature and hw.status (limit already disabled by default)
			setupCompleteDir:    true,
		},
		{
			name: "Status metric disabled",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
			},
			metricsConfig:       disabledStatusMetric,
			expectedMetricCount: 1, // hw.temperature only
			setupCompleteDir:    true,
		},
		{
			name: "Exclude specific sensor",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{"temp*"},
				},
				Exclude: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Strict},
					Sensors: []string{"temp1"},
				},
			},
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			expectedMetricCount: 0, // No sensors match after exclusion
			setupCompleteDir:    true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			hwmonPath := createTestHwmonDir(t)
			if test.setupCompleteDir {
				setupCompleteTestData(t, hwmonPath)
			}

			scraper := &hwTemperatureScraper{
				logger:               zap.NewNop(),
				config:               test.config,
				hwmonPath:            hwmonPath,
				metricsBuilderConfig: test.metricsConfig,
			}

			err := scraper.start(t.Context())
			require.NoError(t, err)

			mb := metadata.NewMetricsBuilder(test.metricsConfig, scrapertest.NewNopSettings(metadata.Type))
			err = scraper.scrape(t.Context(), mb)
			assert.NoError(t, err)

			metrics := mb.Emit()
			assert.Equal(t, test.expectedMetricCount, metrics.MetricCount())
		})
	}
}

func TestReadTemperatureCelsius(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "temp_input")

	// Test valid temperature reading
	err := os.WriteFile(tempFile, []byte("45000"), 0o600) // 45°C in millicelsius
	require.NoError(t, err)

	scraper := &hwTemperatureScraper{}
	temp, err := scraper.readTemperatureCelsius(tempFile)
	assert.NoError(t, err)
	assert.Equal(t, 45.0, temp)

	// Test invalid file
	_, err = scraper.readTemperatureCelsius("/nonexistent/file")
	assert.Error(t, err)

	// Test invalid content
	err = os.WriteFile(tempFile, []byte("invalid"), 0o600)
	require.NoError(t, err)
	_, err = scraper.readTemperatureCelsius(tempFile)
	assert.Error(t, err)
}

func TestDetermineTemperatureState(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	scraper := &hwTemperatureScraper{
		logger: zap.NewNop(),
	}

	type testCase struct {
		name        string
		temperature float64
		limits      temperatureLimits
		expected    metadata.AttributeState
	}

	critTemp := 85.0
	maxTemp := 75.0
	minTemp := 5.0
	lowCritTemp := 0.0

	testCases := []testCase{
		{
			name:        "Normal temperature",
			temperature: 50.0,
			limits:      temperatureLimits{},
			expected:    metadata.AttributeStateOk,
		},
		{
			name:        "Critical high temperature with limit",
			temperature: 90.0,
			limits:      temperatureLimits{critTemp: &critTemp},
			expected:    metadata.AttributeStatePredictedFailure, // 90 >= 85 critTemp
		},
		{
			name:        "Degraded high temperature with limit",
			temperature: 76.0,
			limits:      temperatureLimits{maxTemp: &maxTemp},
			expected:    metadata.AttributeStateDegraded, // 76 >= 75 maxTemp
		},
		{
			name:        "Critical low temperature with limit",
			temperature: -5.0,
			limits:      temperatureLimits{lowCritTemp: &lowCritTemp},
			expected:    metadata.AttributeStatePredictedFailure, // -5 <= 0 lowCritTemp
		},
		{
			name:        "Degraded low temperature with limit",
			temperature: 4.0,
			limits:      temperatureLimits{minTemp: &minTemp},
			expected:    metadata.AttributeStateDegraded, // 4 <= 5 minTemp
		},
		{
			name:        "Needs cleaning (high temp over 75)",
			temperature: 76.0,
			limits:      temperatureLimits{},
			expected:    metadata.AttributeStateNeedsCleaning, // 76 > 75.0
		},
		{
			name:        "Predicted failure (high temp over 85)",
			temperature: 86.0,
			limits:      temperatureLimits{},
			expected:    metadata.AttributeStatePredictedFailure, // 86 > 85.0
		},
		{
			name:        "Degraded high temperature with default threshold",
			temperature: 81.0,
			limits:      temperatureLimits{},
			expected:    metadata.AttributeStateDegraded, // 81 >= 80.0 default
		},
		{
			name:        "Degraded low temperature with default threshold",
			temperature: 4.0,
			limits:      temperatureLimits{},
			expected:    metadata.AttributeStateDegraded, // 4 <= 5.0 default
		},
		{
			name:        "Predicted failure low temperature with default threshold",
			temperature: -1.0,
			limits:      temperatureLimits{},
			expected:    metadata.AttributeStatePredictedFailure, // -1 < 0.0 default
		},
		{
			name:        "Unreasonable temperature",
			temperature: 250.0,
			limits:      temperatureLimits{},
			expected:    metadata.AttributeStateFailed, // 250 > 200 maxReasonableTemp
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			state := scraper.determineTemperatureState(test.temperature, test.limits)
			assert.Equal(t, test.expected, state)
		})
	}
}

// createTestHwmonDir creates a basic hwmon directory structure
func createTestHwmonDir(t *testing.T) string {
	tempDir := t.TempDir()
	hwmonDir := filepath.Join(tempDir, "hwmon0")

	err := os.MkdirAll(hwmonDir, 0o755)
	require.NoError(t, err)

	// Create device name file
	nameFile := filepath.Join(hwmonDir, "name")
	err = os.WriteFile(nameFile, []byte("test_device"), 0o600)
	require.NoError(t, err)

	return tempDir
}

// setupCompleteTestData creates complete test data with temperature files
func setupCompleteTestData(t *testing.T, hwmonBasePath string) {
	hwmonDir := filepath.Join(hwmonBasePath, "hwmon0")

	// Create temperature input file
	tempFile := filepath.Join(hwmonDir, "temp1_input")
	err := os.WriteFile(tempFile, []byte("45000"), 0o600) // 45°C
	require.NoError(t, err)

	// Create temperature label file
	labelFile := filepath.Join(hwmonDir, "temp1_label")
	err = os.WriteFile(labelFile, []byte("temp1"), 0o600)
	require.NoError(t, err)

	// Create temperature max file
	maxFile := filepath.Join(hwmonDir, "temp1_max")
	err = os.WriteFile(maxFile, []byte("80000"), 0o600) // 80°C
	require.NoError(t, err)

	// Create temperature critical file
	critFile := filepath.Join(hwmonDir, "temp1_crit")
	err = os.WriteFile(critFile, []byte("90000"), 0o600) // 90°C
	require.NoError(t, err)
}
