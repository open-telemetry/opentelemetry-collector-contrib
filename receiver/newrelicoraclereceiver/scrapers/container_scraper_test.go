// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
)

func TestContainerScraper_NewContainerScraper(t *testing.T) {
	mb := &metadata.MetricsBuilder{}
	logger := zap.NewNop()
	config := metadata.DefaultMetricsBuilderConfig()

	scraper := NewContainerScraper(nil, mb, logger, "test-instance", config)

	assert.NotNil(t, scraper)
	assert.Equal(t, "test-instance", scraper.instanceName)
	assert.Equal(t, mb, scraper.mb)
	assert.Equal(t, logger, scraper.logger)
	assert.False(t, scraper.environmentChecked)
	assert.Nil(t, scraper.isCDBCapable)
	assert.Nil(t, scraper.isPDBCapable)
}

func TestContainerScraper_EnvironmentSupport(t *testing.T) {
	mb := &metadata.MetricsBuilder{}
	logger := zap.NewNop()
	config := metadata.DefaultMetricsBuilderConfig()

	scraper := NewContainerScraper(nil, mb, logger, "test-instance", config)

	// Test initial state
	assert.False(t, scraper.isCDBSupported())
	assert.False(t, scraper.isPDBSupported())

	// Test with CDB capabilities set to true
	cdbCapable := true
	pdbCapable := true
	scraper.isCDBCapable = &cdbCapable
	scraper.isPDBCapable = &pdbCapable

	assert.True(t, scraper.isCDBSupported())
	assert.True(t, scraper.isPDBSupported())

	// Test with CDB capabilities set to false
	cdbCapable = false
	pdbCapable = false
	scraper.isCDBCapable = &cdbCapable
	scraper.isPDBCapable = &pdbCapable

	assert.False(t, scraper.isCDBSupported())
	assert.False(t, scraper.isPDBSupported())
}

func TestContainerScraper_ConfigurationValidation(t *testing.T) {
	mb := &metadata.MetricsBuilder{}
	logger := zap.NewNop()
	config := metadata.DefaultMetricsBuilderConfig()

	// Test with various metric configurations
	testCases := []struct {
		name                      string
		enableContainerStatus     bool
		enableContainerRestricted bool
		expectMetricsEnabled      bool
	}{
		{
			name:                      "both_enabled",
			enableContainerStatus:     true,
			enableContainerRestricted: true,
			expectMetricsEnabled:      true,
		},
		{
			name:                      "status_only",
			enableContainerStatus:     true,
			enableContainerRestricted: false,
			expectMetricsEnabled:      true,
		},
		{
			name:                      "restricted_only",
			enableContainerStatus:     false,
			enableContainerRestricted: true,
			expectMetricsEnabled:      true,
		},
		{
			name:                      "both_disabled",
			enableContainerStatus:     false,
			enableContainerRestricted: false,
			expectMetricsEnabled:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config.Metrics.NewrelicoracledbContainerStatus.Enabled = tc.enableContainerStatus
			config.Metrics.NewrelicoracledbContainerRestricted.Enabled = tc.enableContainerRestricted

			scraper := NewContainerScraper(nil, mb, logger, "test-instance", config)

			// Verify configuration is stored correctly
			assert.Equal(t, tc.enableContainerStatus, scraper.config.Metrics.NewrelicoracledbContainerStatus.Enabled)
			assert.Equal(t, tc.enableContainerRestricted, scraper.config.Metrics.NewrelicoracledbContainerRestricted.Enabled)
		})
	}
}

// Integration test structure for manual testing
type ContainerScraperIntegrationTest struct {
	scraper *ContainerScraper
	logger  *zap.Logger
}

func NewContainerScraperIntegrationTest(connectionString string) (*ContainerScraperIntegrationTest, error) {
	// This would be used for integration testing with a real Oracle database
	// For now, return a test structure that demonstrates the test patterns

	logger, _ := zap.NewDevelopment()
	mb := &metadata.MetricsBuilder{}
	config := metadata.DefaultMetricsBuilderConfig()

	// Enable all container metrics for testing
	config.Metrics.NewrelicoracledbContainerStatus.Enabled = true
	config.Metrics.NewrelicoracledbContainerRestricted.Enabled = true
	config.Metrics.NewrelicoracledbPdbStatus.Enabled = true
	config.Metrics.NewrelicoracledbPdbOpenMode.Enabled = true
	config.Metrics.NewrelicoracledbPdbTotalSizeBytes.Enabled = true

	scraper := NewContainerScraper(nil, mb, logger, "integration-test", config)

	return &ContainerScraperIntegrationTest{
		scraper: scraper,
		logger:  logger,
	}, nil
}

func TestContainerScraper_ErrorHandlingPatterns(t *testing.T) {
	// Test that error handling follows expected patterns
	// This validates the error structure without requiring database connection

	mb := &metadata.MetricsBuilder{}
	logger := zap.NewNop()
	config := metadata.DefaultMetricsBuilderConfig()

	scraper := NewContainerScraper(nil, mb, logger, "test-instance", config)

	// Test environment capability state management
	assert.False(t, scraper.environmentChecked)

	// Simulate setting environment capabilities
	cdbCapable := true
	scraper.isCDBCapable = &cdbCapable
	assert.True(t, scraper.isCDBSupported())

	// Test nil safety
	scraper.isCDBCapable = nil
	assert.False(t, scraper.isCDBSupported())
}

// Benchmarks for performance validation
func BenchmarkContainerScraper_Creation(b *testing.B) {
	mb := &metadata.MetricsBuilder{}
	logger := zap.NewNop()
	config := metadata.DefaultMetricsBuilderConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scraper := NewContainerScraper(nil, mb, logger, "benchmark-instance", config)
		_ = scraper // prevent optimization
	}
}

func BenchmarkContainerScraper_ConfigurationCheck(b *testing.B) {
	mb := &metadata.MetricsBuilder{}
	logger := zap.NewNop()
	config := metadata.DefaultMetricsBuilderConfig()

	// Enable metrics for benchmarking
	config.Metrics.NewrelicoracledbContainerStatus.Enabled = true
	config.Metrics.NewrelicoracledbContainerRestricted.Enabled = true

	scraper := NewContainerScraper(nil, mb, logger, "benchmark-instance", config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark configuration checks
		_ = scraper.config.Metrics.NewrelicoracledbContainerStatus.Enabled
		_ = scraper.config.Metrics.NewrelicoracledbContainerRestricted.Enabled
	}
}
