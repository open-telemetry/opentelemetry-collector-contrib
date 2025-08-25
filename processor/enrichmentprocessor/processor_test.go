// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// mockDataSource for testing
type mockDataSource struct {
	data        [][]string
	headerIndex map[string]int
	lookup      *lookup
}

func newMockDataSource(data [][]string, headerIndex map[string]int, indexFields []string) *mockDataSource {
	lookup := newLookup()
	lookup.SetAll(data, headerIndex, indexFields)

	return &mockDataSource{
		data:        data,
		headerIndex: headerIndex,
		lookup:      lookup,
	}
}

func (m *mockDataSource) Lookup(lookupField, key string) (enrichmentRow []string, index map[string]int, err error) {
	return m.lookup.Lookup(lookupField, key)
}

func (*mockDataSource) Start(context.Context) error {
	return nil
}

func (*mockDataSource) Stop() error {
	return nil
}

func TestEnrichmentProcessor_Core(t *testing.T) {
	// Create mock data in the new format
	data := [][]string{
		{"test-service", "platform-team", "production", "1.2.3"},
		{"payment-service", "payments-team", "staging", "2.1.0"},
	}
	headerIndex := map[string]int{
		"name":        0,
		"owner":       1,
		"environment": 2,
		"version":     3,
	}
	indexFields := []string{"name", "environment"}

	mockDataSource := newMockDataSource(data, headerIndex, indexFields)

	// Create processor with mock data source
	processor := &enrichmentProcessor{
		config: &Config{
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "test_rule",
					DataSource:         "mock",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            ENRICHCONTEXTINDIVIDUAL,
					Mappings: []FieldMapping{
						{
							SourceField:     "owner",
							TargetAttribute: "team.name",
						},
						{
							SourceField:     "environment",
							TargetAttribute: "deployment.environment",
						},
					},
				},
			},
		},
		dataSources: map[string]DataSource{
			"mock": mockDataSource,
		},
		logger: zap.NewNop(),
	}

	t.Run("successful_enrichment", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "test-service")

		err := processor.applyEnrichmentRule(processor.config.EnrichmentRules[0], attributes)
		assert.NoError(t, err)

		// Check that attributes were added
		teamName, exists := attributes.Get("team.name")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "platform-team", teamName.AsString())
		}

		environment, exists := attributes.Get("deployment.environment")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "production", environment.AsString())
		}
	})

	t.Run("enrichment_not_found", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "non-existent-service")

		err := processor.applyEnrichmentRule(processor.config.EnrichmentRules[0], attributes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "enrichment data not found for field 'name' with value 'non-existent-service'")
	})

	t.Run("lookup_by_different_field", func(t *testing.T) {
		// Create a rule that uses environment as lookup field
		environmentRule := EnrichmentRule{
			Name:               "env_rule",
			DataSource:         "mock",
			LookupAttributeKey: "deployment.environment",
			LookupField:        "environment",
			Context:            ENRICHCONTEXTRESOURCE,
			Mappings: []FieldMapping{
				{
					SourceField:     "name",
					TargetAttribute: "service.name",
				},
				{
					SourceField:     "owner",
					TargetAttribute: "team.name",
				},
			},
		}

		attributes := pcommon.NewMap()
		attributes.PutStr("deployment.environment", "staging")

		err := processor.applyEnrichmentRule(environmentRule, attributes)
		assert.NoError(t, err)

		// Check that attributes were added based on environment lookup
		serviceName, exists := attributes.Get("service.name")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "payment-service", serviceName.AsString())
		}

		teamName, exists := attributes.Get("team.name")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "payments-team", teamName.AsString())
		}
	})
}

