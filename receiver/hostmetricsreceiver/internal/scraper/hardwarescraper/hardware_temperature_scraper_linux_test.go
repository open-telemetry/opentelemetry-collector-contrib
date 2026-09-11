// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hardwarescraper

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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

func TestHardwareTemperatureScraperStart_Linux(t *testing.T) {
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
					Sensors: []string{".*"},
				},
			},
		},
		{
			name:      "Invalid hwmon path",
			hwmonPath: "/nonexistent/path",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{".*"},
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
					Sensors: []string{".*"},
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
			scraper := &hardwareTemperatureScraper{
				logger:               zap.NewNop(),
				config:               test.config,
				hwmonPath:            test.hwmonPath,
				metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
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

func TestHardwareTemperatureScraperScrape_Linux(t *testing.T) {
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
	disabledTempMetric := metadata.NewDefaultMetricsBuilderConfig()
	disabledTempMetric.Metrics.HwTemperature.Enabled = false

	// Disabled temperature limit metric
	disabledLimitMetric := metadata.NewDefaultMetricsBuilderConfig()
	disabledLimitMetric.Metrics.HwTemperatureLimit.Enabled = false

	// All metrics enabled
	allEnabledMetrics := metadata.NewDefaultMetricsBuilderConfig()
	allEnabledMetrics.Metrics.HwTemperatureLimit.Enabled = true

	testCases := []testCase{
		{
			name: "Standard with default metrics",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{".*"},
				},
			},
			metricsConfig:       metadata.NewDefaultMetricsBuilderConfig(),
			expectedMetricCount: 1, // hw.temperature only (limit disabled by default)
			setupCompleteDir:    true,
		},
		{
			name: "All metrics enabled",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{".*"},
				},
			},
			metricsConfig:       allEnabledMetrics,
			expectedMetricCount: 2, // hw.temperature, hw.temperature.limit
			setupCompleteDir:    true,
		},
		{
			name: "Temperature metric disabled",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{".*"},
				},
			},
			metricsConfig:       disabledTempMetric,
			expectedMetricCount: 0, // no metrics (both disabled)
			setupCompleteDir:    true,
		},
		{
			name: "Temperature limit metric disabled",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{".*"},
				},
			},
			metricsConfig:       disabledLimitMetric,
			expectedMetricCount: 1, // hw.temperature only (limit disabled)
			setupCompleteDir:    true,
		},
		{
			name: "Exclude specific sensor",
			config: &TemperatureConfig{
				Include: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Regexp},
					Sensors: []string{".*"},
				},
				Exclude: MatchConfig{
					Config:  filterset.Config{MatchType: filterset.Strict},
					Sensors: []string{"temp1"},
				},
			},
			metricsConfig:       metadata.NewDefaultMetricsBuilderConfig(),
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

			scraper := &hardwareTemperatureScraper{
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

	scraper := &hardwareTemperatureScraper{}
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

func TestDeviceKey(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	t.Run("resolves device symlink to a stable key", func(t *testing.T) {
		tempDir := t.TempDir()
		hwmonDir := filepath.Join(tempDir, "hwmon0")
		require.NoError(t, os.MkdirAll(hwmonDir, 0o755))

		target := filepath.Join(tempDir, "devices", "0000:00:18.3")
		require.NoError(t, os.MkdirAll(target, 0o755))
		require.NoError(t, os.Symlink(target, filepath.Join(hwmonDir, "device")))

		assert.Equal(t, "0000:00:18.3", deviceKey(hwmonDir))
	})

	t.Run("resolves through intermediate symlinks", func(t *testing.T) {
		// Real sysfs "device" links chain through symlinks, so the path is fully resolved.
		tempDir := t.TempDir()
		hwmonDir := filepath.Join(tempDir, "hwmon0")
		require.NoError(t, os.MkdirAll(hwmonDir, 0o755))

		target := filepath.Join(tempDir, "devices", "0000:00:1c.4")
		require.NoError(t, os.MkdirAll(target, 0o755))
		intermediate := filepath.Join(tempDir, "intermediate")
		require.NoError(t, os.Symlink(target, intermediate))
		require.NoError(t, os.Symlink(intermediate, filepath.Join(hwmonDir, "device")))

		assert.Equal(t, "0000:00:1c.4", deviceKey(hwmonDir))
	})

	t.Run("falls back on broken device symlink", func(t *testing.T) {
		tempDir := t.TempDir()
		hwmonDir := filepath.Join(tempDir, "hwmon0")
		require.NoError(t, os.MkdirAll(hwmonDir, 0o755))

		require.NoError(t, os.Symlink(filepath.Join(tempDir, "missing"), filepath.Join(hwmonDir, "device")))

		assert.Equal(t, "hwmon0", deviceKey(hwmonDir))
	})

	t.Run("falls back to hwmon directory name without device link", func(t *testing.T) {
		tempDir := t.TempDir()
		hwmonDir := filepath.Join(tempDir, "hwmon0")
		require.NoError(t, os.MkdirAll(hwmonDir, 0o755))

		assert.Equal(t, "hwmon0", deviceKey(hwmonDir))
	})
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
