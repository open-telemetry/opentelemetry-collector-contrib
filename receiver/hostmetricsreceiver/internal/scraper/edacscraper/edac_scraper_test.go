//go:build linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package edacscraper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/edacscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                 string
		config               *Config
		setupMockFS          func() (string, error)
		mockReadFile         func(filename string) ([]byte, error)
		mockGlob             func(pattern string) ([]string, error)
		expectMetrics        bool
		expectError          bool
		expectedMetricsCount int
	}

	testCases := []testCase{
		{
			name:   "Standard with mock EDAC data",
			config: &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			mockGlob: func(pattern string) ([]string, error) {
				if filepath.Base(pattern) == "mc[0-9]*" {
					return []string{"/sys/devices/system/edac/mc/mc0", "/sys/devices/system/edac/mc/mc1"}, nil
				}
				if filepath.Base(pattern) == "csrow[0-9]*" {
					if filepath.Dir(pattern) == "/sys/devices/system/edac/mc/mc0" {
						return []string{"/sys/devices/system/edac/mc/mc0/csrow0", "/sys/devices/system/edac/mc/mc0/csrow1"}, nil
					}
					if filepath.Dir(pattern) == "/sys/devices/system/edac/mc/mc1" {
						return []string{"/sys/devices/system/edac/mc/mc1/csrow0"}, nil
					}
				}
				return []string{}, nil
			},
			mockReadFile: func(filename string) ([]byte, error) {
				switch filepath.Base(filename) {
				case "ce_count":
					return []byte("123"), nil
				case "ue_count":
					return []byte("45"), nil
				case "ce_noinfo_count":
					return []byte("67"), nil
				case "ue_noinfo_count":
					return []byte("89"), nil
				default:
					return nil, errors.New("file not found")
				}
			},
			expectMetrics:        true,
			expectedMetricsCount: 1, // Should have metrics data
		},
		{
			name:   "No EDAC controllers found",
			config: &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			mockGlob: func(pattern string) ([]string, error) {
				return []string{}, nil // No controllers found
			},
			mockReadFile: func(filename string) ([]byte, error) {
				return nil, errors.New("should not be called")
			},
			expectMetrics:        true,
			expectedMetricsCount: 1, // Empty metrics object
		},
		{
			name:   "Glob error",
			config: &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			mockGlob: func(pattern string) ([]string, error) {
				return nil, errors.New("glob failed")
			},
			mockReadFile: func(filename string) ([]byte, error) {
				return nil, errors.New("should not be called")
			},
			expectMetrics: false,
			expectError:   true,
		},
		{
			name:   "Read file error",
			config: &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			mockGlob: func(pattern string) ([]string, error) {
				if filepath.Base(pattern) == "mc[0-9]*" {
					return []string{"/sys/devices/system/edac/mc/mc0"}, nil
				}
				return []string{}, nil
			},
			mockReadFile: func(filename string) ([]byte, error) {
				return nil, errors.New("read failed")
			},
			expectMetrics: false,
			expectError:   true,
		},
		{
			name:   "Invalid controller path",
			config: &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			mockGlob: func(pattern string) ([]string, error) {
				if filepath.Base(pattern) == "mc[0-9]*" {
					return []string{"/invalid/path/without/mc"}, nil
				}
				return []string{}, nil
			},
			mockReadFile: func(filename string) ([]byte, error) {
				return []byte("123"), nil
			},
			expectMetrics: false,
			expectError:   true,
		},
		{
			name:   "Invalid integer in file",
			config: &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			mockGlob: func(pattern string) ([]string, error) {
				if filepath.Base(pattern) == "mc[0-9]*" {
					return []string{"/sys/devices/system/edac/mc/mc0"}, nil
				}
				return []string{}, nil
			},
			mockReadFile: func(filename string) ([]byte, error) {
				return []byte("not_a_number"), nil
			},
			expectMetrics: false,
			expectError:   true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newEDACMetricsScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), test.config)
			if test.mockGlob != nil {
				scraper.glob = test.mockGlob
			}
			if test.mockReadFile != nil {
				scraper.readFromFile = test.mockReadFile
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			metrics, err := scraper.scrape(context.Background())

			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if test.expectMetrics {
				assert.Equal(t, test.expectedMetricsCount, metrics.ResourceMetrics().Len())
			}
		})
	}
}

func TestScrape_RealFS(t *testing.T) {
	// This test runs against the real filesystem, it will be skipped if no EDAC data is available
	if _, err := os.Stat("/sys/devices/system/edac"); os.IsNotExist(err) {
		t.Skip("EDAC subsystem not available on this system")
	}

	config := &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}
	scraper := newEDACMetricsScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), config)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	_, err = scraper.scrape(context.Background())
	// This should not error even if no controllers are found
	assert.NoError(t, err)
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	// Note: Config doesn't have Validate method, but factory creation validates
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	scraper, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, scraper)
}
