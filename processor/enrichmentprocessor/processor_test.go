// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// MockDataSource for testing
type MockDataSource struct {
	data        [][]string
	headerIndex map[string]int
	lookup      *Lookup
}

func NewMockDataSource(data [][]string, headerIndex map[string]int, indexFields []string) *MockDataSource {
	lookup := NewLookup()
	lookup.SetAll(data, headerIndex, indexFields)

	return &MockDataSource{
		data:        data,
		headerIndex: headerIndex,
		lookup:      lookup,
	}
}

func (m *MockDataSource) Lookup(ctx context.Context, lookupField string, key string) (enrichmentRow []string, index map[string]int, err error) {
	return m.lookup.Lookup(ctx, lookupField, key)
}

func (m *MockDataSource) Start(ctx context.Context) error {
	return nil
}

func (m *MockDataSource) Stop() error {
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

	mockDataSource := NewMockDataSource(data, headerIndex, indexFields)

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

	ctx := context.Background()

	t.Run("successful_enrichment", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "test-service")

		err := processor.applyEnrichmentRule(ctx, processor.config.EnrichmentRules[0], attributes)
		assert.NoError(t, err)

		// Check that attributes were added
		teamName, exists := attributes.Get("team.name")
		assert.True(t, exists)
		assert.Equal(t, "platform-team", teamName.AsString())

		environment, exists := attributes.Get("deployment.environment")
		assert.True(t, exists)
		assert.Equal(t, "production", environment.AsString())
	})

	t.Run("enrichment_not_found", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "non-existent-service")

		err := processor.applyEnrichmentRule(ctx, processor.config.EnrichmentRules[0], attributes)
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

		err := processor.applyEnrichmentRule(ctx, environmentRule, attributes)
		assert.NoError(t, err)

		// Check that attributes were added based on environment lookup
		serviceName, exists := attributes.Get("service.name")
		assert.True(t, exists)
		assert.Equal(t, "payment-service", serviceName.AsString())

		teamName, exists := attributes.Get("team.name")
		assert.True(t, exists)
		assert.Equal(t, "payments-team", teamName.AsString())
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
				assert.Equal(t, expectedValue, value.AsString())
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

	mockDataSource := NewMockDataSource(data, headerIndex, indexFields)

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

	ctx := context.Background()

	t.Run("resource_context_filtering", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "test-service")

		err := processor.enrichAttributes(ctx, attributes, ENRICHCONTEXTRESOURCE)
		assert.NoError(t, err)

		// Should have team.name from resource_rule
		teamName, exists := attributes.Get("team.name")
		assert.True(t, exists)
		assert.Equal(t, "platform-team", teamName.AsString())

		// Should not have service.version from individual_rule
		_, exists = attributes.Get("service.version")
		assert.False(t, exists)
	})

	t.Run("individual_context_filtering", func(t *testing.T) {
		attributes := pcommon.NewMap()
		attributes.PutStr("service.name", "test-service")

		err := processor.enrichAttributes(ctx, attributes, ENRICHCONTEXTINDIVIDUAL)
		assert.NoError(t, err)

		// Should have service.version from individual_rule
		version, exists := attributes.Get("service.version")
		assert.True(t, exists)
		assert.Equal(t, "1.2.3", version.AsString())

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

	mockDataSource := NewMockDataSource(data, headerIndex, indexFields)

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

	ctx := context.Background()

	// Test case 1: previously matching condition — now enrichment always applies
	attributes := pcommon.NewMap()
	attributes.PutStr("service.name", "test-service")
	attributes.PutStr("environment", "production")

	err := processor.enrichAttributes(ctx, attributes, ENRICHCONTEXTINDIVIDUAL)
	assert.NoError(t, err)

	teamName, exists := attributes.Get("team.name")
	assert.True(t, exists)
	assert.Equal(t, "platform-team", teamName.AsString())

	// Test case 2: previously non-matching condition — enrichment still applies
	attributes2 := pcommon.NewMap()
	attributes2.PutStr("service.name", "test-service")
	attributes2.PutStr("environment", "staging")

	err = processor.enrichAttributes(ctx, attributes2, ENRICHCONTEXTINDIVIDUAL)
	assert.NoError(t, err)

	teamName2, exists := attributes2.Get("team.name")
	assert.True(t, exists)
	assert.Equal(t, "platform-team", teamName2.AsString())
}
