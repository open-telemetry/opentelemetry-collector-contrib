package kineticaotelexporter

import (
	"context"
	"fmt"
	"sync"

	"bitbucket.org/gisfederal/gpudb-api-go/kinetica"
	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// AttributeValue - struct to contain attribute values of different types
// Used by other metric structs
type AttributeValue struct {
	IntValue    int     `avro:"int_value"`
	StringValue string  `avro:"string_value"`
	BoolValue   int8    `avro:"bool_value"`
	DoubleValue float64 `avro:"double_value"`
	BytesValue  []byte  `avro:"bytes_value"`
}

// NewAttributeValue Constructor for AttributeValue
//
//	@param intValue
//	@param stringValue
//	@param boolValue
//	@param doubleValue
//	@param bytesValue
//	@return *AttributeValue
func NewAttributeValue(intValue int, stringValue string, boolValue int8, doubleValue float64, bytesValue []byte) *AttributeValue {
	o := new(AttributeValue)
	o.IntValue = intValue
	o.StringValue = stringValue
	o.BoolValue = boolValue
	o.DoubleValue = doubleValue
	o.BytesValue = bytesValue
	return o
}

// BEGIN Log Handling

// Log - struct for the Log data
type Log struct {
	LogID                string `mapstructure:"log_id" avro:"log_id"`
	TraceID              string `mapstructure:"trace_id" avro:"trace_id"`
	SpanID               string `mapstructure:"span_id" avro:"span_id"`
	TimeUnixNano         int64  `mapstructure:"time_unix_nano" avro:"time_unix_nano"`
	ObservedTimeUnixNano int64  `mapstructure:"observed_time_unix_nano" avro:"observed_time_unix_nano"`
	SeverityID           int8   `mapstructure:"severity_id" avro:"severity_id"`
	SeverityText         string `mapstructure:"severity_text" avro:"severity_text"`
	Body                 string `mapstructure:"body" avro:"body"`
	Flags                int    `mapstructure:"flags" avro:"flags"`
}

// NewLog Constructor for Logs
//
//	@param LogID
//	@param ResourceID
//	@param ScopeID
//	@param TraceID
//	@param SpanID
//	@param TimeUnixNano
//	@param ObservedTimeUnixNano
//	@param SeverityID
//	@param SeverityText
//	@param Body
//	@param Flags
//	@return *Logs
func NewLog(LogID string, TraceID string, SpanID string, TimeUnixNano int64, ObservedTimeUnixNano int64, SeverityID int8, SeverityText string, Body string, Flags int) *Log {
	o := new(Log)

	o.LogID = LogID
	o.TraceID = TraceID
	o.SpanID = SpanID
	o.TimeUnixNano = TimeUnixNano
	o.ObservedTimeUnixNano = ObservedTimeUnixNano
	o.SeverityID = SeverityID
	o.SeverityText = SeverityText
	o.Body = Body
	o.Flags = Flags

	return o
}

// LogAttribute - struct to contain the Log specific attributes
// contains '[AttributeValue]' struct
type LogAttribute struct {
	LogID          string `avro:"log_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewLogAttribute Constructor for LogsAttribute
//
//	@param key
//	@param attributes
//	@return *LogsAttribute
func NewLogAttribute(logID string, key string, attributes AttributeValue) *LogAttribute {
	o := new(LogAttribute)
	o.LogID = logID
	o.Key = key
	o.AttributeValue = attributes
	return o
}

// END LogAttribute

// ResourceAttribute - struct containing the Log Resource attributes
type ResourceAttribute struct {
	LogID          string `avro:"log_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewResourceAttribute Constructor for LogsResourceAttribute
//
//	@param resourceID
//	@param key
//	@param attributes
//	@return *LogsResourceAttribute
func NewResourceAttribute(logID string, key string, attributes AttributeValue) *ResourceAttribute {
	o := new(ResourceAttribute)
	o.LogID = logID
	o.Key = key
	o.AttributeValue = attributes
	return o
}

// End LogResourceAttribute

// ScopeAttribute - struct containing the Log Scope attribute
type ScopeAttribute struct {
	LogID          string `avro:"log_id"`
	ScopeName      string `avro:"scope_name"`
	ScopeVersion   string `avro:"scope_version"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewScopeAttribute Constructor for LogsScopeAttribute
//
//	@param scopeID
//	@param key
//	@param scopeName
//	@param scopeVersion
//	@param attributes
//	@return *LogsScopeAttribute
func NewScopeAttribute(logID string, key string, scopeName string, scopeVersion string, attributes AttributeValue) *ScopeAttribute {
	o := new(ScopeAttribute)
	o.LogID = uuid.New().String()
	o.Key = key
	o.ScopeName = scopeName
	o.ScopeVersion = scopeVersion
	o.AttributeValue = attributes
	return o
}

// END LogScopeAttribute

// KiWriter - struct modeling the Kinetica connection, contains the
// Kinetica connection [kinetica.Kinetica], the Kinetica Options [kinetica.KineticaOptions],
// the config [Config] and the logger [zap.Logger]
type KiWriter struct {
	Db      kinetica.Kinetica
	Options kinetica.KineticaOptions
	cfg     Config
	logger  *zap.Logger
}

// GetDb - Getter for the Kinetica instance
//
//	@receiver kiwriter
//	@return gpudb.Gpudb
func (kiwriter *KiWriter) GetDb() kinetica.Kinetica {
	return kiwriter.Db
}

// GetOptions - Getter for the Kinetica options.
//
//	@receiver kiwriter
//	@return gpudb.GpudbOptions
func (kiwriter *KiWriter) GetOptions() kinetica.KineticaOptions {
	return kiwriter.Options
}

// GetCfg - Getter for the [Config] value
//
//	@receiver kiwriter
//	@return Config
func (kiwriter *KiWriter) GetCfg() Config {
	return kiwriter.cfg
}

// Writer - global pointer to kiwriter struct initialized in the init func
var Writer *KiWriter

// init
func init() {
	ctx := context.TODO()
	cfg := CreateDefaultConfig()
	config := cfg.(*Config)
	options := kinetica.KineticaOptions{Username: config.Username, Password: string(config.Password), ByPassSslCertCheck: config.BypassSslCertCheck}
	gpudbInst := kinetica.NewWithOptions(ctx, config.Host, &options)
	Writer = &KiWriter{*gpudbInst, options, *config, nil}
}

// NewKiWriter - Constructor for the [KiWriter] struct
//
//	@param ctx
//	@param cfg
//	@return *KiWriter
func NewKiWriter(ctx context.Context, cfg Config, logger *zap.Logger) *KiWriter {
	options := kinetica.KineticaOptions{Username: cfg.Username, Password: string(cfg.Password), ByPassSslCertCheck: cfg.BypassSslCertCheck}
	gpudbInst := kinetica.NewWithOptions(ctx, cfg.Host, &options)
	return &KiWriter{*gpudbInst, options, cfg, logger}
}

// GetGpuDbInst - Creates and returns a new [kinetica.Kinetica] struct
//
//	@param cfg
//	@return *gpudb.Gpudb
func GetGpuDbInst(cfg *Config) *kinetica.Kinetica {
	ctx := context.TODO()
	options := kinetica.KineticaOptions{Username: cfg.Username, Password: string(cfg.Password), ByPassSslCertCheck: cfg.BypassSslCertCheck}
	// fmt.Println("Options", options)
	gpudbInst := kinetica.NewWithOptions(ctx, cfg.Host, &options)

	return gpudbInst

}

// END Log Handling

// BEGIN Trace Handling

// Span - struct modeling the Span record
type Span struct {
	ID                     string `mapstructure:"id" avro:"id" `
	TraceID                string `mapstructure:"trace_id" avro:"trace_id"`
	SpanID                 string `mapstructure:"span_id" avro:"span_id"`
	ParentSpanID           string `mapstructure:"parent_span_id" avro:"parent_span_id"`
	TraceState             string `mapstructure:"trace_state" avro:"trace_state"`
	Name                   string `mapstructure:"name" avro:"name"`
	SpanKind               int8   `mapstructure:"span_kind" avro:"span_kind"`
	StartTimeUnixNano      int64  `mapstructure:"start_time_unix_nano" avro:"start_time_unix_nano"`
	EndTimeUnixNano        int64  `mapstructure:"end_time_unix_nano" avro:"end_time_unix_nano"`
	DroppedAttributesCount int    `mapstructure:"dropped_attributes_count" avro:"dropped_attributes_count"`
	DroppedEventsCount     int    `mapstructure:"dropped_events_count" avro:"dropped_events_count"`
	DroppedLinksCount      int    `mapstructure:"dropped_links_count" avro:"dropped_links_count"`
	Message                string `mapstructure:"message" avro:"message"`
	StatusCode             int8   `mapstructure:"status_code" avro:"status_code"`
}

// NewSpan Constructor for Span
//
//	@param resourceID
//	@param scopeID
//	@param eventID
//	@param linkID
//	@param traceID
//	@param spanID
//	@param parentSpanID
//	@param traceState
//	@param name
//	@param spanKind
//	@param startTimeUnixNano
//	@param endTimeUnixNano
//	@param droppedAttributeCount
//	@param droppedEventCount
//	@param droppedLinkCount
//	@param message
//	@param statusCode
//	@return *Span
func NewSpan(traceID string, spanID string, parentSpanID string, traceState string, name string, spanKind int8, startTimeUnixNano int64, endTimeUnixNano int64, droppedAttributeCount int, droppedEventCount int, droppedLinkCount int, message string, statusCode int8) *Span {
	o := new(Span)
	o.ID = uuid.New().String()
	o.TraceID = traceID
	o.SpanID = spanID
	o.ParentSpanID = parentSpanID
	o.TraceState = traceState
	o.Name = name
	o.SpanKind = spanKind
	o.StartTimeUnixNano = startTimeUnixNano
	o.EndTimeUnixNano = endTimeUnixNano
	o.DroppedAttributesCount = droppedAttributeCount
	o.DroppedEventsCount = droppedEventCount
	o.DroppedLinksCount = droppedLinkCount
	o.Message = message
	o.StatusCode = statusCode
	return o
}

// SpanAttribute - struct modeling the Trace Span attributes
//
// Contains the [AttributeValue] struct
type SpanAttribute struct {
	SpanID         string `avro:"span_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewSpanAttribute Constructor for SpanAttribute
//
//	@param spanID
//	@param key
//	@param atributeValue
//	@return *SpanAttribute
func NewSpanAttribute(spanID string, key string, attributeValue AttributeValue) *SpanAttribute {
	o := new(SpanAttribute)
	o.SpanID = spanID
	o.Key = key
	o.AttributeValue = attributeValue
	return o
}

// TraceResourceAttribute - struct modelig the Trace Resource attributes
type TraceResourceAttribute struct {
	SpanID         string `avro:"span_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewTraceResourceAttribute Constructor for TraceResourceAttribute
//
//	@param SpanID
//	@param key
//	@param attributes
//	@return *TraceResourceAttribute
func NewTraceResourceAttribute(SpanID string, key string, attributes AttributeValue) *TraceResourceAttribute {
	o := new(TraceResourceAttribute)
	o.SpanID = SpanID
	o.Key = key
	o.AttributeValue = attributes
	return o
}

// End TraceResourceAttribute

// TraceScopeAttribute - struct modeling the Trace Scope attribute
type TraceScopeAttribute struct {
	SpanID         string `avro:"span_id"`
	ScopeName      string `avro:"scope_name"`
	ScopeVersion   string `avro:"scope_version"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewtraceScopeAttribute Constructor for TraceScopeAttribute
//
//	@param key
//	@param scopeName
//	@param scopeVersion
//	@param attributes
//	@return *TraceScopeAttribute
func NewtraceScopeAttribute(SpanID string, key string, scopeName string, scopeVersion string, attributes AttributeValue) *TraceScopeAttribute {
	o := new(TraceScopeAttribute)
	o.SpanID = SpanID
	o.Key = key
	o.ScopeName = scopeName
	o.ScopeVersion = scopeVersion
	o.AttributeValue = attributes
	return o
}

// END TraceScopeAttribute

// EventAttribute - struct modeling the Trace Event attributes
type EventAttribute struct {
	SpanID         string `avro:"span_id"`
	EventName      string `avro:"event_name"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewEventAttribute Constructor for TraceEventAttribute
//
//	@param key
//	@param eventName
//	@param attributes
//	@return *TraceEventAttribute
func NewEventAttribute(spanID string, eventName string, key string, attributes AttributeValue) *EventAttribute {
	o := new(EventAttribute)
	o.SpanID = spanID
	o.Key = key
	o.EventName = eventName
	o.AttributeValue = attributes
	return o
}

// END TraceEventAttribute

// LinkAttribute - struct modeling the Trace Link attributes
type LinkAttribute struct {
	LinkSpanID     string `avro:"link_span_id"`
	TraceID        string `avro:"trace_id"`
	SpanID         string `avro:"span_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// NewLinkAttribute Constructor for LinkAttribute
//
//	@param linkID
//	@param key
//	@param traceID
//	@param spanID
//	@param attributes
//	@return *LinkAttribute
func NewLinkAttribute(linkSpanID string, key string, traceID string, spanID string, attributes AttributeValue) *LinkAttribute {
	o := new(LinkAttribute)
	o.LinkSpanID = linkSpanID
	o.Key = key
	o.TraceID = traceID
	o.SpanID = spanID
	o.AttributeValue = attributes
	return o
}

// END LinkAttribute

// END Trace Handling

// Metrics Handling

// Gauge - struct modeling the Gauge data
type Gauge struct {
	GaugeID     string `avro:"gauge_id"`
	MetricName  string `avro:"metric_name"`
	Description string `avro:"metric_description"`
	Unit        string `avro:"metric_unit"`
}

// GaugeDatapoint - struct modeling the Gauge Datapoint
type GaugeDatapoint struct {
	GaugeID       string  `avro:"gauge_id"`
	ID            string  `avro:"id"`
	StartTimeUnix int64   `mapstructure:"start_time_unix" avro:"start_time_unix"`
	TimeUnix      int64   `mapstructure:"time_unix" avro:"time_unix"`
	GaugeValue    float64 `mapstructure:"gauge_value" avro:"gauge_value"`
	Flags         int     `mapstructure:"flags" avro:"flags"`
}

// GaugeDatapointAttribute - struct modeling the Gauge Datapoint attributes
type GaugeDatapointAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// GaugeDatapointExemplar - struct modeling a Gauge Datapoint Exemplar
type GaugeDatapointExemplar struct {
	GaugeID     string  `avro:"gauge_id"`
	DatapointID string  `avro:"datapoint_id"`
	ExemplarID  string  `avro:"exemplar_id"`
	TimeUnix    int64   `mapstructure:"time_unix" avro:"time_unix"`
	GaugeValue  float64 `mapstructure:"gauge_value" avro:"gauge_value"`
	TraceID     string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID      string  `mapstructure:"span_id" avro:"span_id"`
}

// GaugeDataPointExemplarAttribute - struct modeling a Gauge Datapoint Exemplar attribute
type GaugeDataPointExemplarAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// GaugeResourceAttribute - struct modeling a Gauge resource attribute
type GaugeResourceAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// GaugeScopeAttribute - struct modeling a Gauge Scope attribute
type GaugeScopeAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// END Gauge

// Sum

// Sum - struct modeling a Sum metric
type Sum struct {
	SumID                  string `avro:"sum_id"`
	MetricName             string `avro:"metric_name"`
	Description            string `avro:"metric_description"`
	Unit                   string `avro:"metric_unit"`
	AggregationTemporality int8   `avro:"aggregation_temporality"`
	IsMonotonic            int8   `avro:"is_monotonic"`
}

// SumDatapoint - struct modeling a Sum Datapoint
type SumDatapoint struct {
	SumID         string  `avro:"sum_id"`
	ID            string  `avro:"id"`
	StartTimeUnix int64   `mapstructure:"start_time_unix" avro:"start_time_unix"`
	TimeUnix      int64   `mapstructure:"time_unix" avro:"time_unix"`
	SumValue      float64 `mapstructure:"sum_value" avro:"sum_value"`
	Flags         int     `mapstructure:"flags" avro:"flags"`
}

// SumDataPointAttribute - struct modeling a Sum Datapoint attribute
type SumDataPointAttribute struct {
	SumID          string `avro:"sum_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// SumDatapointExemplar - struct modeling a Sum Datapoint Exemplar
type SumDatapointExemplar struct {
	SumID       string  `avro:"sum_id"`
	DatapointID string  `avro:"datapoint_id"`
	ExemplarID  string  `avro:"exemplar_id"`
	TimeUnix    int64   `mapstructure:"time_unix" avro:"time_unix"`
	SumValue    float64 `mapstructure:"sum_value" avro:"sum_value"`
	TraceID     string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID      string  `mapstructure:"span_id" avro:"span_id"`
}

// SumDataPointExemplarAttribute  - struct modeling a Sum Datapoint Exemplar attribute
type SumDataPointExemplarAttribute struct {
	SumID          string `avro:"sum_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// SumResourceAttribute - struct modeling a Sum Resource attribute
type SumResourceAttribute struct {
	SumID          string `avro:"sum_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// SumScopeAttribute - struct modeling a Sum Scope attribute
type SumScopeAttribute struct {
	SumID          string `avro:"sum_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// END Sum

// Histogram

// Histogram - struct modeling a Histogram metric type
type Histogram struct {
	HistogramID            string `avro:"histogram_id"`
	MetricName             string `avro:"metric_name"`
	Description            string `avro:"metric_description"`
	Unit                   string `avro:"metric_unit"`
	AggregationTemporality int8   `avro:"aggregation_temporality"`
}

// HistogramDatapoint - struct modeling a Histogram Datapoint
type HistogramDatapoint struct {
	HistogramID   string  `avro:"histogram_id"`
	ID            string  `avro:"id"`
	StartTimeUnix int64   `avro:"start_time_unix"`
	TimeUnix      int64   `avro:"time_unix"`
	Count         int64   `avro:"count"`
	Sum           float64 `avro:"data_sum"`
	Min           float64 `avro:"data_min"`
	Max           float64 `avro:"data_max"`
	Flags         int     `avro:"flags"`
}

// HistogramDataPointAttribute - struct modeling a Histogram Datapoint attribute
type HistogramDataPointAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// HistogramDatapointBucketCount - struct modeling a Histogram Datapoint Bucket Count
type HistogramDatapointBucketCount struct {
	HistogramID string `avro:"histogram_id"`
	DatapointID string `avro:"datapoint_id"`
	CountID     string `avro:"count_id"`
	Count       int64  `avro:"count"`
}

// HistogramDatapointExplicitBound - struct modeling a Histogram Datapoint Explicit Bound
type HistogramDatapointExplicitBound struct {
	HistogramID   string  `avro:"histogram_id"`
	DatapointID   string  `avro:"datapoint_id"`
	BoundID       string  `avro:"bound_id"`
	ExplicitBound float64 `avro:"explicit_bound"`
}

// HistogramDatapointExemplar - struct modeling a Histogram Datapoint Exemplar
type HistogramDatapointExemplar struct {
	HistogramID    string  `avro:"histogram_id"`
	DatapointID    string  `avro:"datapoint_id"`
	ExemplarID     string  `avro:"exemplar_id"`
	TimeUnix       int64   `avro:"time_unix"`
	HistogramValue float64 `avro:"histogram_value"`
	TraceID        string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID         string  `mapstructure:"span_id" avro:"span_id"`
}

// HistogramDataPointExemplarAttribute - struct modeling a Histogram Datapoint Exemplar attribute
type HistogramDataPointExemplarAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// HistogramResourceAttribute - struct modeling a Histogram Resource Attribute
type HistogramResourceAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// HistogramScopeAttribute - struct modeling a Histogram Scope Attribute
type HistogramScopeAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// End Histogram

// Exponential Histogram

// ExponentialHistogram - struct modeling an Exponential Histogram
type ExponentialHistogram struct {
	HistogramID            string `avro:"histogram_id"`
	MetricName             string `avro:"metric_name"`
	Description            string `avro:"metric_description"`
	Unit                   string `avro:"metric_unit"`
	AggregationTemporality int8   `avro:"aggregation_temporality"`
}

// ExponentialHistogramDatapoint - struct modeling an Exponential Histogram Datapoint
type ExponentialHistogramDatapoint struct {
	HistogramID           string  `avro:"histogram_id"`
	ID                    string  `avro:"id"`
	StartTimeUnix         int64   `avro:"start_time_unix"`
	TimeUnix              int64   `avro:"time_unix"`
	Count                 int64   `avro:"count"`
	Sum                   float64 `avro:"data_sum"`
	Min                   float64 `avro:"data_min"`
	Max                   float64 `avro:"data_max"`
	Flags                 int     `avro:"flags"`
	Scale                 int     `avro:"scale"`
	ZeroCount             int64   `avro:"zero_count"`
	BucketsPositiveOffset int     `avro:"buckets_positive_offset"`
	BucketsNegativeOffset int     `avro:"buckets_negative_offset"`
}

// ExponentialHistogramDataPointAttribute - struct modeling an Exponential Histogram Datapoint attribute
type ExponentialHistogramDataPointAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// ExponentialHistogramBucketNegativeCount - struct modeling an Exponential Histogram Bucket Negative Count
type ExponentialHistogramBucketNegativeCount struct {
	HistogramID string `avro:"histogram_id"`
	DatapointID string `avro:"datapoint_id"`
	CountID     string `avro:"count_id"`
	Count       uint64 `avro:"count"`
}

// ExponentialHistogramBucketPositiveCount - struct modeling an Exponential Histogram Bucket Positive Count
type ExponentialHistogramBucketPositiveCount struct {
	HistogramID string `avro:"histogram_id"`
	DatapointID string `avro:"datapoint_id"`
	CountID     string `avro:"count_id"`
	Count       int64  `avro:"count"`
}

// ExponentialHistogramDatapointExemplar - struct modeling an Exponential Histogram Datapoint Exemplar
type ExponentialHistogramDatapointExemplar struct {
	HistogramID    string  `avro:"histogram_id"`
	DatapointID    string  `avro:"datapoint_id"`
	ExemplarID     string  `avro:"exemplar_id"`
	TimeUnix       int64   `avro:"time_unix"`
	HistogramValue float64 `avro:"histogram_value"`
	TraceID        string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID         string  `mapstructure:"span_id" avro:"span_id"`
}

// ExponentialHistogramDataPointExemplarAttribute - struct modeling an Exponential Histogram Datapoint Exemplar attribute
type ExponentialHistogramDataPointExemplarAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// ExponentialHistogramResourceAttribute - struct modeling an Exponential Histogram Resource attribute
type ExponentialHistogramResourceAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// ExponentialHistogramScopeAttribute - struct modeling an Exponential Histogram Scope attribute
type ExponentialHistogramScopeAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// END Exponential Histogram

// Summary

// Summary - struct modeling a Summary type metric
type Summary struct {
	SummaryID   string `avro:"summary_id"`
	MetricName  string `avro:"metric_name"`
	Description string `avro:"metric_description"`
	Unit        string `avro:"metric_unit"`
}

// SummaryDatapoint - struct modeling a Summary Datapoint
type SummaryDatapoint struct {
	SummaryID     string  `avro:"summary_id"`
	ID            string  `avro:"id"`
	StartTimeUnix int64   `avro:"start_time_unix"`
	TimeUnix      int64   `avro:"time_unix"`
	Count         int64   `avro:"count"`
	Sum           float64 `avro:"data_sum"`
	Flags         int     `avro:"flags"`
}

// SummaryDataPointAttribute - struct modeling a Summary Datapoint attribute
type SummaryDataPointAttribute struct {
	SummaryID      string `avro:"summary_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// SummaryDatapointQuantileValues - struct modeling a Summary Datapoint Quantile value
type SummaryDatapointQuantileValues struct {
	SummaryID   string  `avro:"summary_id"`
	DatapointID string  `avro:"datapoint_id"`
	QuantileID  string  `avro:"quantile_id"`
	Quantile    float64 `avro:"quantile"`
	Value       float64 `avro:"value"`
}

// SummaryResourceAttribute - struct modeling a Summary Resource attribute
type SummaryResourceAttribute struct {
	SummaryID      string `avro:"summary_id"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// SummaryScopeAttribute - struct modeling a Summary Scope attribute
type SummaryScopeAttribute struct {
	SummaryID      string `avro:"summary_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	AttributeValue `mapstructure:",squash"`
}

// END Summary

// END Metrics Handling

// persistLogRecord - method to write Log records into Kinetica
//
//	@receiver kiwriter
//	@param logRecords
//	@return error
func (kiwriter *KiWriter) persistLogRecord(logRecords []kineticaLogRecord) error {

	var errs []error
	var logs []any
	var logAttribs []interface{}
	var resourceAttribs []interface{}
	var scopeAttribs []interface{}

	for _, logrecord := range logRecords {

		// For each chunk of 10K records persist eveything
		logs = append(logs, *logrecord.log)

		for _, la := range logrecord.logAttribute {
			logAttribs = append(logAttribs, la)
		}

		for _, ra := range logrecord.resourceAttribute {
			resourceAttribs = append(resourceAttribs, ra)
		}

		for _, sa := range logrecord.scopeAttribute {
			scopeAttribs = append(scopeAttribs, sa)
		}
	}

	err := kiwriter.doChunkedInsert(context.TODO(), LogTable, logs)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), LogAttributeTable, logAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), LogResourceAttributeTable, resourceAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), LogScopeAttributeTable, scopeAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	return multierr.Combine(errs...)
}

// persistTraceRecord - method to write Trace records into Kinetica
//
//	@receiver kiwriter
//	@param traceRecords
//	@return error
func (kiwriter *KiWriter) persistTraceRecord(traceRecords []kineticaTraceRecord) error {
	var errs []error
	var spans []any
	var spanAttribs []interface{}
	var spanResourceAttribs []interface{}
	var spanScopeAttribs []interface{}
	var spanEventAttribs []interface{}
	var spanLinkAttribs []interface{}

	for _, tracerecord := range traceRecords {

		spans = append(spans, *tracerecord.span)

		for _, spa := range tracerecord.spanAttribute {
			spanAttribs = append(spanAttribs, spa)
		}

		for _, ra := range tracerecord.resourceAttribute {
			spanResourceAttribs = append(spanResourceAttribs, ra)
		}

		for _, sa := range tracerecord.scopeAttribute {
			spanScopeAttribs = append(spanScopeAttribs, sa)
		}

		for _, ea := range tracerecord.eventAttribute {
			spanEventAttribs = append(spanEventAttribs, ea)
		}

		for _, la := range tracerecord.linkAttribute {
			spanLinkAttribs = append(spanLinkAttribs, la)
		}
	}

	err := kiwriter.doChunkedInsert(context.TODO(), TraceSpanTable, spans)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), TraceSpanAttributeTable, spanAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), TraceResourceAttributeTable, spanResourceAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), TraceScopeAttributeTable, spanScopeAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), TraceEventAttributeTable, spanEventAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	err = kiwriter.doChunkedInsert(context.TODO(), TraceLinkAttributeTable, spanLinkAttribs)
	if err != nil {
		errs = append(errs, err)
	}

	return multierr.Combine(errs...)
}

// writeMetric - a helper method used by different metric persistence methods to write the
// metric data in order.
//
//	@receiver kiwriter - pointer to [KiWriter]
//	@param metricType - a [pmetric.MetricTypeGauge] or something else converted to string
//	@param tableDataMap - a map from table name to the relevant data
//	@return error
func (kiwriter *KiWriter) writeMetric(metricType string, tableDataMap *orderedmap.OrderedMap) error {

	kiwriter.logger.Debug("Writing metric", zap.String("Type", metricType))

	var errs []error
	errsChan := make(chan error, tableDataMap.Len())

	wg := &sync.WaitGroup{}
	for pair := tableDataMap.Oldest(); pair != nil; pair = pair.Next() {
		tableName := pair.Key.(string)
		data := pair.Value.([]any)

		wg.Add(1)

		go func(tableName string, data []any, wg *sync.WaitGroup) {
			err := kiwriter.doChunkedInsert(context.TODO(), tableName, data)
			if err != nil {
				errsChan <- err
			}
			wg.Done()
		}(tableName, data, wg)

	}
	wg.Wait()

	close(errsChan)

	var insErrs error
	for err := range errsChan {
		insErrs = multierr.Append(insErrs, err)
	}
	errs = append(errs, insErrs)
	return multierr.Combine(errs...)
}

func (kiwriter *KiWriter) persistGaugeRecord(gaugeRecords []kineticaGaugeRecord) error {
	kiwriter.logger.Debug("In persistGaugeRecord ...")

	var errs []error
	var gauges []any
	var resourceAttributes []any
	var scopeAttributes []any
	var datapoints []any
	var datapointAttributes []any
	var exemplars []any
	var exemplarAttributes []any

	for _, gaugerecord := range gaugeRecords {

		gauges = append(gauges, *gaugerecord.gauge)

		for _, gr := range gaugerecord.resourceAttribute {
			resourceAttributes = append(resourceAttributes, gr)
		}

		for _, sa := range gaugerecord.scopeAttribute {
			scopeAttributes = append(scopeAttributes, sa)
		}

		for _, dp := range gaugerecord.datapoint {
			datapoints = append(datapoints, dp)
		}

		for _, dpattr := range gaugerecord.datapointAttribute {
			datapointAttributes = append(datapointAttributes, dpattr)
		}

		for _, ge := range gaugerecord.exemplars {
			exemplars = append(exemplars, ge)
		}

		for _, geattr := range gaugerecord.exemplarAttribute {
			exemplarAttributes = append(exemplarAttributes, geattr)
		}

	}

	tableDataMap := orderedmap.New()

	tableDataMap.Set(GaugeTable, gauges)
	tableDataMap.Set(GaugeDatapointTable, datapoints)
	tableDataMap.Set(GaugeDatapointAttributeTable, datapointAttributes)
	tableDataMap.Set(GaugeResourceAttributeTable, resourceAttributes)
	tableDataMap.Set(GaugeScopeAttributeTable, scopeAttributes)
	tableDataMap.Set(GaugeDatapointExemplarTable, exemplars)
	tableDataMap.Set(GaugeDatapointExemplarAttributeTable, exemplarAttributes)

	errs = append(errs, kiwriter.writeMetric(pmetric.MetricTypeGauge.String(), tableDataMap))

	return multierr.Combine(errs...)
}

func (kiwriter *KiWriter) persistSumRecord(sumRecords []kineticaSumRecord) error {
	kiwriter.logger.Debug("In persistSumRecord ...")

	var errs []error

	var sums []any
	var resourceAttributes []any
	var scopeAttributes []any
	var datapoints []any
	var datapointAttributes []any
	var exemplars []any
	var exemplarAttributes []any

	for _, sumrecord := range sumRecords {

		sums = append(sums, *sumrecord.sum)

		for _, sr := range sumrecord.sumResourceAttribute {
			resourceAttributes = append(resourceAttributes, sr)
		}

		for _, sa := range sumrecord.sumScopeAttribute {
			scopeAttributes = append(scopeAttributes, sa)
		}

		for _, dp := range sumrecord.datapoint {
			datapoints = append(datapoints, dp)
		}

		for _, dpattr := range sumrecord.datapointAttribute {
			datapointAttributes = append(datapointAttributes, dpattr)
		}

		for _, se := range sumrecord.exemplars {
			exemplars = append(exemplars, se)
		}

		for _, seattr := range sumrecord.exemplarAttribute {
			exemplarAttributes = append(exemplarAttributes, seattr)
		}

	}

	tableDataMap := orderedmap.New()

	tableDataMap.Set(SumTable, sums)
	tableDataMap.Set(SumDatapointTable, datapoints)
	tableDataMap.Set(SumDatapointAttributeTable, datapointAttributes)
	tableDataMap.Set(SumResourceAttributeTable, resourceAttributes)
	tableDataMap.Set(SumScopeAttributeTable, scopeAttributes)
	tableDataMap.Set(SumDatapointExemplarTable, exemplars)
	tableDataMap.Set(SumDataPointExemplarAttributeTable, exemplarAttributes)

	errs = append(errs, kiwriter.writeMetric(pmetric.MetricTypeSum.String(), tableDataMap))

	return multierr.Combine(errs...)
}

func (kiwriter *KiWriter) persistHistogramRecord(histogramRecords []kineticaHistogramRecord) error {
	kiwriter.logger.Debug("In persistHistogramRecord ...")

	var errs []error

	var histograms []any
	var resourceAttributes []any
	var scopeAttributes []any
	var datapoints []any
	var datapointAttributes []any
	var bucketCounts []any
	var explicitBounds []any
	var exemplars []any
	var exemplarAttributes []any

	for _, histogramrecord := range histogramRecords {

		histograms = append(histograms, *histogramrecord.histogram)

		for _, ra := range histogramrecord.histogramResourceAttribute {
			resourceAttributes = append(resourceAttributes, ra)
		}

		for _, sa := range histogramrecord.histogramScopeAttribute {
			scopeAttributes = append(scopeAttributes, sa)
		}

		for _, dp := range histogramrecord.histogramDatapoint {
			datapoints = append(datapoints, dp)
		}

		for _, dpattr := range histogramrecord.histogramDatapointAtribute {
			datapointAttributes = append(datapointAttributes, dpattr)
		}

		for _, bc := range histogramrecord.histogramBucketCount {
			bucketCounts = append(bucketCounts, bc)
		}

		for _, eb := range histogramrecord.histogramExplicitBound {
			explicitBounds = append(explicitBounds, eb)
		}

		for _, ex := range histogramrecord.exemplars {
			exemplars = append(exemplars, ex)
		}

		for _, exattr := range histogramrecord.exemplarAttribute {
			exemplarAttributes = append(exemplarAttributes, exattr)
		}
	}

	tableDataMap := orderedmap.New()

	tableDataMap.Set(HistogramTable, histograms)
	tableDataMap.Set(HistogramDatapointTable, datapoints)
	tableDataMap.Set(HistogramDatapointAttributeTable, datapointAttributes)
	tableDataMap.Set(HistogramBucketCountsTable, bucketCounts)
	tableDataMap.Set(HistogramExplicitBoundsTable, explicitBounds)
	tableDataMap.Set(HistogramResourceAttributeTable, resourceAttributes)
	tableDataMap.Set(HistogramScopeAttributeTable, scopeAttributes)
	tableDataMap.Set(HistogramDatapointExemplarTable, exemplars)
	tableDataMap.Set(HistogramDataPointExemplarAttributeTable, exemplarAttributes)

	errs = append(errs, kiwriter.writeMetric(pmetric.MetricTypeHistogram.String(), tableDataMap))

	return multierr.Combine(errs...)
}

func (kiwriter *KiWriter) persistExponentialHistogramRecord(exponentialHistogramRecords []kineticaExponentialHistogramRecord) error {
	kiwriter.logger.Debug("In persistExponentialHistogramRecord ...")

	var errs []error

	var histograms []any
	var resourceAttributes []any
	var scopeAttributes []any
	var datapoints []any
	var datapointAttributes []any
	var positiveBucketCounts []any
	var negativeBucketCounts []any
	var exemplars []any
	var exemplarAttributes []any

	for _, histogramrecord := range exponentialHistogramRecords {

		histograms = append(histograms, *histogramrecord.histogram)

		for _, ra := range histogramrecord.histogramResourceAttribute {
			resourceAttributes = append(resourceAttributes, ra)
		}

		for _, sa := range histogramrecord.histogramScopeAttribute {
			scopeAttributes = append(scopeAttributes, sa)
		}

		for _, dp := range histogramrecord.histogramDatapoint {
			datapoints = append(datapoints, dp)
		}

		for _, dpattr := range histogramrecord.histogramDatapointAttribute {
			datapointAttributes = append(datapointAttributes, dpattr)
		}

		for _, posbc := range histogramrecord.histogramBucketPositiveCount {
			positiveBucketCounts = append(positiveBucketCounts, posbc)
		}

		for _, negbc := range histogramrecord.histogramBucketNegativeCount {
			negativeBucketCounts = append(negativeBucketCounts, negbc)
		}

		for _, ex := range histogramrecord.exemplars {
			exemplars = append(exemplars, ex)
		}

		for _, exattr := range histogramrecord.exemplarAttribute {
			exemplarAttributes = append(exemplarAttributes, exattr)
		}
	}

	tableDataMap := orderedmap.New()

	tableDataMap.Set(ExpHistogramTable, histograms)
	tableDataMap.Set(ExpHistogramDatapointTable, datapoints)
	tableDataMap.Set(ExpHistogramDatapointAttributeTable, datapointAttributes)
	tableDataMap.Set(ExpHistogramPositiveBucketCountsTable, positiveBucketCounts)
	tableDataMap.Set(ExpHistogramNegativeBucketCountsTable, negativeBucketCounts)
	tableDataMap.Set(ExpHistogramResourceAttributeTable, resourceAttributes)
	tableDataMap.Set(ExpHistogramScopeAttributeTable, scopeAttributes)
	tableDataMap.Set(ExpHistogramDatapointExemplarTable, exemplars)
	tableDataMap.Set(ExpHistogramDataPointExemplarAttributeTable, exemplarAttributes)

	errs = append(errs, kiwriter.writeMetric(pmetric.MetricTypeExponentialHistogram.String(), tableDataMap))

	return multierr.Combine(errs...)
}

func (kiwriter *KiWriter) persistSummaryRecord(summaryRecords []kineticaSummaryRecord) error {
	kiwriter.logger.Debug("In persistSummaryRecord ...")

	var errs []error

	var summaries []any
	var resourceAttributes []any
	var scopeAttributes []any
	var datapoints []any
	var datapointAttributes []any
	var datapointQuantiles []any

	for _, summaryrecord := range summaryRecords {

		summaries = append(summaries, *summaryrecord.summary)

		for _, ra := range summaryrecord.summaryResourceAttribute {
			resourceAttributes = append(resourceAttributes, ra)
		}

		for _, sa := range summaryrecord.summaryScopeAttribute {
			scopeAttributes = append(scopeAttributes, sa)
		}

		for _, dp := range summaryrecord.summaryDatapoint {
			datapoints = append(datapoints, dp)
		}

		for _, dpattr := range summaryrecord.summaryDatapointAttribute {
			datapointAttributes = append(datapointAttributes, dpattr)
		}

		for _, dpq := range summaryrecord.summaryDatapointQuantileValues {
			datapointQuantiles = append(datapointQuantiles, dpq)
		}
	}

	tableDataMap := orderedmap.New()

	tableDataMap.Set(SummaryTable, summaries)
	tableDataMap.Set(SummaryDatapointTable, datapoints)
	tableDataMap.Set(SummaryDatapointAttributeTable, datapointAttributes)
	tableDataMap.Set(SummaryDatapointQuantileValueTable, datapointQuantiles)
	tableDataMap.Set(SummaryResourceAttributeTable, resourceAttributes)
	tableDataMap.Set(SummaryScopeAttributeTable, scopeAttributes)

	errs = append(errs, kiwriter.writeMetric(pmetric.MetricTypeSummary.String(), tableDataMap))

	return multierr.Combine(errs...)

}

func (kiwriter *KiWriter) doChunkedInsert(ctx context.Context, tableName string, records []any) error {

	// Build the final table name with the schema prepended
	var finalTable string
	if len(kiwriter.cfg.Schema) != 0 {
		finalTable = fmt.Sprintf("%s.%s", kiwriter.cfg.Schema, tableName)
	} else {
		finalTable = tableName
	}

	kiwriter.logger.Debug("Writing to - ", zap.String("Table", finalTable), zap.Int("Record count", len(records)))

	recordChunks := ChunkBySize(records, ChunkSize)

	errsChan := make(chan error, len(recordChunks))

	wg := &sync.WaitGroup{}

	for _, recordChunk := range recordChunks {
		wg.Add(1)
		go func(data []any, wg *sync.WaitGroup) {
			_, err := kiwriter.Db.InsertRecordsRaw(context.TODO(), finalTable, data)
			errsChan <- err

			wg.Done()
		}(recordChunk, wg)
	}
	wg.Wait()
	close(errsChan)
	var errs error
	for err := range errsChan {
		errs = multierr.Append(errs, err)
	}
	return errs
}
