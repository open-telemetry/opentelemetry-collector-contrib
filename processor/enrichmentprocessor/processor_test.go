// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Cache.Enabled)
	assert.Equal(t, 5*time.Minute, cfg.Cache.TTL)
	assert.Equal(t, 1000, cfg.Cache.MaxSize)
	assert.Empty(t, cfg.DataSources)
	assert.Empty(t, cfg.EnrichmentRules)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "empty config",
			config: &Config{
				DataSources:     []DataSourceConfig{},
				EnrichmentRules: []EnrichmentRule{},
			},
			wantErr: true,
		},
		{
			name: "missing data source name",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "",
						Type: "http",
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:        "test",
						DataSource:  "test",
						LookupKey:   "key",
						LookupField: "field",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "test_source",
						Type: "http",
						HTTP: &HTTPDataSourceConfig{
							URL:             "http://example.com",
							Timeout:         30 * time.Second,
							RefreshInterval: 5 * time.Minute,
						},
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:        "test_rule",
						DataSource:  "test_source",
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
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCacheOperations(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     100 * time.Millisecond,
		MaxSize: 2,
	}

	cache := NewCache(config)

	// Test set and get
	data := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	cache.Set("test", data)

	retrieved, found := cache.Get("test")
	assert.True(t, found)
	assert.Equal(t, data, retrieved)

	// Test cache miss
	_, found = cache.Get("nonexistent")
	assert.False(t, found)

	// Test TTL expiration
	time.Sleep(150 * time.Millisecond)
	_, found = cache.Get("test")
	assert.False(t, found)

	// Test max size eviction
	cache.Set("key1", data)
	cache.Set("key2", data)
	cache.Set("key3", data) // Should evict oldest

	assert.Equal(t, 2, cache.Size())
}

func TestConditionEvaluation(t *testing.T) {
	processor := &enrichmentProcessor{}

	attributes := pcommon.NewMap()
	attributes.PutStr("service.type", "database")
	attributes.PutStr("environment", "production")

	tests := []struct {
		name      string
		condition Condition
		expected  bool
	}{
		{
			name: "equals match",
			condition: Condition{
				Attribute: "service.type",
				Operator:  "equals",
				Value:     "database",
			},
			expected: true,
		},
		{
			name: "equals no match",
			condition: Condition{
				Attribute: "service.type",
				Operator:  "equals",
				Value:     "web",
			},
			expected: false,
		},
		{
			name: "contains match",
			condition: Condition{
				Attribute: "environment",
				Operator:  "contains",
				Value:     "prod",
			},
			expected: true,
		},
		{
			name: "not_equals match",
			condition: Condition{
				Attribute: "service.type",
				Operator:  "not_equals",
				Value:     "web",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.evaluateCondition(tt.condition, attributes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransformations(t *testing.T) {
	processor := &enrichmentProcessor{}

	tests := []struct {
		name      string
		transform string
		value     interface{}
		expected  interface{}
	}{
		{
			name:      "upper case",
			transform: "upper",
			value:     "hello",
			expected:  "HELLO",
		},
		{
			name:      "lower case",
			transform: "lower",
			value:     "HELLO",
			expected:  "hello",
		},
		{
			name:      "trim spaces",
			transform: "trim",
			value:     "  hello  ",
			expected:  "hello",
		},
		{
			name:      "no transform",
			transform: "",
			value:     "hello",
			expected:  "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.applyTransform(tt.transform, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessorCreation(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopSettings(metadata.Type)

	// Test traces processor creation
	tp, err := factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())
	require.Error(t, err) // Should fail due to empty config
	assert.Nil(t, tp)

	// Test metrics processor creation
	mp, err := factory.CreateMetrics(context.Background(), set, cfg, consumertest.NewNop())
	require.Error(t, err) // Should fail due to empty config
	assert.Nil(t, mp)

	// Test logs processor creation
	lp, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
	require.Error(t, err) // Should fail due to empty config
	assert.Nil(t, lp)
}

func TestFactoryType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

// MockDataSource for testing
type MockDataSource struct {
	data map[string]map[string]interface{}
}

func (m *MockDataSource) Lookup(ctx context.Context, key string) (map[string]interface{}, error) {
	if data, exists := m.data[key]; exists {
		return data, nil
	}
	return nil, assert.AnError
}

func (m *MockDataSource) Start(ctx context.Context) error {
	return nil
}

func (m *MockDataSource) Stop() error {
	return nil
}

func TestEnrichmentProcessor(t *testing.T) {
	// Create a mock data source
	mockData := map[string]map[string]interface{}{
		"test-service": {
			"owner":       "platform-team",
			"environment": "production",
			"version":     "1.2.3",
		},
	}

	mockDataSource := &MockDataSource{data: mockData}

	// Create processor with mock data source
	processor := &enrichmentProcessor{
		config: &Config{
			EnrichmentRules: []EnrichmentRule{
				{
					Name:        "test_rule",
					DataSource:  "mock",
					LookupKey:   "service.name",
					LookupField: "name",
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
		cache: NewCache(CacheConfig{Enabled: false}),
	}

	// Test enriching attributes
	attributes := pcommon.NewMap()
	attributes.PutStr("service.name", "test-service")

	err := processor.enrichAttributes(context.Background(), attributes)
	assert.NoError(t, err)

	// Check that attributes were added
	teamName, exists := attributes.Get("team.name")
	assert.True(t, exists)
	assert.Equal(t, "platform-team", teamName.AsString())

	environment, exists := attributes.Get("deployment.environment")
	assert.True(t, exists)
	assert.Equal(t, "production", environment.AsString())
}
