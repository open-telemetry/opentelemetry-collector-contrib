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

func TestFactory(t *testing.T) {
	factory := NewFactory()

	t.Run("factory_creation", func(t *testing.T) {
		assert.NotNil(t, factory)
		assert.Equal(t, metadata.Type, factory.Type())
	})

	t.Run("default_config", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		assert.NotNil(t, cfg)

		enrichmentCfg, ok := cfg.(*Config)
		require.True(t, ok)

		// Check default values
		assert.Empty(t, enrichmentCfg.DataSources)
		assert.Empty(t, enrichmentCfg.EnrichmentRules)
	})
}

func TestCreateProcessors_EmptyConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := processortest.NewNopSettings(metadata.Type)

	// All processor types should work with empty config
	processors := []struct {
		name   string
		create func() (interface{}, error)
	}{
		{"traces", func() (interface{}, error) {
			return factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())
		}},
		{"metrics", func() (interface{}, error) {
			return factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
		}},
		{"logs", func() (interface{}, error) {
			return factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
		}},
	}

	for _, proc := range processors {
		t.Run(proc.name, func(t *testing.T) {
			processor, err := proc.create()
			assert.NoError(t, err)
			assert.NotNil(t, processor)
		})
	}
}

func TestCreateProcessors_ValidConfig(t *testing.T) {
	factory := NewFactory()

	// Create test data files
	tempDir := t.TempDir()

	// JSON file
	jsonFile := filepath.Join(tempDir, "services.json")
	jsonData := []map[string]interface{}{
		{"name": "test-service", "owner": "test-team", "version": "v1.0.0"},
	}
	data, _ := json.Marshal(jsonData)
	os.WriteFile(jsonFile, data, 0o644)

	// CSV file
	csvFile := filepath.Join(tempDir, "hosts.csv")
	csvData := "hostname,location,region\ntest-host,us-east-1,east"
	os.WriteFile(csvFile, []byte(csvData), 0o644)

	testCases := []struct {
		name   string
		config *Config
	}{
		{
			name: "json_data_source",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "services",
						Type: "file",
						File: &FileDataSourceConfig{
							Path:            jsonFile,
							Format:          "json",
							RefreshInterval: 5 * time.Minute,
						},
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:               "service-enrichment",
						DataSource:         "services",
						LookupAttributeKey: "service.name",
						LookupField:        "name",
						Mappings: []FieldMapping{
							{SourceField: "owner", TargetAttribute: "team.name"},
							{SourceField: "version", TargetAttribute: "service.version"},
						},
					},
				},
			},
		},
		{
			name: "csv_data_source",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "hosts",
						Type: "file",
						File: &FileDataSourceConfig{
							Path:            csvFile,
							Format:          "csv",
							RefreshInterval: 2 * time.Minute,
						},
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:               "host-enrichment",
						DataSource:         "hosts",
						LookupAttributeKey: "host.name",
						LookupField:        "hostname",
						Mappings: []FieldMapping{
							{SourceField: "location", TargetAttribute: "geo.location"},
							{SourceField: "region", TargetAttribute: "geo.region"},
						},
					},
				},
			},
		},
		{
			name: "multiple_data_sources",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "services",
						Type: "file",
						File: &FileDataSourceConfig{
							Path:            jsonFile,
							Format:          "json",
							RefreshInterval: 5 * time.Minute,
						},
					},
					{
						Name: "hosts",
						Type: "file",
						File: &FileDataSourceConfig{
							Path:            csvFile,
							Format:          "csv",
							RefreshInterval: 2 * time.Minute,
						},
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:               "service-enrichment",
						DataSource:         "services",
						LookupAttributeKey: "service.name",
						LookupField:        "name",
						Mappings: []FieldMapping{
							{SourceField: "owner", TargetAttribute: "team.name"},
						},
					},
					{
						Name:               "host-enrichment",
						DataSource:         "hosts",
						LookupAttributeKey: "host.name",
						LookupField:        "hostname",
						Mappings: []FieldMapping{
							{SourceField: "location", TargetAttribute: "geo.location"},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			set := processortest.NewNopSettings(metadata.Type)

			// Test all processor types with the same config
			processors := []struct {
				name   string
				create func() (interface{}, error)
			}{
				{"traces", func() (interface{}, error) {
					return factory.CreateTraces(context.Background(), set, tc.config, consumertest.NewNop())
				}},
				{"metrics", func() (interface{}, error) {
					return factory.CreateMetrics(context.Background(), set, tc.config, consumertest.NewNop())
				}},
				{"logs", func() (interface{}, error) {
					return factory.CreateLogs(context.Background(), set, tc.config, consumertest.NewNop())
				}},
			}

			for _, proc := range processors {
				t.Run(proc.name, func(t *testing.T) {
					processor, err := proc.create()
					assert.NoError(t, err)
					assert.NotNil(t, processor)

					// Ensure shutdown is called to clean up any goroutines
					defer func() {
						if shutdownable, ok := processor.(interface{ Shutdown(context.Context) error }); ok {
							shutdownable.Shutdown(context.Background())
						}
					}()

					// Verify processor implements correct interfaces
					switch proc.name {
					case "traces":
						_, ok := processor.(consumer.Traces)
						assert.True(t, ok, "processor should implement traces consumer interface")
					case "metrics":
						_, ok := processor.(consumer.Metrics)
						assert.True(t, ok, "processor should implement metrics consumer interface")
					case "logs":
						_, ok := processor.(consumer.Logs)
						assert.True(t, ok, "processor should implement logs consumer interface")
					}
				})
			}
		})
	}
}
