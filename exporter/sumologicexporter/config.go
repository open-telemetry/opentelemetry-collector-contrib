// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicexporter

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Sumo Logic exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

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
	//   * otlp - Logs will be send in otlp format and will appear in Sumo Logic in text format.
	LogFormat LogFormatType `mapstructure:"log_format"`

	// Metrics related configuration
	// The format of metrics you will be sending, either otlp or prometheus (Default is otlp)
	MetricFormat MetricFormatType `mapstructure:"metric_format"`

	// Decompose OTLP Histograms into individual metrics, similar to how they're represented in Prometheus format
	DecomposeOtlpHistograms bool `mapstructure:"decompose_otlp_histograms"`

	// Traces related configuration
	// The format of traces you will be sending, currently only otlp format is supported
	TraceFormat TraceFormatType `mapstructure:"trace_format"`

	// DEPRECATED: The below attributes only exist so we can print a nicer error
	// message about not supporting them anymore.

	// Attribute used by routingprocessor which should be dropped during data ingestion
	// This is workaround for the following issue:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/7407
	DropRoutingAttribute string `mapstructure:"routing_atttribute_to_drop"`

	// Sumo specific options
	// Name of the client
	Client string `mapstructure:"client"`

	// ClearTimestamp defines if timestamp for logs should be set to 0.
	// It indicates that backend will extract timestamp from logs.
	// This option affects OTLP format only.
	// By default this is true.
	ClearLogsTimestamp bool `mapstructure:"clear_logs_timestamp"`

	JSONLogs `mapstructure:"json_logs"`
}

type JSONLogs struct {
	// LogKey defines which key will be used to attach the log body at.
	// This option affects JSON log format only.
	// By default this is "log".
	LogKey string `mapstructure:"log_key"`
	// AddTimestamp defines whether to include a timestamp field when sending
	// JSON logs, which would contain UNIX epoch timestamp in milliseconds.
	// This option affects JSON log format only.
	// By default this is true.
	AddTimestamp bool `mapstructure:"add_timestamp"`
	// When add_timestamp is set to true then this key defines what is the name
	// of the timestamp key.
	// By default this is "timestamp".
	TimestampKey string `mapstructure:"timestamp_key"`
	// When flatten_body is set to true and log is a map,
	// log's body is going to be flattened and `log_key` won't be used
	// By default this is false.
	FlattenBody bool `mapstructure:"flatten_body"`
}

// CreateDefaultHTTPClientSettings returns default http client settings
func CreateDefaultHTTPClientSettings() confighttp.HTTPClientSettings {
	return confighttp.HTTPClientSettings{
		Timeout: defaultTimeout,
		Auth: &configauth.Authentication{
			AuthenticatorID: component.NewID("sumologic"),
		},
	}
}

func (cfg *Config) Validate() error {

	switch cfg.LogFormat {
	case OTLPLogFormat:
	case JSONFormat:
	case TextFormat:
	default:
		return fmt.Errorf("unexpected log format: %s", cfg.LogFormat)
	}

	switch cfg.MetricFormat {
	case OTLPMetricFormat:
	case PrometheusFormat:
	case RemovedGraphiteFormat:
		return fmt.Errorf("support for the graphite metric format was removed, please use prometheus or otlp instead")
	case RemovedCarbon2Format:
		return fmt.Errorf("support for the carbon2 metric format was removed, please use prometheus or otlp instead")
	default:
		return fmt.Errorf("unexpected metric format: %s", cfg.MetricFormat)
	}

	switch cfg.TraceFormat {
	case OTLPTraceFormat:
	default:
		return fmt.Errorf("unexpected trace format: %s", cfg.TraceFormat)
	}

	if err := cfg.CompressEncoding.Validate(); err != nil {
		return err
	}

	if len(cfg.HTTPClientSettings.Endpoint) == 0 && cfg.HTTPClientSettings.Auth == nil {
		return errors.New("no endpoint and no auth extension specified")
	}

	if _, err := url.Parse(cfg.HTTPClientSettings.Endpoint); err != nil {
		return fmt.Errorf("failed parsing endpoint URL: %s; err: %w",
			cfg.HTTPClientSettings.Endpoint, err,
		)
	}

	if err := cfg.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	return nil
}

// LogFormatType represents log_format
type LogFormatType string

// MetricFormatType represents metric_format
type MetricFormatType string

// TraceFormatType represents trace_format
type TraceFormatType string

// PipelineType represents type of the pipeline
type PipelineType string

// CompressEncodingType represents type of the pipeline
type CompressEncodingType string

func (cet CompressEncodingType) Validate() error {
	switch cet {
	case GZIPCompression:
	case NoCompression:
	case DeflateCompression:

	default:
		return fmt.Errorf("invalid compression encoding type: %v", cet)
	}

	return nil
}

const (
	// TextFormat represents log_format: text
	TextFormat LogFormatType = "text"
	// JSONFormat represents log_format: json
	JSONFormat LogFormatType = "json"
	// OTLPLogFormat represents log_format: otlp
	OTLPLogFormat LogFormatType = "otlp"
	// RemovedGraphiteFormat represents the no longer supported graphite metric format
	RemovedGraphiteFormat MetricFormatType = "graphite"
	// RemovedCarbon2Format represents the no longer supported carbon2 metric format
	RemovedCarbon2Format MetricFormatType = "carbon2"
	// PrometheusFormat represents metric_format: prometheus
	PrometheusFormat MetricFormatType = "prometheus"
	// OTLPMetricFormat represents metric_format: otlp
	OTLPMetricFormat MetricFormatType = "otlp"
	// OTLPTraceFormat represents trace_format: otlp
	OTLPTraceFormat TraceFormatType = "otlp"
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
	// TracesPipeline represents traces pipeline
	TracesPipeline PipelineType = "traces"
	// defaultTimeout
	defaultTimeout time.Duration = 5 * time.Second
	// DefaultCompress defines default Compress
	DefaultCompress bool = true
	// DefaultCompressEncoding defines default CompressEncoding
	DefaultCompressEncoding CompressEncodingType = "gzip"
	// DefaultMaxRequestBodySize defines default MaxRequestBodySize in bytes
	DefaultMaxRequestBodySize int = 1 * 1024 * 1024
	// DefaultLogFormat defines default LogFormat
	DefaultLogFormat LogFormatType = OTLPLogFormat
	// DefaultMetricFormat defines default MetricFormat
	DefaultMetricFormat MetricFormatType = OTLPMetricFormat
	// DefaultClient defines default Client
	DefaultClient string = "otelcol"
	// DefaultClearTimestamp defines default ClearLogsTimestamp value
	DefaultClearLogsTimestamp bool = true
	// DefaultLogKey defines default LogKey value
	DefaultLogKey string = "log"
	// DefaultAddTimestamp defines default AddTimestamp value
	DefaultAddTimestamp bool = true
	// DefaultTimestampKey defines default TimestampKey value
	DefaultTimestampKey string = "timestamp"
	// DefaultFlattenBody defines default FlattenBody value
	DefaultFlattenBody bool = false
	// DefaultDropRoutingAttribute defines default DropRoutingAttribute
	DefaultDropRoutingAttribute string = ""
)
