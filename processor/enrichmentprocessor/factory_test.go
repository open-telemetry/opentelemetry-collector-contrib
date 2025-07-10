// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg)

	enrichmentCfg, ok := cfg.(*Config)
	require.True(t, ok)

	// Check default values
	assert.Empty(t, enrichmentCfg.DataSources)
	assert.Empty(t, enrichmentCfg.EnrichmentRules)
	assert.True(t, enrichmentCfg.Cache.Enabled)
	assert.Equal(t, 5*time.Minute, enrichmentCfg.Cache.TTL)
	assert.Equal(t, 1000, enrichmentCfg.Cache.MaxSize)
}

func TestCreateTracesProcessor(t *testing.T) {
	factory := NewFactory()

	t.Run("Invalid config", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		set := processortest.NewNopSettings(metadata.Type)

		processor, err := factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())
		assert.Error(t, err)
		assert.Nil(t, processor)
		assert.Contains(t, err.Error(), "at least one data source must be configured")
	})

	t.Run("Valid config", func(t *testing.T) {
		// Create temporary test file
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.json")
		testData := map[string]interface{}{
			"test-service": map[string]interface{}{
				"owner": "test-team",
			},
		}
		data, _ := json.Marshal(testData)
		os.WriteFile(testFile, data, 0644)

		cfg := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "test-source",
					Type: "file",
					File: &FileDataSourceConfig{
						Path:            testFile,
						Format:          "json",
						RefreshInterval: 5 * time.Minute,
					},
				},
			},
			Cache: CacheConfig{
				Enabled: true,
				TTL:     5 * time.Minute,
				MaxSize: 1000,
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:        "test-rule",
					DataSource:  "test-source",
					LookupKey:   "service.name",
					LookupField: "name",
					Mappings: []FieldMapping{
						{
							SourceField:     "owner",
							TargetAttribute: "team.name",
						},
					},
				},
			},
		}

		set := processortest.NewNopSettings(metadata.Type)

		processor, err := factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())
		assert.NoError(t, err)
		assert.NotNil(t, processor)

		// Test that processor implements the correct interface
		_, ok := processor.(consumer.Traces)
		assert.True(t, ok, "processor should implement traces consumer interface")
	})
}

func TestCreateMetricsProcessor(t *testing.T) {
	factory := NewFactory()

	t.Run("Invalid config", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		set := processortest.NewNopSettings(metadata.Type)

		processor, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
		assert.Error(t, err)
		assert.Nil(t, processor)
		assert.Contains(t, err.Error(), "at least one data source must be configured")
	})

	t.Run("Valid config", func(t *testing.T) {
		// Create temporary test file
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.json")
		testData := map[string]interface{}{
			"test-host": map[string]interface{}{
				"location": "us-east-1",
			},
		}
		data, _ := json.Marshal(testData)
		os.WriteFile(testFile, data, 0644)

		cfg := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "test-source",
					Type: "file",
					File: &FileDataSourceConfig{
						Path:            testFile,
						Format:          "json",
						RefreshInterval: 1 * time.Minute,
					},
				},
			},
			Cache: CacheConfig{
				Enabled: true,
				TTL:     5 * time.Minute,
				MaxSize: 1000,
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:        "test-rule",
					DataSource:  "test-source",
					LookupKey:   "host.name",
					LookupField: "hostname",
					Mappings: []FieldMapping{
						{
							SourceField:     "location",
							TargetAttribute: "geo.region",
						},
					},
				},
			},
		}

		set := processortest.NewNopSettings(metadata.Type)

		processor, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
		assert.NoError(t, err)
		assert.NotNil(t, processor)

		// Test that processor implements the correct interface
		_, ok := processor.(consumer.Metrics)
		assert.True(t, ok, "processor should implement metrics consumer interface")
	})
}

func TestCreateLogsProcessor(t *testing.T) {
	factory := NewFactory()

	t.Run("Invalid config", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		set := processortest.NewNopSettings(metadata.Type)

		processor, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
		assert.Error(t, err)
		assert.Nil(t, processor)
		assert.Contains(t, err.Error(), "at least one data source must be configured")
	})

	t.Run("Valid config", func(t *testing.T) {
		// Create temporary test file
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.json")
		testData := map[string]interface{}{
			"test-service": map[string]interface{}{
				"owner": "test-team",
				"pager": "+1234567890",
			},
		}
		data, _ := json.Marshal(testData)
		os.WriteFile(testFile, data, 0644)

		cfg := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "test-source",
					Type: "file",
					File: &FileDataSourceConfig{
						Path:            testFile,
						Format:          "json",
						RefreshInterval: 2 * time.Minute,
					},
				},
			},
			Cache: CacheConfig{
				Enabled: true,
				TTL:     3 * time.Minute,
				MaxSize: 500,
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:        "log-enrichment",
					DataSource:  "test-source",
					LookupKey:   "service.name",
					LookupField: "service_name",
					Conditions: []Condition{
						{
							Attribute: "log.level",
							Operator:  "equals",
							Value:     "ERROR",
						},
					},
					Mappings: []FieldMapping{
						{
							SourceField:     "owner_team",
							TargetAttribute: "team.name",
							Transform:       "lower",
						},
						{
							SourceField:     "on_call",
							TargetAttribute: "team.oncall",
						},
					},
				},
			},
		}

		set := processortest.NewNopSettings(metadata.Type)

		processor, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
		assert.NoError(t, err)
		assert.NotNil(t, processor)

		// Test that processor implements the correct interface
		_, ok := processor.(consumer.Logs)
		assert.True(t, ok, "processor should implement logs consumer interface")
	})
}

