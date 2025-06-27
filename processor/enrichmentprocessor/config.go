// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"errors"
	"fmt"
	"time"
)

// Config defines the configuration for the enrichment processor.
type Config struct {
	// DataSources defines the external data sources for enrichment
	DataSources []DataSourceConfig `mapstructure:"data_sources"`

	// Cache configuration
	Cache CacheConfig `mapstructure:"cache"`

	// EnrichmentRules defines how to enrich telemetry data
	EnrichmentRules []EnrichmentRule `mapstructure:"enrichment_rules"`
}

// DataSourceConfig defines configuration for an external data source
type DataSourceConfig struct {
	// Name is a unique identifier for this data source
	Name string `mapstructure:"name"`

	// Type specifies the data source type (http, file, prometheus)
	Type string `mapstructure:"type"`

	// HTTP configuration (used when Type is "http")
	HTTP *HTTPDataSourceConfig `mapstructure:"http,omitempty"`

	// File configuration (used when Type is "file")
	File *FileDataSourceConfig `mapstructure:"file,omitempty"`

	// Prometheus configuration (used when Type is "prometheus")
	Prometheus *PrometheusDataSourceConfig `mapstructure:"prometheus,omitempty"`
}

// HTTPDataSourceConfig defines configuration for HTTP-based data sources
type HTTPDataSourceConfig struct {
	// URL is the HTTP endpoint URL
	URL string `mapstructure:"url"`

	// Headers to include in the request
	Headers map[string]string `mapstructure:"headers"`

	// Timeout for HTTP requests
	Timeout time.Duration `mapstructure:"timeout"`

	// RefreshInterval specifies how often to refresh the data
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`

	// JSONPath to extract relevant data from the response
	JSONPath string `mapstructure:"json_path"`
}

// FileDataSourceConfig defines configuration for file-based data sources
type FileDataSourceConfig struct {
	// Path to the file
	Path string `mapstructure:"path"`

	// Format of the file (json, csv, yaml)
	Format string `mapstructure:"format"`

	// RefreshInterval specifies how often to check for file changes
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// PrometheusDataSourceConfig defines configuration for Prometheus-based data sources
type PrometheusDataSourceConfig struct {
	// URL is the Prometheus endpoint URL
	URL string `mapstructure:"url"`

	// Query is the PromQL query to execute
	Query string `mapstructure:"query"`

	// Headers to include in the request
	Headers map[string]string `mapstructure:"headers"`

	// RefreshInterval specifies how often to refresh the data
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// CacheConfig defines cache configuration
type CacheConfig struct {
	// Enabled determines if caching is enabled
	Enabled bool `mapstructure:"enabled"`

	// TTL defines how long to cache entries
	TTL time.Duration `mapstructure:"ttl"`

	// MaxSize defines the maximum number of entries to cache
	MaxSize int `mapstructure:"max_size"`
}

// EnrichmentRule defines how to enrich telemetry data
type EnrichmentRule struct {
	// Name is a unique identifier for this rule
	Name string `mapstructure:"name"`

	// DataSource specifies which data source to use
	DataSource string `mapstructure:"data_source"`

	// LookupKey specifies which attribute/field to use for lookup
	LookupKey string `mapstructure:"lookup_key"`

	// LookupField specifies which field in the data source to match against
	LookupField string `mapstructure:"lookup_field"`

	// Mappings define how to map data source fields to telemetry attributes
	Mappings []FieldMapping `mapstructure:"mappings"`

	// Conditions define when this rule should be applied
	Conditions []Condition `mapstructure:"conditions"`
}

// FieldMapping defines how to map a field from data source to telemetry attribute
type FieldMapping struct {
	// SourceField is the field name in the data source
	SourceField string `mapstructure:"source_field"`

	// TargetAttribute is the attribute name in telemetry data
	TargetAttribute string `mapstructure:"target_attribute"`

	// Transform optionally specifies a transformation function
	Transform string `mapstructure:"transform"`
}

// Condition defines when an enrichment rule should be applied
type Condition struct {
	// Attribute to check
	Attribute string `mapstructure:"attribute"`

	// Operator for comparison (equals, contains, regex, etc.)
	Operator string `mapstructure:"operator"`

	// Value to compare against
	Value string `mapstructure:"value"`
}

func (config *Config) Validate() error {
	if len(config.DataSources) == 0 {
		return errors.New("at least one data source must be configured")
	}

	if len(config.EnrichmentRules) == 0 {
		return errors.New("at least one enrichment rule must be configured")
	}

	// Validate data sources
	dataSourceNames := make(map[string]bool)
	for _, ds := range config.DataSources {
		if ds.Name == "" {
			return errors.New("data source name cannot be empty")
		}

		if dataSourceNames[ds.Name] {
			return fmt.Errorf("duplicate data source name: %s", ds.Name)
		}
		dataSourceNames[ds.Name] = true

		if ds.Type == "" {
			return fmt.Errorf("data source type cannot be empty for data source: %s", ds.Name)
		}

		if ds.Type != "http" && ds.Type != "file" && ds.Type != "prometheus" {
			return fmt.Errorf("unsupported data source type: %s", ds.Type)
		}

		// Validate type-specific configuration
		switch ds.Type {
		case "http":
			if ds.HTTP == nil {
				return fmt.Errorf("HTTP configuration is required for http data source: %s", ds.Name)
			}
			if ds.HTTP.URL == "" {
				return fmt.Errorf("URL is required for HTTP data source: %s", ds.Name)
			}
		case "file":
			if ds.File == nil {
				return fmt.Errorf("File configuration is required for file data source: %s", ds.Name)
			}
			if ds.File.Path == "" {
				return fmt.Errorf("Path is required for file data source: %s", ds.Name)
			}
		case "prometheus":
			if ds.Prometheus == nil {
				return fmt.Errorf("Prometheus configuration is required for prometheus data source: %s", ds.Name)
			}
			if ds.Prometheus.URL == "" {
				return fmt.Errorf("URL is required for Prometheus data source: %s", ds.Name)
			}
		}
	}

	// Validate enrichment rules
	ruleNames := make(map[string]bool)
	for _, rule := range config.EnrichmentRules {
		if rule.Name == "" {
			return errors.New("enrichment rule name cannot be empty")
		}

		if ruleNames[rule.Name] {
			return fmt.Errorf("duplicate enrichment rule name: %s", rule.Name)
		}
		ruleNames[rule.Name] = true

		if rule.DataSource == "" {
			return fmt.Errorf("data source must be specified for rule: %s", rule.Name)
		}

		if !dataSourceNames[rule.DataSource] {
			return fmt.Errorf("data source %s not found for rule: %s", rule.DataSource, rule.Name)
		}

		if rule.LookupKey == "" {
			return fmt.Errorf("lookup key must be specified for rule: %s", rule.Name)
		}

		if rule.LookupField == "" {
			return fmt.Errorf("lookup field must be specified for rule: %s", rule.Name)
		}

		if len(rule.Mappings) == 0 {
			return fmt.Errorf("at least one mapping must be specified for rule: %s", rule.Name)
		}

		// Validate mappings
		for _, mapping := range rule.Mappings {
			if mapping.SourceField == "" {
				return fmt.Errorf("source field cannot be empty in rule: %s", rule.Name)
			}

			if mapping.TargetAttribute == "" {
				return fmt.Errorf("target attribute cannot be empty in rule: %s", rule.Name)
			}
		}
	}

	return nil
}