func TestApplyMappings(t *testing.T) {
	processor := &enrichmentProcessor{
		logger: zap.NewNop(),
	}

	enrichmentRow := []string{"test-service", "platform-team", "production", "1.2.3"}
	headerIndex := map[string]int{
		"name":        0,
		"owner":       1,
		"environment": 2,
		"version":     3,
	}

	testCases := []struct {
		name            string
		mappings        []FieldMapping
		expectedAttrs   map[string]string
		unexpectedAttrs []string
	}{
		{
			name: "valid_mappings",
			mappings: []FieldMapping{
				{SourceField: "owner", TargetAttribute: "team.name"},
				{SourceField: "environment", TargetAttribute: "deployment.environment"},
			},
			expectedAttrs: map[string]string{
				"team.name":              "platform-team",
				"deployment.environment": "production",
			},
		},
		{
			name: "invalid_source_field",
			mappings: []FieldMapping{
				{SourceField: "owner", TargetAttribute: "team.name"},
				{SourceField: "nonexistent", TargetAttribute: "should.not.exist"},
			},
			expectedAttrs: map[string]string{
				"team.name": "platform-team",
			},
			unexpectedAttrs: []string{"should.not.exist"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attributes := pcommon.NewMap()
			processor.applyMappings(tc.mappings, enrichmentRow, headerIndex, attributes)

			// Check expected attributes
			for attrName, expectedValue := range tc.expectedAttrs {
				value, exists := attributes.Get(attrName)
				assert.True(t, exists, "Expected attribute %s should exist", attrName)
				if exists {
					assert.Equal(t, expectedValue, value.AsString())
				}
			}

			// Check unexpected attributes
			for _, attrName := range tc.unexpectedAttrs {
				_, exists := attributes.Get(attrName)
				assert.False(t, exists, "Unexpected attribute %s should not exist", attrName)
			}
		})
	}
}

func TestEnrichmentContextFiltering(t *testing.T) {
	data := [][]string{
		{"test-service", "platform-team", "production", "1.2.3"},
	}
	headerIndex := map[string]int{
		"name":        0,
		"owner":       1,
		"environment": 2,
		"version":     3,
	}
	indexFields := []string{"name"}

	mockDataSource := newMockDataSource(data, headerIndex, indexFields)

	processor := &enrichmentProcessor{
		config: &Config{
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "resource_rule",
					DataSource:         "mock",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            ENRICHCONTEXTRESOURCE,
					Mappings: []FieldMapping{
						{SourceField: "owner", TargetAttribute: "team.name"},
					},
				},
				{
					Name:               "individual_rule",
					DataSource:         "mock",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            ENRICHCONTEXTINDIVIDUAL,
					Mappings: []FieldMapping{
						{SourceField: "version", TargetAttribute: "service.version"},
					},
				},
			},
		},
		dataSources: map[string]DataSource{
			"mock": mockDataSource,
		},
		logger: zap.NewNop(),
	}

	t.Run("resource_context_filtering", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "test-service")

		processor.enrichAttributes(attributes, ENRICHCONTEXTRESOURCE)

		// Should have team.name from resource_rule
		teamName, exists := attributes.Get("team.name")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "platform-team", teamName.AsString())
		}

		// Should not have service.version from individual_rule
		_, exists = attributes.Get("service.version")
		assert.False(t, exists)
	})

	t.Run("individual_context_filtering", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "test-service")

		processor.enrichAttributes(attributes, ENRICHCONTEXTINDIVIDUAL)

		// Should have service.version from individual_rule
		version, exists := attributes.Get("service.version")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "1.2.3", version.AsString())
		}

		// Should not have team.name from resource_rule
		_, exists = attributes.Get("team.name")
		assert.False(t, exists)
	})
}

func TestEnrichmentWithConditions(t *testing.T) {
	// Create mock data
	data := [][]string{
		{"test-service", "platform-team", "production", "1.2.3"},
		{"payment-service", "payments-team", "staging", "2.1.0"},
	}
	headerIndex := map[string]int{
		"name":        0,
		"owner":       1,
		"environment": 2,
		"version":     3,
	}
	indexFields := []string{"name"}

	mockDataSource := newMockDataSource(data, headerIndex, indexFields)

	// Create processor with conditional rule (conditions are ignored)
	processor := &enrichmentProcessor{
		config: &Config{
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "conditional_rule",
					DataSource:         "mock",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            ENRICHCONTEXTINDIVIDUAL,
					Mappings: []FieldMapping{
						{
							SourceField:     "owner",
							TargetAttribute: "team.name",
						},
					},
				},
			},
		},
		dataSources: map[string]DataSource{
			"mock": mockDataSource,
		},
		logger: zap.NewNop(),
	}

	// Test case 1: previously matching condition — now enrichment always applies
	attributes := pcommon.NewMap()
	attributes.PutStr("service.name", "test-service")
	attributes.PutStr("environment", "production")

	processor.enrichAttributes(attributes, ENRICHCONTEXTINDIVIDUAL)

	teamName, exists := attributes.Get("team.name")
	assert.True(t, exists)
	if exists {
		assert.Equal(t, "platform-team", teamName.AsString())
	}

	// Test case 2: previously non-matching condition — enrichment still applies
	attributes2 := pcommon.NewMap()
	attributes2.PutStr("service.name", "test-service")
	attributes2.PutStr("environment", "staging")

	processor.enrichAttributes(attributes2, ENRICHCONTEXTINDIVIDUAL)

	teamName2, exists := attributes2.Get("team.name")
	assert.True(t, exists)
	if exists {
		assert.Equal(t, "platform-team", teamName2.AsString())
	}
}

