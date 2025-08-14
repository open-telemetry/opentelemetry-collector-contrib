// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func TestConfigValidate_EmptyAndDuplicateNames(t *testing.T) {
	testCases := []struct {
		name          string
		config        *Config
		expectedError string
	}{
		{
			name: "empty_data_source_name",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "",
						Type: "http",
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:               "test-rule",
						DataSource:         "test-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
				},
			},
			expectedError: "data source name cannot be empty",
		},
		{
			name: "empty_rule_name",
			config: &Config{
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
						Name:               "",
						DataSource:         "test-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
				},
			},
			expectedError: "enrichment rule name cannot be empty",
		},
		{
			name: "duplicate_data_source_name",
			config: &Config{
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
						Name:               "test-rule",
						DataSource:         "duplicate-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
				},
			},
			expectedError: "duplicate data source name: duplicate-source",
		},
		{
			name: "duplicate_rule_name",
			config: &Config{
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
						Name:               "duplicate-rule",
						DataSource:         "test-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
					{
						Name:               "duplicate-rule",
						DataSource:         "test-source",
						LookupAttributeKey: "key2",
						LookupField:        "field2",
						Mappings: []FieldMapping{
							{
								SourceField:     "source2",
								TargetAttribute: "target2",
							},
						},
					},
				},
			},
			expectedError: "duplicate enrichment rule name: duplicate-rule",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestConfigValidate_DataSourceAndRuleValidation(t *testing.T) {
	testCases := []struct {
		name          string
		config        *Config
		expectedError string
	}{
		{
			name: "unsupported_data_source_type",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "test-source",
						Type: "unsupported",
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:               "test-rule",
						DataSource:         "test-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
				},
			},
			expectedError: "unsupported data source type: unsupported",
		},
		{
			name: "missing_data_source",
			config: &Config{
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
						Name:               "test-rule",
						DataSource:         "nonexistent-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
				},
			},
			expectedError: "data source nonexistent-source not found for rule: test-rule",
		},
		{
			name: "invalid_context",
			config: &Config{
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
						Name:               "test-rule",
						DataSource:         "test-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Context:            "invalid-context",
						Mappings: []FieldMapping{
							{
								SourceField:     "source",
								TargetAttribute: "target",
							},
						},
					},
				},
			},
			expectedError: "invalid context value invalid-context for rule test-rule. Valid values: resource, individual",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestConfigValidate_EmptyFields(t *testing.T) {
	baseConfig := func() *Config {
		return &Config{
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
					Name:               "test-rule",
					DataSource:         "test-source",
					LookupAttributeKey: "key",
					LookupField:        "field",
					Mappings: []FieldMapping{
						{
							SourceField:     "source",
							TargetAttribute: "target",
						},
					},
				},
			},
		}
	}

	testCases := []struct {
		name          string
		modifyConfig  func(*Config)
		expectedError string
	}{
		{
			name: "empty_lookup_attribute_key",
			modifyConfig: func(c *Config) {
				c.EnrichmentRules[0].LookupAttributeKey = ""
			},
			expectedError: "lookup key must be specified for rule: test-rule",
		},
		{
			name: "empty_lookup_field",
			modifyConfig: func(c *Config) {
				c.EnrichmentRules[0].LookupField = ""
			},
			expectedError: "lookup field must be specified for rule: test-rule",
		},
		{
			name: "empty_mappings",
			modifyConfig: func(c *Config) {
				c.EnrichmentRules[0].Mappings = []FieldMapping{}
			},
			expectedError: "at least one mapping must be specified for rule: test-rule",
		},
		{
			name: "empty_source_field",
			modifyConfig: func(c *Config) {
				c.EnrichmentRules[0].Mappings[0].SourceField = ""
			},
			expectedError: "source field cannot be empty in rule: test-rule",
		},
		{
			name: "empty_target_attribute",
			modifyConfig: func(c *Config) {
				c.EnrichmentRules[0].Mappings[0].TargetAttribute = ""
			},
			expectedError: "target attribute cannot be empty in rule: test-rule",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := baseConfig()
			tc.modifyConfig(config)

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestConfigValidate_ValidContext(t *testing.T) {
	validContexts := []string{ENRICHCONTEXTRESOURCE, ENRICHCONTEXTINDIVIDUAL}

	for _, context := range validContexts {
		t.Run("context_"+context, func(t *testing.T) {
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
						Name:               "test-rule",
						DataSource:         "test-source",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Context:            context,
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
			assert.NoError(t, err)
		})
	}
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
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:               "rule1",
				DataSource:         "http-source",
				LookupAttributeKey: "service.name",
				LookupField:        "name",
				Context:            ENRICHCONTEXTINDIVIDUAL,
				Mappings: []FieldMapping{
					{
						SourceField:     "owner",
						TargetAttribute: "team.name",
					},
					{
						SourceField:     "version",
						TargetAttribute: "service.version",
					},
				},
			},
			{
				Name:               "rule2",
				DataSource:         "file-source",
				LookupAttributeKey: "host.name",
				LookupField:        "hostname",
				Context:            ENRICHCONTEXTRESOURCE,
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
