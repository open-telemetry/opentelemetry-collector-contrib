// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate_ValidConfig(t *testing.T) {
	config := &Config{
		DataSources: []DataSourceConfig{
			{
				Name: "http-source",
				Type: "http",
				HTTP: &HTTPDataSourceConfig{
					URL:             "http://example.com/api",
					Format:          "json",
					Timeout:         30 * time.Second,
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
		EnrichmentRules: []EnrichmentRule{
			{
				Name:               "test-rule",
				DataSource:         "http-source",
				LookupAttributeKey: "service.name",
				LookupField:        "name",
				Mappings: []FieldMapping{
					{
						SourceField:     "owner",
						TargetAttribute: "team.name",
					},
				},
			},
		},
	}

	err := config.Validate()
	assert.NoError(t, err)
}

func TestConfigValidate_InvalidCases(t *testing.T) {
	testCases := []struct {
		name          string
		config        *Config
		expectedError string
	}{
		{
			name: "empty_data_sources",
			config: &Config{
				EnrichmentRules: []EnrichmentRule{
					{Name: "rule", DataSource: "source"},
				},
			},
			expectedError: "at least one data source must be configured",
		},
		{
			name: "missing_http_url",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "test",
						Type: "http",
						HTTP: &HTTPDataSourceConfig{},
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{Name: "rule", DataSource: "test"},
				},
			},
			expectedError: "URL is required for HTTP data source",
		},
		{
			name: "invalid_http_format",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "test",
						Type: "http",
						HTTP: &HTTPDataSourceConfig{
							URL:    "http://example.com",
							Format: "xml",
						},
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{Name: "rule", DataSource: "test"},
				},
			},
			expectedError: "unsupported format 'xml' for HTTP data source 'test'. Valid formats: json, csv",
		},
		{
			name: "missing_rule_mappings",
			config: &Config{
				DataSources: []DataSourceConfig{
					{
						Name: "test",
						Type: "http",
						HTTP: &HTTPDataSourceConfig{URL: "http://example.com"},
					},
				},
				EnrichmentRules: []EnrichmentRule{
					{
						Name:               "rule",
						DataSource:         "test",
						LookupAttributeKey: "key",
						LookupField:        "field",
						Mappings:           []FieldMapping{},
					},
				},
			},
			expectedError: "at least one mapping must be specified",
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