func TestMetricsProcessorEnrichment(t *testing.T) {
	// Create mock data
	data := [][]string{
		{"test-service", "platform-team", "production", "1.2.3"},
	}
	headerIndex := map[string]int{
		"name":        0,
		"owner":       1,
		"environment": 2,
		"version":     3,
	}
	indexFields := []string{"name"}

	mockDataSource := newMockDataSource(data, headerIndex, indexFields)

	enrichmentProc := &enrichmentProcessor{
		config: &Config{
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "resource_rule",
					DataSource:         "mock",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            ENRICHCONTEXTRESOURCE,
					Mappings: []FieldMapping{
						{SourceField: "owner", TargetAttribute: "team.name"},
					},
				},
				{
					Name:               "individual_rule",
					DataSource:         "mock",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            ENRICHCONTEXTINDIVIDUAL,
					Mappings: []FieldMapping{
						{SourceField: "version", TargetAttribute: "service.version"},
					},
				},
			},
		},
		dataSources: map[string]DataSource{
			"mock": mockDataSource,
		},
		logger: zap.NewNop(),
	}

	processor := &metricsProcessor{
		enrichmentProcessor: enrichmentProc,
	}

	t.Run("enrich_gauge_metrics", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("test_gauge")
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("service.name", "test-service")
		dp.SetDoubleValue(42.0)

		processor.enrichMetricDataPoints(metric)

		// Check individual enrichment on data point
		dpAttrs := gauge.DataPoints().At(0).Attributes()
		version, exists := dpAttrs.Get("service.version")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "1.2.3", version.AsString())
		}
	})

	t.Run("enrich_sum_metrics", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("test_sum")
		sum := metric.SetEmptySum()
		dp := sum.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("service.name", "test-service")
		dp.SetDoubleValue(100.0)

		processor.enrichMetricDataPoints(metric)

		// Check individual enrichment on data point
		dpAttrs := sum.DataPoints().At(0).Attributes()
		version, exists := dpAttrs.Get("service.version")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "1.2.3", version.AsString())
		}
	})

	t.Run("enrich_histogram_metrics", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("test_histogram")
		histogram := metric.SetEmptyHistogram()
		dp := histogram.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("service.name", "test-service")
		dp.SetCount(10)

		processor.enrichMetricDataPoints(metric)

		// Check individual enrichment on data point
		dpAttrs := histogram.DataPoints().At(0).Attributes()
		version, exists := dpAttrs.Get("service.version")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "1.2.3", version.AsString())
		}
	})

	t.Run("enrich_summary_metrics", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("test_summary")
		summary := metric.SetEmptySummary()
		dp := summary.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("service.name", "test-service")
		dp.SetCount(5)

		processor.enrichMetricDataPoints(metric)

		// Check individual enrichment on data point
		dpAttrs := summary.DataPoints().At(0).Attributes()
		version, exists := dpAttrs.Get("service.version")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "1.2.3", version.AsString())
		}
	})
}

func TestDataSourceCreation(t *testing.T) {
	processor := &enrichmentProcessor{
		config: &Config{
			EnrichmentRules: []EnrichmentRule{
				{
					Name:        "test_rule",
					DataSource:  "http_source",
					LookupField: "name",
				},
			},
		},
		logger: zap.NewNop(),
	}

	t.Run("create_http_data_source", func(t *testing.T) {
		config := DataSourceConfig{
			Name: "http_source",
			Type: "http",
			HTTP: &HTTPDataSourceConfig{
				URL: "http://example.com/data.json",
			},
		}

		dataSource, err := processor.createDataSource(config)
		assert.NoError(t, err)
		assert.NotNil(t, dataSource)
	})

	t.Run("create_file_data_source", func(t *testing.T) {
		config := DataSourceConfig{
			Name: "file_source",
			Type: "file",
			File: &FileDataSourceConfig{
				Path:   "/tmp/test.csv",
				Format: "csv",
			},
		}

		dataSource, err := processor.createDataSource(config)
		assert.NoError(t, err)
		assert.NotNil(t, dataSource)
	})

	t.Run("unsupported_data_source_type", func(t *testing.T) {
		config := DataSourceConfig{
			Name: "unknown_source",
			Type: "unknown",
		}

		dataSource, err := processor.createDataSource(config)
		assert.Error(t, err)
		assert.Nil(t, dataSource)
		assert.Contains(t, err.Error(), "unsupported data source type: unknown")
	})

	t.Run("missing_http_config", func(t *testing.T) {
		config := DataSourceConfig{
			Name: "http_source",
			Type: "http",
			HTTP: nil,
		}

		dataSource, err := processor.createDataSource(config)
		assert.Error(t, err)
		assert.Nil(t, dataSource)
		assert.Contains(t, err.Error(), "HTTP configuration is required")
	})

	t.Run("missing_file_config", func(t *testing.T) {
		config := DataSourceConfig{
			Name: "file_source",
			Type: "file",
			File: nil,
		}

		dataSource, err := processor.createDataSource(config)
		assert.Error(t, err)
		assert.Nil(t, dataSource)
		assert.Contains(t, err.Error(), "File configuration is required")
	})
}

