// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"errors"
	"time"
)

const (
	// ENRICHCONTEXTRESOURCE indicates that the enrichment rule applies to resource attributes
	ENRICHCONTEXTRESOURCE = "resource"
	// ENRICHCONTEXTINDIVIDUAL indicates that the enrichment rule applies to log/metric/span attributes
	ENRICHCONTEXTINDIVIDUAL = "individual"
)

// Config defines the configuration for the enrichment processor.
type Config struct {
	// DataSources defines the external data sources for enrichment
	DataSources []DataSourceConfig `mapstructure:"data_sources"`

	// EnrichmentRules defines how to enrich telemetry data
	EnrichmentRules []EnrichmentRule `mapstructure:"enrichment_rules"`
}

// DataSourceConfig defines configuration for an external data source
type DataSourceConfig struct {
	// Name is a unique identifier for this data source
	Name string `mapstructure:"name"`

	// Type specifies the data source type (http, file)
	Type string `mapstructure:"type"`

	// HTTP configuration (used when Type is "http")
	HTTP *HTTPDataSourceConfig `mapstructure:"http,omitempty"`

	// File configuration (used when Type is "file")
	File *FileDataSourceConfig `mapstructure:"file,omitempty"`
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

	// Format specifies the data format (json, csv)
	// If not specified, will be auto-detected from Content-Type header
	Format string `mapstructure:"format"`
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

// EnrichmentRule defines how to enrich telemetry data
type EnrichmentRule struct {
	// Name is a unique identifier for this rule
	Name string `mapstructure:"name"`

	// DataSource specifies which data source to use
	DataSource string `mapstructure:"data_source"`

	// LookupAttributeKey specifies which attribute/field to use for lookup
	LookupAttributeKey string `mapstructure:"lookup_attributekey"`

	// LookupField specifies which field in the data source to match against
	LookupField string `mapstructure:"lookup_field"`

	// Mappings define how to map data source fields to telemetry attributes
	Mappings []FieldMapping `mapstructure:"mappings"`

	// Context specifies which telemetry context to enrich
	// Valid values: "resource", "span", "metric", "log"
	// Default: applies to all contexts if not specified
	Context string `mapstructure:"context"`
}

// FieldMapping defines how to map a field from data source to telemetry attribute
type FieldMapping struct {
	// SourceField is the field name in the data source
	SourceField string `mapstructure:"source_field"`

	// TargetAttribute is the attribute name in telemetry data
	TargetAttribute string `mapstructure:"target_attribute"`
}

func (config *Config) Validate() error {
	// Allow empty configuration for default config
	if len(config.DataSources) == 0 && len(config.EnrichmentRules) == 0 {
		return nil
	}

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
			return errors.New("duplicate data source name: " + ds.Name)
		}
		dataSourceNames[ds.Name] = true

		if ds.Type == "" {
			return errors.New("data source type cannot be empty for data source: " + ds.Name)
		}

		if ds.Type != "http" && ds.Type != "file" {
			return errors.New("unsupported data source type: " + ds.Type)
		}

		// Validate type-specific configuration
		switch ds.Type {
		case "http":
			if ds.HTTP == nil {
				return errors.New("HTTP configuration is required for http data source: " + ds.Name)
			}
			if ds.HTTP.URL == "" {
				return errors.New("URL is required for HTTP data source: " + ds.Name)
			}
			// Validate format if specified
			if ds.HTTP.Format != "" && ds.HTTP.Format != "json" && ds.HTTP.Format != "csv" {
				return errors.New("unsupported format '" + ds.HTTP.Format + "' for HTTP data source '" + ds.Name + "'. Valid formats: json, csv")
			}
		case "file":
			if ds.File == nil {
				return errors.New("File configuration is required for file data source: " + ds.Name)
			}
			if ds.File.Path == "" {
				return errors.New("Path is required for file data source: " + ds.Name)
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
			return errors.New("duplicate enrichment rule name: " + rule.Name)
		}
		ruleNames[rule.Name] = true

		if rule.DataSource == "" {
			return errors.New("data source must be specified for rule: " + rule.Name)
		}

		if !dataSourceNames[rule.DataSource] {
			return errors.New("data source " + rule.DataSource + " not found for rule: " + rule.Name)
		}

		if rule.LookupAttributeKey == "" {
			return errors.New("lookup key must be specified for rule: " + rule.Name)
		}

		if rule.LookupField == "" {
			return errors.New("lookup field must be specified for rule: " + rule.Name)
		}

		if len(rule.Mappings) == 0 {
			return errors.New("at least one mapping must be specified for rule: " + rule.Name)
		}
		// Validate context if specified
		if rule.Context != "" {
			if rule.Context != ENRICHCONTEXTRESOURCE && rule.Context != ENRICHCONTEXTINDIVIDUAL {
				return errors.New("invalid context value " + rule.Context + " for rule " + rule.Name + ". Valid values: resource, individual")
			}
		}
		// Validate mappings
		for _, mapping := range rule.Mappings {
			if mapping.SourceField == "" {
				return errors.New("source field cannot be empty in rule: " + rule.Name)
			}

			if mapping.TargetAttribute == "" {
				return errors.New("target attribute cannot be empty in rule: " + rule.Name)
			}
		}
	}

	return nil
}