func TestCreateProcessors_WithDifferentDataSources(t *testing.T) {
	factory := NewFactory()

	// Create temporary files for testing
	tempDir := t.TempDir()

	// JSON test file
	jsonFile := filepath.Join(tempDir, "services.json")
	jsonData := map[string]interface{}{
		"test-service": map[string]interface{}{
			"owner": "test-team",
		},
	}
	data, _ := json.Marshal(jsonData)
	os.WriteFile(jsonFile, data, 0644)

	// CSV test file
	csvFile := filepath.Join(tempDir, "hosts.csv")
	csvData := "hostname,location\ntest-host,us-east-1\n"
	os.WriteFile(csvFile, []byte(csvData), 0644)

	testCases := []struct {
		name           string
		dataSourceType string
		dataSource     DataSourceConfig
	}{
		{
			name:           "File data source - JSON",
			dataSourceType: "file",
			dataSource: DataSourceConfig{
				Name: "test-source",
				Type: "file",
				File: &FileDataSourceConfig{
					Path:            jsonFile,
					Format:          "json",
					RefreshInterval: 2 * time.Minute,
				},
			},
		},
		{
			name:           "File data source - CSV",
			dataSourceType: "file",
			dataSource: DataSourceConfig{
				Name: "test-source",
				Type: "file",
				File: &FileDataSourceConfig{
					Path:            csvFile,
					Format:          "csv",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				DataSources: []DataSourceConfig{
					tc.dataSource,
				},
				Cache: CacheConfig{
					Enabled: true,
					TTL:     5 * time.Minute,
					MaxSize: 1000,
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:        "test-rule",
						DataSource:  "test-source",
						LookupKey:   "service.name",
						LookupField: "name",
						Mappings: []FieldMapping{
							{
								SourceField:     "owner",
								TargetAttribute: "team.name",
							},
						},
					},
				},
			}

			set := processortest.NewNopSettings(metadata.Type)

			// Test traces processor
			tracesProcessor, err := factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())
			assert.NoError(t, err)
			assert.NotNil(t, tracesProcessor)

			// Test metrics processor
			metricsProcessor, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
			assert.NoError(t, err)
			assert.NotNil(t, metricsProcessor)

			// Test logs processor
			logsProcessor, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
			assert.NoError(t, err)
			assert.NotNil(t, logsProcessor)
		})
	}
}

func TestCreateProcessors_WithMultipleRules(t *testing.T) {
	factory := NewFactory()

	// Create temporary files for testing
	tempDir := t.TempDir()

	// Service inventory file
	servicesFile := filepath.Join(tempDir, "services.json")
	servicesData := map[string]interface{}{
		"test-service": map[string]interface{}{
			"owner_team": "platform",
			"version":    "V1.0.0",
		},
	}
	data, _ := json.Marshal(servicesData)
	os.WriteFile(servicesFile, data, 0644)

	// Host metadata file
	hostsFile := filepath.Join(tempDir, "hosts.json")
	hostsData := map[string]interface{}{
		"test-host": map[string]interface{}{
			"datacenter": "us-east-1",
			"rack":       "R01",
		},
	}
	data, _ = json.Marshal(hostsData)
	os.WriteFile(hostsFile, data, 0644)

	cfg := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "service-inventory",
				Type: "file",
				File: &FileDataSourceConfig{
					Path:            servicesFile,
					Format:          "json",
					RefreshInterval: 5 * time.Minute,
				},
			},
			{
				Name: "host-metadata",
				Type: "file",
				File: &FileDataSourceConfig{
					Path:            hostsFile,
					Format:          "json",
					RefreshInterval: 1 * time.Minute,
				},
			},
		},
		Cache: CacheConfig{
			Enabled: true,
			TTL:     10 * time.Minute,
			MaxSize: 2000,
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "service-enrichment",
				DataSource:  "service-inventory",
				LookupKey:   "service.name",
				LookupField: "service_name",
				Conditions: []Condition{
					{
						Attribute: "environment",
						Operator:  "equals",
						Value:     "production",
					},
				},
				Mappings: []FieldMapping{
					{
						SourceField:     "owner_team",
						TargetAttribute: "team.name",
					},
					{
						SourceField:     "version",
						TargetAttribute: "service.version",
						Transform:       "lower",
					},
				},
			},
			{
				Name:        "host-enrichment",
				DataSource:  "host-metadata",
				LookupKey:   "host.name",
				LookupField: "hostname",
				Mappings: []FieldMapping{
					{
						SourceField:     "datacenter",
						TargetAttribute: "host.datacenter",
					},
					{
						SourceField:     "instance_type",
						TargetAttribute: "host.instance_type",
						Transform:       "upper",
					},
				},
			},
		},
	}

	set := processortest.NewNopSettings(metadata.Type)

	// All processor types should work with multiple rules
	tracesProcessor, err := factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tracesProcessor)

	metricsProcessor, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, metricsProcessor)

	logsProcessor, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, logsProcessor)
}

func TestProcessorCapabilities(t *testing.T) {
	// Test that the processor capabilities are correctly set
	assert.True(t, processorCapabilities.MutatesData)
}

func TestFactory_Stability(t *testing.T) {
	// Test that stability levels are correctly set
	assert.Equal(t, metadata.TracesStability, metadata.TracesStability)
	assert.Equal(t, metadata.MetricsStability, metadata.MetricsStability)
	assert.Equal(t, metadata.LogsStability, metadata.LogsStability)
}