func TestProcessorShutdown(t *testing.T) {
	// Create a mock data source that can simulate error on Stop
	type ErrorDataSource struct {
		*mockDataSource
		shouldError bool
	}

	errorDS := &ErrorDataSource{
		mockDataSource: newMockDataSource([][]string{}, map[string]int{}, []string{}),
		shouldError:    true,
	}

	processor := &enrichmentProcessor{
		config: &Config{},
		logger: zap.NewNop(),
		dataSources: map[string]DataSource{
			"error_source": errorDS,
		},
	}

	ctx := t.Context()
	err := processor.Shutdown(ctx)
	assert.NoError(t, err) // Should not return error even if data source Stop fails
}

func TestEmptyConfiguration(t *testing.T) {
	processor := &enrichmentProcessor{
		config:      &Config{},
		logger:      zap.NewNop(),
		dataSources: make(map[string]DataSource),
	}

	attributes := pcommon.NewMap()
	attributes.PutStr("service.name", "test-service")

	processor.enrichAttributes(attributes, ENRICHCONTEXTINDIVIDUAL)

	// No enrichment should have occurred
	assert.Equal(t, 1, attributes.Len()) // Only the original attribute
}

func TestEnrichmentRuleErrors(t *testing.T) {
	mockDataSource := newMockDataSource([][]string{}, map[string]int{}, []string{})

	processor := &enrichmentProcessor{
		config: &Config{
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "test_rule",
					DataSource:         "mock",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            ENRICHCONTEXTINDIVIDUAL,
					Mappings: []FieldMapping{
						{
							SourceField:     "owner",
							TargetAttribute: "team.name",
						},
					},
				},
			},
		},
		dataSources: map[string]DataSource{
			"mock": mockDataSource,
		},
		logger: zap.NewNop(),
	}

	t.Run("missing_lookup_attribute", func(t *testing.T) {
		attributes := pcommon.NewMap()
		// No service.name attribute

		err := processor.applyEnrichmentRule(processor.config.EnrichmentRules[0], attributes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lookup key service.name not found")
	})

	t.Run("empty_lookup_value", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "")

		err := processor.applyEnrichmentRule(processor.config.EnrichmentRules[0], attributes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lookup key service.name has empty value")
	})

	t.Run("nonexistent_data_source", func(t *testing.T) {
		rule := EnrichmentRule{
			Name:               "bad_rule",
			DataSource:         "nonexistent",
			LookupAttributeKey: "service.name",
			LookupField:        "name",
			Context:            ENRICHCONTEXTINDIVIDUAL,
			Mappings: []FieldMapping{
				{
					SourceField:     "owner",
					TargetAttribute: "team.name",
				},
			},
		}

		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "test-service")

		err := processor.applyEnrichmentRule(rule, attributes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "data source nonexistent not found")
	})
}

