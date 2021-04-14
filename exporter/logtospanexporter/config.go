package logtospanexporter

import (
	"fmt"

	"go.opentelemetry.io/collector/consumer/consumererror"
	otlp "go.opentelemetry.io/collector/exporter/otlpexporter"
)

// Config defines configuration for the Log to Span exporter.
type Config struct {
	// Config represents the OTLP Exporter configuration.
	otlp.Config `mapstructure:",squash"`

	// TraceType related configuration
	// Type of span that the log represents. (default w3c)
	//   * w3c - Logs contain W3C Trace Context 'traceparent' and 'tracestate' attributes
	TraceType TraceType `mapstructure:"trace_type"`

	// TimeFormat related configuration
	// The time format of 'span_start_time' and 'span_end_time'.
	//   * unix_epoch_micro - time format as UNIX Epoch time in microseconds
	//   * unix_epoch_nano - time format as UNIX Epoch time in nanoseconds
	TimeFormat TimeFormat `mapstructure:"time_format"`

	// FieldMap represents the fields that are mapped from the log to the outbound span.
	FieldMap FieldMap `mapstructure:"field_map"`
}

// FieldMap defines the log fields that map to span fields (case sensitive).
type FieldMap struct {
	// W3CTraceContextFields defines the log fields used for the W3C Trace Context
	W3CTraceContextFields W3CTraceContextFields `mapstructure:"w3c"`

	// ServiceName defines the name of the attribute containing the service name where the span originated.
	ServiceName string `mapstructure:"service_name"`

	// SpanName defines the name of attribute containing the name of the span.
	SpanName string `mapstructure:"span_name"`

	// SpanKind defines the name of attribute containing the span kind.
	SpanKind string `mapstructure:"span_kind"`

	// SpanStartTime defines the name of the attribute containing the start time of the span.
	SpanStartTime string `mapstructure:"span_start_time"`

	// SpanEndTime defines the name of the attribute containing the end time of the span.
	SpanEndTime string `mapstructure:"span_end_time"`

	// Ignored defines the names of attributes that will not be mapped to the span.
	Ignored []string `mapstructure:"ignored"`
}

// TraceType represents the type of span the log represents
type TraceType string

// TimeFormat represents the format of 'span_start_time' and 'span_end_time'
type TimeFormat string

// W3CTraceContextFields defines the log fields used for the W3C Trace Context
type W3CTraceContextFields struct {
	// Traceparent defines the name of the attribute containing the W3C 'traceparent' header value.
	Traceparent string `mapstructure:"traceparent"`

	// Tracestate defines the name of the attribute containing the W3C 'tracestate' header value.
	Tracestate string `mapstructure:"tracestate"`
}

func (c *Config) sanitize() error {
	var errs []error

	if len(c.Endpoint) == 0 {
		errs = append(errs, fmt.Errorf("\"endpoint\" must be set"))
	}

	switch c.TraceType {
	case W3CTraceType:
		if len(c.FieldMap.W3CTraceContextFields.Traceparent) == 0 {
			errs = append(errs, fmt.Errorf("\"field_map.w3c.traceparent\" must be set"))
		}

		if len(c.FieldMap.W3CTraceContextFields.Tracestate) == 0 {
			errs = append(errs, fmt.Errorf("\"field_map.w3c.tracestate\" must be set"))
		}
	default:
		errs = append(errs, fmt.Errorf("\"trace_type\" of '%s' is not supported", c.TraceType))
	}

	switch c.TimeFormat {
	case UnixEpochMicroTimeFormat:
	case UnixEpochNanoTimeFormat:
	default:
		errs = append(errs, fmt.Errorf("\"time_format\" of '%s' is not supported", c.TimeFormat))
	}

	if len(c.FieldMap.SpanName) == 0 {
		errs = append(errs, fmt.Errorf("\"field_map.span_name\" must be set"))
	}

	if len(c.FieldMap.SpanStartTime) == 0 {
		errs = append(errs, fmt.Errorf("\"field_map.span_start_time\" must be set"))
	}

	if len(c.FieldMap.SpanEndTime) == 0 {
		errs = append(errs, fmt.Errorf("\"field_map.span_end_time\" must be set"))
	}

	if len(errs) > 0 {
		return consumererror.Combine(errs)
	}

	return nil
}

const (
	// W3CTraceType represents the trace type of W3C Trace Context within the log
	W3CTraceType TraceType = "w3c"

	// UnixEpochMicroTimeFormat represents a time format as UNIX Epoch time in microseconds
	UnixEpochMicroTimeFormat = "unix_epoch_micro"

	// UnixEpochNanoTimeFormat represents a time format as UNIX Epoch time in nanoseconds
	UnixEpochNanoTimeFormat = "unix_epoch_nano"
)
