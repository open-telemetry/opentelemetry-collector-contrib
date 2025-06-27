// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate_EmptyConfig(t *testing.T) {
	config := &Config{
		DataSources:     []DataSourceConfig{},
		EnrichmentRules: []EnrichmentRule{},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one data source must be configured")
}

func TestConfigValidate_EmptyRules(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
			},
		},
		EnrichmentRules: []EnrichmentRule{},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one enrichment rule must be configured")
}

func TestConfigValidate_EmptyDataSourceName(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "",
				Type: "http",
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "test-source",
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
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "data source name cannot be empty")
}

func TestConfigValidate_DuplicateDataSourceName(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "duplicate-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
			{
				Name: "duplicate-source",
				Type: "file",
				File: &FileDataSourceConfig{
					Path:   "/path/to/file.json",
					Format: "json",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "duplicate-source",
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
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate data source name: duplicate-source")
}

func TestConfigValidate_UnsupportedDataSourceType(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "unsupported",
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "test-source",
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
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported data source type: unsupported")
}

func TestConfigValidate_EmptyRuleName(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "",
				DataSource:  "test-source",
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
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "enrichment rule name cannot be empty")
}

func TestConfigValidate_DuplicateRuleName(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "duplicate-rule",
				DataSource:  "test-source",
				LookupKey:   "key",
				LookupField: "field",
				Mappings: []FieldMapping{
					{
						SourceField:     "source",
						TargetAttribute: "target",
					},
				},
			},
			{
				Name:        "duplicate-rule",
				DataSource:  "test-source",
				LookupKey:   "key2",
				LookupField: "field2",
				Mappings: []FieldMapping{
					{
						SourceField:     "source2",
						TargetAttribute: "target2",
					},
				},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate enrichment rule name: duplicate-rule")
}

func TestConfigValidate_MissingDataSource(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "nonexistent-source",
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
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "data source nonexistent-source not found for rule: test-rule")
}

func TestConfigValidate_EmptyLookupKey(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "test-source",
				LookupKey:   "",
				LookupField: "field",
				Mappings: []FieldMapping{
					{
						SourceField:     "source",
						TargetAttribute: "target",
					},
				},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lookup key must be specified for rule: test-rule")
}

func TestConfigValidate_EmptyLookupField(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "test-source",
				LookupKey:   "key",
				LookupField: "",
				Mappings: []FieldMapping{
					{
						SourceField:     "source",
						TargetAttribute: "target",
					},
				},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lookup field must be specified for rule: test-rule")
}

func TestConfigValidate_EmptyMappings(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "test-source",
				LookupKey:   "key",
				LookupField: "field",
				Mappings:    []FieldMapping{},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one mapping must be specified for rule: test-rule")
}

func TestConfigValidate_EmptySourceField(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "test-source",
				LookupKey:   "key",
				LookupField: "field",
				Mappings: []FieldMapping{
					{
						SourceField:     "",
						TargetAttribute: "target",
					},
				},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source field cannot be empty in rule: test-rule")
}

func TestConfigValidate_EmptyTargetAttribute(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "test-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL: "http://example.com",
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:        "test-rule",
				DataSource:  "test-source",
				LookupKey:   "key",
				LookupField: "field",
				Mappings: []FieldMapping{
					{
						SourceField:     "source",
						TargetAttribute: "",
					},
				},
			},
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target attribute cannot be empty in rule: test-rule")
}

