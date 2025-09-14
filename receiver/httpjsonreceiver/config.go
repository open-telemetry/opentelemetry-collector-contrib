package httpjsonreceiver // import "httpjsonreceiver"

import (
	"errors"
	"fmt"
	"net/url"
	"time"
)

type Config struct {
	CollectionInterval time.Duration     `mapstructure:"collection_interval"`
	InitialDelay       time.Duration     `mapstructure:"initial_delay"`
	Timeout            time.Duration     `mapstructure:"timeout"`
	Endpoints          []EndpointConfig  `mapstructure:"endpoints"`
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`
}

type EndpointConfig struct {
	URL     string            `mapstructure:"url"`
	Method  string            `mapstructure:"method"`
	Headers map[string]string `mapstructure:"headers"`
	Body    string            `mapstructure:"body,omitempty"`
	Name    string            `mapstructure:"name,omitempty"`
	Metrics []MetricConfig    `mapstructure:"metrics"`
	Timeout time.Duration     `mapstructure:"timeout,omitempty"`
}

type MetricConfig struct {
	Name        string            `mapstructure:"name"`
	JSONPath    string            `mapstructure:"json_path"`
	Type        string            `mapstructure:"type"`
	Description string            `mapstructure:"description,omitempty"`
	Unit        string            `mapstructure:"unit,omitempty"`
	Attributes  map[string]string `mapstructure:"attributes,omitempty"`
	ValueType   string            `mapstructure:"value_type,omitempty"`
}

func (cfg *Config) Validate() error {
	if len(cfg.Endpoints) == 0 {
		return errors.New("at least one endpoint must be specified")
	}

	if cfg.CollectionInterval <= 0 {
		cfg.CollectionInterval = 60 * time.Second
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	for i, endpoint := range cfg.Endpoints {
		if err := cfg.validateEndpoint(i, &endpoint); err != nil {
			return err
		}
	}

	return nil
}

// In config.go, update the validateEndpoint method:
func (cfg *Config) validateEndpoint(index int, endpoint *EndpointConfig) error {
	if endpoint.URL == "" {
		return fmt.Errorf("endpoints[%d]: url is required", index)
	}

	if _, err := url.Parse(endpoint.URL); err != nil {
		return fmt.Errorf("endpoints[%d]: invalid url: %w", index, err)
	}

	// Apply defaults
	if endpoint.Method == "" {
		cfg.Endpoints[index].Method = "GET" // Fix: assign to cfg.Endpoints[index]
	}

	if len(endpoint.Metrics) == 0 {
		return fmt.Errorf("endpoints[%d]: no metrics configured", index)
	}

	for j, metric := range endpoint.Metrics {
		if err := cfg.validateMetric(fmt.Sprintf("endpoints[%d].metrics[%d]", index, j), &metric, index, j); err != nil {
			return err
		}
	}

	return nil
}

// Update validateMetric to also apply defaults:
func (cfg *Config) validateMetric(prefix string, metric *MetricConfig, endpointIndex, metricIndex int) error {
	if metric.Name == "" {
		return fmt.Errorf("%s: name is required", prefix)
	}

	if metric.JSONPath == "" {
		return fmt.Errorf("%s: json_path is required", prefix)
	}

	// Apply defaults
	if metric.Type == "" {
		cfg.Endpoints[endpointIndex].Metrics[metricIndex].Type = "gauge"
	} else {
		switch metric.Type {
		case "gauge", "counter", "histogram":
			// Valid types
		default:
			return fmt.Errorf("%s: invalid type %q, must be one of: gauge, counter, histogram", prefix, metric.Type)
		}
	}

	if metric.ValueType == "" {
		cfg.Endpoints[endpointIndex].Metrics[metricIndex].ValueType = "double"
	} else {
		switch metric.ValueType {
		case "int", "double":
			// Valid types
		default:
			return fmt.Errorf("%s: invalid value_type %q, must be one of: int, double", prefix, metric.ValueType)
		}
	}

	return nil
}
