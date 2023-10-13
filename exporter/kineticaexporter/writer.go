// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"context"
	"fmt"
	"sync"

	"github.com/kineticadb/kinetica-api-go/kinetica"
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

// newAttributeValue Constructor for AttributeValue
//
//	@param intValue
//	@param stringValue
//	@param boolValue
//	@param doubleValue
//	@param bytesValue
//	@return *AttributeValue
// func newAttributeValue(intValue int, stringValue string, boolValue int8, doubleValue float64, bytesValue []byte) *AttributeValue {
// 	o := new(AttributeValue)
// 	o.IntValue = intValue
// 	o.StringValue = stringValue
// 	o.BoolValue = boolValue
// 	o.DoubleValue = doubleValue
// 	o.BytesValue = bytesValue
// 	return o
// }

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
	cfg := createDefaultConfig()
	config := cfg.(*Config)
	options := kinetica.KineticaOptions{Username: config.Username, Password: string(config.Password), ByPassSslCertCheck: config.BypassSslCertCheck}
	gpudbInst := kinetica.NewWithOptions(ctx, config.Host, &options)
	Writer = &KiWriter{*gpudbInst, options, *config, nil}
}

// newKiWriter - Constructor for the [KiWriter] struct
//
//	@param ctx
//	@param cfg
//	@return *KiWriter
func newKiWriter(ctx context.Context, cfg Config, logger *zap.Logger) *KiWriter {
	options := kinetica.KineticaOptions{Username: cfg.Username, Password: string(cfg.Password), ByPassSslCertCheck: cfg.BypassSslCertCheck}
	gpudbInst := kinetica.NewWithOptions(ctx, cfg.Host, &options)
	return &KiWriter{*gpudbInst, options, cfg, logger}
}

// getGpuDbInst - Creates and returns a new [kinetica.Kinetica] struct
//
//	@param cfg
//	@return *gpudb.Gpudb
// func getGpuDbInst(cfg *Config) *kinetica.Kinetica {
// 	ctx := context.TODO()
// 	options := kinetica.KineticaOptions{Username: cfg.Username, Password: string(cfg.Password), ByPassSslCertCheck: cfg.BypassSslCertCheck}
// 	// fmt.Println("Options", options)
// 	gpudbInst := kinetica.NewWithOptions(ctx, cfg.Host, &options)

// 	return gpudbInst

// }

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
	SumID                          string `avro:"sum_id"`
	MetricName                     string `avro:"metric_name"`
	Description                    string `avro:"metric_description"`
	Unit                           string `avro:"metric_unit"`
	pmetric.AggregationTemporality `avro:"aggregation_temporality"`
	IsMonotonic                    int8 `avro:"is_monotonic"`
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
	HistogramID                    string `avro:"histogram_id"`
	MetricName                     string `avro:"metric_name"`
	Description                    string `avro:"metric_description"`
	Unit                           string `avro:"metric_unit"`
	pmetric.AggregationTemporality `avro:"aggregation_temporality"`
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
	HistogramID                    string `avro:"histogram_id"`
	MetricName                     string `avro:"metric_name"`
	Description                    string `avro:"metric_description"`
	Unit                           string `avro:"metric_unit"`
	pmetric.AggregationTemporality `avro:"aggregation_temporality"`
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

func (kiwriter *KiWriter) doChunkedInsert(_ context.Context, tableName string, records []any) error {

	// Build the final table name with the schema prepended
	var finalTable string
	if len(kiwriter.cfg.Schema) != 0 {
		finalTable = fmt.Sprintf("%s.%s", kiwriter.cfg.Schema, tableName)
	} else {
		finalTable = tableName
	}

	kiwriter.logger.Debug("Writing to - ", zap.String("Table", finalTable), zap.Int("Record count", len(records)))

	recordChunks := chunkBySize(records, ChunkSize)

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
