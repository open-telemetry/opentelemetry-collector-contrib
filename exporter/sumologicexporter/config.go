// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Sumo Logic exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	configretry.BackOffConfig     `mapstructure:"retry_on_failure"`

	// Compression encoding format, either empty string, gzip or deflate (default gzip)
	// Empty string means no compression
	CompressEncoding CompressEncodingType `mapstructure:"compress_encoding"`
	// Max HTTP request body size in bytes before compression (if applied).
	// By default 1MB is recommended.
	MaxRequestBodySize int `mapstructure:"max_request_body_size"`

	// Logs related configuration
	// Format to post logs into Sumo. (default json)
	//   * text - Logs will appear in Sumo Logic in text format.
	//   * json - Logs will appear in Sumo Logic in json format.
	LogFormat LogFormatType `mapstructure:"log_format"`

	// Metrics related configuration
	// The format of metrics you will be sending, either graphite or carbon2 or prometheus (Default is prometheus)
	// Possible values are `carbon2` and `prometheus`
	MetricFormat MetricFormatType `mapstructure:"metric_format"`
	// Graphite template.
	// Placeholders `%{attr_name}` will be replaced with attribute value for attr_name.
	GraphiteTemplate string `mapstructure:"graphite_template"`

	// List of regexes for attributes which should be send as metadata
	MetadataAttributes []string `mapstructure:"metadata_attributes"`

	// Sumo specific options
	// Desired source category.
	// Useful if you want to override the source category configured for the source.
	// Placeholders `%{attr_name}` will be replaced with attribute value for attr_name.
	SourceCategory string `mapstructure:"source_category"`
	// Desired source name.
	// Useful if you want to override the source name configured for the source.
	// Placeholders `%{attr_name}` will be replaced with attribute value for attr_name.
	SourceName string `mapstructure:"source_name"`
	// Desired host name.
	// Useful if you want to override the source host configured for the source.
	// Placeholders `%{attr_name}` will be replaced with attribute value for attr_name.
	SourceHost string `mapstructure:"source_host"`
	// Name of the client
	Client string `mapstructure:"client"`
}

// createDefaultHTTPClientSettings returns default http client settings
func createDefaultHTTPClientSettings() confighttp.HTTPClientSettings {
	return confighttp.HTTPClientSettings{
		Timeout: defaultTimeout,
	}
}

// LogFormatType represents log_format
type LogFormatType string

// MetricFormatType represents metric_format
type MetricFormatType string

// PipelineType represents type of the pipeline
type PipelineType string

// CompressEncodingType represents type of the pipeline
type CompressEncodingType string

const (
	// TextFormat represents log_format: text
	TextFormat LogFormatType = "text"
	// JSONFormat represents log_format: json
	JSONFormat LogFormatType = "json"
	// GraphiteFormat represents metric_format: text
	GraphiteFormat MetricFormatType = "graphite"
	// Carbon2Format represents metric_format: json
	Carbon2Format MetricFormatType = "carbon2"
	// PrometheusFormat represents metric_format: json
	PrometheusFormat MetricFormatType = "prometheus"
	// GZIPCompression represents compress_encoding: gzip
	GZIPCompression CompressEncodingType = "gzip"
	// DeflateCompression represents compress_encoding: deflate
	DeflateCompression CompressEncodingType = "deflate"
	// NoCompression represents disabled compression
	NoCompression CompressEncodingType = ""
	// MetricsPipeline represents metrics pipeline
	MetricsPipeline PipelineType = "metrics"
	// LogsPipeline represents metrics pipeline
	LogsPipeline PipelineType = "logs"
	// defaultTimeout
	defaultTimeout time.Duration = 5 * time.Second
	// DefaultCompress defines default Compress
	DefaultCompress bool = true
	// DefaultCompressEncoding defines default CompressEncoding
	DefaultCompressEncoding CompressEncodingType = "gzip"
	// DefaultMaxRequestBodySize defines default MaxRequestBodySize in bytes
	DefaultMaxRequestBodySize int = 1 * 1024 * 1024
	// DefaultLogFormat defines default LogFormat
	DefaultLogFormat LogFormatType = JSONFormat
	// DefaultMetricFormat defines default MetricFormat
	DefaultMetricFormat MetricFormatType = PrometheusFormat
	// DefaultSourceCategory defines default SourceCategory
	DefaultSourceCategory string = ""
	// DefaultSourceName defines default SourceName
	DefaultSourceName string = ""
	// DefaultSourceHost defines default SourceHost
	DefaultSourceHost string = ""
	// DefaultClient defines default Client
	DefaultClient string = "otelcol"
	// DefaultGraphiteTemplate defines default template for Graphite
	DefaultGraphiteTemplate string = "%{_metric_}"
)

func (cfg *Config) Validate() error {
	switch cfg.LogFormat {
	case JSONFormat:
	case TextFormat:
	default:
		return fmt.Errorf("unexpected log format: %s", cfg.LogFormat)
	}

	switch cfg.MetricFormat {
	case GraphiteFormat:
	case Carbon2Format:
	case PrometheusFormat:
	default:
		return fmt.Errorf("unexpected metric format: %s", cfg.MetricFormat)
	}

	switch cfg.CompressEncoding {
	case GZIPCompression:
	case DeflateCompression:
	case NoCompression:
	default:
		return fmt.Errorf("unexpected compression encoding: %s", cfg.CompressEncoding)
	}

	if len(cfg.HTTPClientSettings.Endpoint) == 0 {
		return errors.New("endpoint is not set")
	}

	if err := cfg.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	return nil
}