func TestConfigValidationAdditionalCases(t *testing.T) {
	t.Run("duplicate_data_source_names", func(t *testing.T) {
		config := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "duplicate",
					Type: "http",
					HTTP: &HTTPDataSourceConfig{URL: "http://example.com"},
				},
				{
					Name: "duplicate",
					Type: "file",
					File: &FileDataSourceConfig{Path: "/tmp/test.csv"},
				},
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "test_rule",
					DataSource:         "duplicate",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Mappings: []FieldMapping{
						{SourceField: "field", TargetAttribute: "attr"},
					},
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate data source name: duplicate")
	})

	t.Run("duplicate_rule_names", func(t *testing.T) {
		config := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "source",
					Type: "http",
					HTTP: &HTTPDataSourceConfig{URL: "http://example.com"},
				},
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "duplicate_rule",
					DataSource:         "source",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Mappings: []FieldMapping{
						{SourceField: "field", TargetAttribute: "attr"},
					},
				},
				{
					Name:               "duplicate_rule",
					DataSource:         "source",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Mappings: []FieldMapping{
						{SourceField: "field2", TargetAttribute: "attr2"},
					},
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate enrichment rule name: duplicate_rule")
	})

	t.Run("invalid_context_value", func(t *testing.T) {
		config := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "source",
					Type: "http",
					HTTP: &HTTPDataSourceConfig{URL: "http://example.com"},
				},
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "test_rule",
					DataSource:         "source",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Context:            "invalid_context",
					Mappings: []FieldMapping{
						{SourceField: "field", TargetAttribute: "attr"},
					},
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid context value invalid_context")
	})

	t.Run("empty_mapping_fields", func(t *testing.T) {
		config := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "source",
					Type: "http",
					HTTP: &HTTPDataSourceConfig{URL: "http://example.com"},
				},
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "test_rule",
					DataSource:         "source",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Mappings: []FieldMapping{
						{SourceField: "", TargetAttribute: "attr"},
					},
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source field cannot be empty")
	})

	t.Run("invalid_http_format", func(t *testing.T) {
		config := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "source",
					Type: "http",
					HTTP: &HTTPDataSourceConfig{
						URL:    "http://example.com",
						Format: "invalid_format",
					},
				},
			},
			EnrichmentRules: []EnrichmentRule{
				{
					Name:               "test_rule",
					DataSource:         "source",
					LookupAttributeKey: "service.name",
					LookupField:        "name",
					Mappings: []FieldMapping{
						{SourceField: "field", TargetAttribute: "attr"},
					},
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported format 'invalid_format'")
	})
}

func TestNewEnrichmentProcessorErrors(t *testing.T) {
	t.Run("invalid_config_validation", func(t *testing.T) {
		config := &Config{
			DataSources: []DataSourceConfig{
				{
					Name: "", // Empty name should cause validation error
					Type: "http",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one enrichment rule must be configured")
	})

	t.Run("unsupported_data_source_type", func(t *testing.T) {
		processor := &enrichmentProcessor{
			config: &Config{
				EnrichmentRules: []EnrichmentRule{
					{
						Name:        "test_rule",
						DataSource:  "test_source",
						LookupField: "name",
					},
				},
			},
			logger: zap.NewNop(),
		}

		config := DataSourceConfig{
			Name: "test_source",
			Type: "unknown_type", // This should cause createDataSource to fail
		}

		dataSource, err := processor.createDataSource(config)
		assert.Error(t, err)
		assert.Nil(t, dataSource)
		assert.Contains(t, err.Error(), "unsupported data source type: unknown_type")
	})
}

func TestApplyMappingsEdgeCases(t *testing.T) {
	processor := &enrichmentProcessor{
		logger: zap.NewNop(),
	}

	t.Run("column_index_out_of_range", func(t *testing.T) {
		enrichmentRow := []string{"value1", "value2"}
		headerIndex := map[string]int{
			"field1": 0,
			"field2": 1,
			"field3": 5, // Out of range
		}

		mappings := []FieldMapping{
			{SourceField: "field1", TargetAttribute: "attr1"},
			{SourceField: "field3", TargetAttribute: "attr3"}, // Should be skipped
		}

		attributes := pcommon.NewMap()
		processor.applyMappings(mappings, enrichmentRow, headerIndex, attributes)

		// Only field1 should be applied
		value1, exists := attributes.Get("attr1")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "value1", value1.AsString())
		}

		// field3 should not be applied due to out of range
		_, exists = attributes.Get("attr3")
		assert.False(t, exists)
	})

	t.Run("empty_value_not_applied", func(t *testing.T) {
		enrichmentRow := []string{"", "value2"}
		headerIndex := map[string]int{
			"field1": 0,
			"field2": 1,
		}

		mappings := []FieldMapping{
			{SourceField: "field1", TargetAttribute: "attr1"}, // Empty value
			{SourceField: "field2", TargetAttribute: "attr2"}, // Non-empty value
		}

		attributes := pcommon.NewMap()
		processor.applyMappings(mappings, enrichmentRow, headerIndex, attributes)

		// field1 should not be applied due to empty value
		_, exists := attributes.Get("attr1")
		assert.False(t, exists)

		// field2 should be applied
		value2, exists := attributes.Get("attr2")
		assert.True(t, exists)
		if exists {
			assert.Equal(t, "value2", value2.AsString())
		}
	})
}