func TestConfigValidate_ValidConfig(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "http-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL:             "http://example.com/api",
					Timeout:         30 * time.Second,
					RefreshInterval: 5 * time.Minute,
				},
			},
			{
				Name: "file-source",
				Type: "file",
				File: &FileDataSourceConfig{
					Path:            "/path/to/file.json",
					Format:          "json",
					RefreshInterval: 1 * time.Minute,
				},
			},
			{
				Name: "prometheus-source",
				Type: "prometheus",
				Prometheus: &PrometheusDataSourceConfig{
					URL:             "http://prometheus:9090",
					Query:           "up",
					RefreshInterval: 30 * time.Second,
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
				Name:        "rule1",
				DataSource:  "http-source",
				LookupKey:   "service.name",
				LookupField: "name",
				Conditions: []Condition{
					{
						Attribute: "environment",
						Operator:  "equals",
						Value:     "production",
					},
				},
				Mappings: []FieldMapping{
					{
						SourceField:     "owner",
						TargetAttribute: "team.name",
						Transform:       "lower",
					},
					{
						SourceField:     "version",
						TargetAttribute: "service.version",
					},
				},
			},
			{
				Name:        "rule2",
				DataSource:  "file-source",
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

	err := config.Validate()
	assert.NoError(t, err)
}

func TestDataSourceConfigs(t *testing.T) {
	t.Run("HTTPDataSourceConfig", func(t *testing.T) {
		config := HTTPDataSourceConfig{
			URL: "https://api.example.com/inventory",
			Headers: map[string]string{
				"Authorization": "Bearer token123",
				"Content-Type":  "application/json",
			},
			Timeout:         30 * time.Second,
			RefreshInterval: 5 * time.Minute,
			JSONPath:        "$.data",
		}

		assert.Equal(t, "https://api.example.com/inventory", config.URL)
		assert.Equal(t, "Bearer token123", config.Headers["Authorization"])
		assert.Equal(t, 30*time.Second, config.Timeout)
		assert.Equal(t, 5*time.Minute, config.RefreshInterval)
		assert.Equal(t, "$.data", config.JSONPath)
	})

	t.Run("FileDataSourceConfig", func(t *testing.T) {
		config := FileDataSourceConfig{
			Path:            "/etc/inventory/services.json",
			Format:          "json",
			RefreshInterval: 2 * time.Minute,
		}

		assert.Equal(t, "/etc/inventory/services.json", config.Path)
		assert.Equal(t, "json", config.Format)
		assert.Equal(t, 2*time.Minute, config.RefreshInterval)
	})

	t.Run("PrometheusDataSourceConfig", func(t *testing.T) {
		config := PrometheusDataSourceConfig{
			URL:   "http://prometheus:9090",
			Query: "up{job=\"kubernetes-pods\"}",
			Headers: map[string]string{
				"X-Custom-Header": "value",
			},
			RefreshInterval: 1 * time.Minute,
		}

		assert.Equal(t, "http://prometheus:9090", config.URL)
		assert.Equal(t, "up{job=\"kubernetes-pods\"}", config.Query)
		assert.Equal(t, "value", config.Headers["X-Custom-Header"])
		assert.Equal(t, 1*time.Minute, config.RefreshInterval)
	})
}

func TestCacheConfig(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     10 * time.Minute,
		MaxSize: 2000,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 10*time.Minute, config.TTL)
	assert.Equal(t, 2000, config.MaxSize)
}

func TestEnrichmentRule(t *testing.T) {
	rule := EnrichmentRule{
		Name:        "test-enrichment",
		DataSource:  "inventory-api",
		LookupKey:   "service.name",
		LookupField: "service_name",
		Conditions: []Condition{
			{
				Attribute: "environment",
				Operator:  "equals",
				Value:     "production",
			},
			{
				Attribute: "service.type",
				Operator:  "contains",
				Value:     "database",
			},
		},
		Mappings: []FieldMapping{
			{
				SourceField:     "owner_team",
				TargetAttribute: "team.name",
				Transform:       "lower",
			},
			{
				SourceField:     "region",
				TargetAttribute: "geo.region",
			},
		},
	}

	assert.Equal(t, "test-enrichment", rule.Name)
	assert.Equal(t, "inventory-api", rule.DataSource)
	assert.Equal(t, "service.name", rule.LookupKey)
	assert.Equal(t, "service_name", rule.LookupField)
	assert.Len(t, rule.Conditions, 2)
	assert.Len(t, rule.Mappings, 2)

	// Test conditions
	assert.Equal(t, "environment", rule.Conditions[0].Attribute)
	assert.Equal(t, "equals", rule.Conditions[0].Operator)
	assert.Equal(t, "production", rule.Conditions[0].Value)

	assert.Equal(t, "service.type", rule.Conditions[1].Attribute)
	assert.Equal(t, "contains", rule.Conditions[1].Operator)
	assert.Equal(t, "database", rule.Conditions[1].Value)

	// Test mappings
	assert.Equal(t, "owner_team", rule.Mappings[0].SourceField)
	assert.Equal(t, "team.name", rule.Mappings[0].TargetAttribute)
	assert.Equal(t, "lower", rule.Mappings[0].Transform)

	assert.Equal(t, "region", rule.Mappings[1].SourceField)
	assert.Equal(t, "geo.region", rule.Mappings[1].TargetAttribute)
	assert.Empty(t, rule.Mappings[1].Transform)
}

func TestFieldMapping(t *testing.T) {
	mapping := FieldMapping{
		SourceField:     "source_field_name",
		TargetAttribute: "target.attribute.name",
		Transform:       "upper",
	}

	assert.Equal(t, "source_field_name", mapping.SourceField)
	assert.Equal(t, "target.attribute.name", mapping.TargetAttribute)
	assert.Equal(t, "upper", mapping.Transform)
}

func TestCondition(t *testing.T) {
	condition := Condition{
		Attribute: "service.environment",
		Operator:  "regex",
		Value:     "^prod.*",
	}

	assert.Equal(t, "service.environment", condition.Attribute)
	assert.Equal(t, "regex", condition.Operator)
	assert.Equal(t, "^prod.*", condition.Value)
}
