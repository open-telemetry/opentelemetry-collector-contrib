// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"context"
	"fmt"
	"sync"

	"github.com/kineticadb/kinetica-api-go/kinetica"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// attributeValue - struct to contain attribute values of different types
// Used by other metric structs
type attributeValue struct {
	IntValue    int     `avro:"int_value"`
	StringValue string  `avro:"string_value"`
	BoolValue   int8    `avro:"bool_value"`
	DoubleValue float64 `avro:"double_value"`
	BytesValue  []byte  `avro:"bytes_value"`
}

// kiWriter - struct modeling the Kinetica connection, contains the
// Kinetica connection [kinetica.Kinetica], the Kinetica Options [kinetica.KineticaOptions],
// the config [Config] and the logger [zap.Logger]
type kiWriter struct {
	Db      kinetica.Kinetica
	Options kinetica.KineticaOptions
	cfg     Config
	logger  *zap.Logger
}

// GetDb - Getter for the Kinetica instance
//
//	@receiver kiwriter
//	@return gpudb.Gpudb
func (kiwriter *kiWriter) GetDb() kinetica.Kinetica {
	return kiwriter.Db
}

// GetOptions - Getter for the Kinetica options.
//
//	@receiver kiwriter
//	@return gpudb.GpudbOptions
func (kiwriter *kiWriter) GetOptions() kinetica.KineticaOptions {
	return kiwriter.Options
}

// GetCfg - Getter for the [Config] value
//
//	@receiver kiwriter
//	@return Config
func (kiwriter *kiWriter) GetCfg() Config {
	return kiwriter.cfg
}

// Writer - global pointer to kiwriter struct initialized in the init func
var Writer *kiWriter

// init
func init() {
	ctx := context.TODO()
	cfg := createDefaultConfig()
	config := cfg.(*Config)
	options := kinetica.KineticaOptions{Username: config.Username, Password: string(config.Password), ByPassSslCertCheck: config.BypassSslCertCheck}
	gpudbInst := kinetica.NewWithOptions(ctx, config.Host, &options)
	Writer = &kiWriter{*gpudbInst, options, *config, nil}
}

// newKiWriter - Constructor for the [kiWriter] struct
//
//	@param ctx
//	@param cfg
//	@return *kiWriter
func newKiWriter(ctx context.Context, cfg Config, logger *zap.Logger) *kiWriter {
	options := kinetica.KineticaOptions{Username: cfg.Username, Password: string(cfg.Password), ByPassSslCertCheck: cfg.BypassSslCertCheck}
	gpudbInst := kinetica.NewWithOptions(ctx, cfg.Host, &options)
	return &kiWriter{*gpudbInst, options, cfg, logger}
}

// Metrics Handling

// gauge - struct modeling the gauge data
type gauge struct {
	GaugeID     string `avro:"gauge_id"`
	MetricName  string `avro:"metric_name"`
	Description string `avro:"metric_description"`
	Unit        string `avro:"metric_unit"`
}

// gaugeDatapoint - struct modeling the gauge Datapoint
type gaugeDatapoint struct {
	GaugeID       string  `avro:"gauge_id"`
	ID            string  `avro:"id"`
	StartTimeUnix int64   `mapstructure:"start_time_unix" avro:"start_time_unix"`
	TimeUnix      int64   `mapstructure:"time_unix" avro:"time_unix"`
	GaugeValue    float64 `mapstructure:"gauge_value" avro:"gauge_value"`
	Flags         int     `mapstructure:"flags" avro:"flags"`
}

// gaugeDatapointAttribute - struct modeling the gauge Datapoint attributes
type gaugeDatapointAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// gaugeDatapointExemplar - struct modeling a gauge Datapoint Exemplar
type gaugeDatapointExemplar struct {
	GaugeID     string  `avro:"gauge_id"`
	DatapointID string  `avro:"datapoint_id"`
	ExemplarID  string  `avro:"exemplar_id"`
	TimeUnix    int64   `mapstructure:"time_unix" avro:"time_unix"`
	GaugeValue  float64 `mapstructure:"gauge_value" avro:"gauge_value"`
	TraceID     string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID      string  `mapstructure:"span_id" avro:"span_id"`
}

// gaugeDataPointExemplarAttribute - struct modeling a gauge Datapoint Exemplar attribute
type gaugeDataPointExemplarAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// gaugeResourceAttribute - struct modeling a gauge resource attribute
type gaugeResourceAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// gaugeScopeAttribute - struct modeling a gauge Scope attribute
type gaugeScopeAttribute struct {
	GaugeID        string `avro:"gauge_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// END gauge

// sum

// sum - struct modeling a sum metric
type sum struct {
	SumID                          string `avro:"sum_id"`
	MetricName                     string `avro:"metric_name"`
	Description                    string `avro:"metric_description"`
	Unit                           string `avro:"metric_unit"`
	pmetric.AggregationTemporality `avro:"aggregation_temporality"`
	IsMonotonic                    int8 `avro:"is_monotonic"`
}

// sumDatapoint - struct modeling a sum Datapoint
type sumDatapoint struct {
	SumID         string  `avro:"sum_id"`
	ID            string  `avro:"id"`
	StartTimeUnix int64   `mapstructure:"start_time_unix" avro:"start_time_unix"`
	TimeUnix      int64   `mapstructure:"time_unix" avro:"time_unix"`
	SumValue      float64 `mapstructure:"sum_value" avro:"sum_value"`
	Flags         int     `mapstructure:"flags" avro:"flags"`
}

// sumDataPointAttribute - struct modeling a sum Datapoint attribute
type sumDataPointAttribute struct {
	SumID          string `avro:"sum_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// sumDatapointExemplar - struct modeling a sum Datapoint Exemplar
type sumDatapointExemplar struct {
	SumID       string  `avro:"sum_id"`
	DatapointID string  `avro:"datapoint_id"`
	ExemplarID  string  `avro:"exemplar_id"`
	TimeUnix    int64   `mapstructure:"time_unix" avro:"time_unix"`
	SumValue    float64 `mapstructure:"sum_value" avro:"sum_value"`
	TraceID     string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID      string  `mapstructure:"span_id" avro:"span_id"`
}

// sumDataPointExemplarAttribute  - struct modeling a sum Datapoint Exemplar attribute
type sumDataPointExemplarAttribute struct {
	SumID          string `avro:"sum_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// sumResourceAttribute - struct modeling a sum Resource attribute
type sumResourceAttribute struct {
	SumID          string `avro:"sum_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// sumScopeAttribute - struct modeling a sum Scope attribute
type sumScopeAttribute struct {
	SumID          string `avro:"sum_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// END sum

// histogram

// histogram - struct modeling a histogram metric type
type histogram struct {
	HistogramID                    string `avro:"histogram_id"`
	MetricName                     string `avro:"metric_name"`
	Description                    string `avro:"metric_description"`
	Unit                           string `avro:"metric_unit"`
	pmetric.AggregationTemporality `avro:"aggregation_temporality"`
}

// histogramDatapoint - struct modeling a histogram Datapoint
type histogramDatapoint struct {
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

// histogramDataPointAttribute - struct modeling a histogram Datapoint attribute
type histogramDataPointAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// histogramDatapointBucketCount - struct modeling a histogram Datapoint Bucket Count
type histogramDatapointBucketCount struct {
	HistogramID string `avro:"histogram_id"`
	DatapointID string `avro:"datapoint_id"`
	CountID     string `avro:"count_id"`
	Count       int64  `avro:"count"`
}

// histogramDatapointExplicitBound - struct modeling a histogram Datapoint Explicit Bound
type histogramDatapointExplicitBound struct {
	HistogramID   string  `avro:"histogram_id"`
	DatapointID   string  `avro:"datapoint_id"`
	BoundID       string  `avro:"bound_id"`
	ExplicitBound float64 `avro:"explicit_bound"`
}

// histogramDatapointExemplar - struct modeling a histogram Datapoint Exemplar
type histogramDatapointExemplar struct {
	HistogramID    string  `avro:"histogram_id"`
	DatapointID    string  `avro:"datapoint_id"`
	ExemplarID     string  `avro:"exemplar_id"`
	TimeUnix       int64   `avro:"time_unix"`
	HistogramValue float64 `avro:"histogram_value"`
	TraceID        string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID         string  `mapstructure:"span_id" avro:"span_id"`
}

// histogramDataPointExemplarAttribute - struct modeling a histogram Datapoint Exemplar attribute
type histogramDataPointExemplarAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// histogramResourceAttribute - struct modeling a histogram Resource Attribute
type histogramResourceAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// histogramScopeAttribute - struct modeling a histogram Scope Attribute
type histogramScopeAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// End histogram

// Exponential histogram

// exponentialHistogram - struct modeling an Exponential histogram
type exponentialHistogram struct {
	HistogramID                    string `avro:"histogram_id"`
	MetricName                     string `avro:"metric_name"`
	Description                    string `avro:"metric_description"`
	Unit                           string `avro:"metric_unit"`
	pmetric.AggregationTemporality `avro:"aggregation_temporality"`
}

// exponentialHistogramDatapoint - struct modeling an Exponential histogram Datapoint
type exponentialHistogramDatapoint struct {
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

// exponentialHistogramDataPointAttribute - struct modeling an Exponential histogram Datapoint attribute
type exponentialHistogramDataPointAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// exponentialHistogramBucketNegativeCount - struct modeling an Exponential histogram Bucket Negative Count
type exponentialHistogramBucketNegativeCount struct {
	HistogramID string `avro:"histogram_id"`
	DatapointID string `avro:"datapoint_id"`
	CountID     string `avro:"count_id"`
	Count       uint64 `avro:"count"`
}

// exponentialHistogramBucketPositiveCount - struct modeling an Exponential histogram Bucket Positive Count
type exponentialHistogramBucketPositiveCount struct {
	HistogramID string `avro:"histogram_id"`
	DatapointID string `avro:"datapoint_id"`
	CountID     string `avro:"count_id"`
	Count       int64  `avro:"count"`
}

// exponentialHistogramDatapointExemplar - struct modeling an Exponential histogram Datapoint Exemplar
type exponentialHistogramDatapointExemplar struct {
	HistogramID    string  `avro:"histogram_id"`
	DatapointID    string  `avro:"datapoint_id"`
	ExemplarID     string  `avro:"exemplar_id"`
	TimeUnix       int64   `avro:"time_unix"`
	HistogramValue float64 `avro:"histogram_value"`
	TraceID        string  `mapstructure:"trace_id" avro:"trace_id"`
	SpanID         string  `mapstructure:"span_id" avro:"span_id"`
}

// exponentialHistogramDataPointExemplarAttribute - struct modeling an Exponential histogram Datapoint Exemplar attribute
type exponentialHistogramDataPointExemplarAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	DatapointID    string `avro:"datapoint_id"`
	ExemplarID     string `avro:"exemplar_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// exponentialHistogramResourceAttribute - struct modeling an Exponential histogram Resource attribute
type exponentialHistogramResourceAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// exponentialHistogramScopeAttribute - struct modeling an Exponential histogram Scope attribute
type exponentialHistogramScopeAttribute struct {
	HistogramID    string `avro:"histogram_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// END Exponential histogram

// summary

// summary - struct modeling a summary type metric
type summary struct {
	SummaryID   string `avro:"summary_id"`
	MetricName  string `avro:"metric_name"`
	Description string `avro:"metric_description"`
	Unit        string `avro:"metric_unit"`
}

// summaryDatapoint - struct modeling a summary Datapoint
type summaryDatapoint struct {
	SummaryID     string  `avro:"summary_id"`
	ID            string  `avro:"id"`
	StartTimeUnix int64   `avro:"start_time_unix"`
	TimeUnix      int64   `avro:"time_unix"`
	Count         int64   `avro:"count"`
	Sum           float64 `avro:"data_sum"`
	Flags         int     `avro:"flags"`
}

// SummaryDataPointAttribute - struct modeling a summary Datapoint attribute
type SummaryDataPointAttribute struct {
	SummaryID      string `avro:"summary_id"`
	DatapointID    string `avro:"datapoint_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// summaryDatapointQuantileValues - struct modeling a summary Datapoint Quantile value
type summaryDatapointQuantileValues struct {
	SummaryID   string  `avro:"summary_id"`
	DatapointID string  `avro:"datapoint_id"`
	QuantileID  string  `avro:"quantile_id"`
	Quantile    float64 `avro:"quantile"`
	Value       float64 `avro:"value"`
}

// summaryResourceAttribute - struct modeling a summary Resource attribute
type summaryResourceAttribute struct {
	SummaryID      string `avro:"summary_id"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// summaryScopeAttribute - struct modeling a summary Scope attribute
type summaryScopeAttribute struct {
	SummaryID      string `avro:"summary_id"`
	ScopeName      string `avro:"name"`
	ScopeVersion   string `avro:"version"`
	Key            string `avro:"key"`
	attributeValue `mapstructure:",squash"`
}

// END summary

// END Metrics Handling

// writeMetric - a helper method used by different metric persistence methods to write the
// metric data in order.
//
//	@receiver kiwriter - pointer to [kiWriter]
//	@param metricType - a [pmetric.MetricTypeGauge] or something else converted to string
//	@param tableDataMap - a map from table name to the relevant data
//	@return error
func (kiwriter *kiWriter) writeMetric(metricType string, tableDataMap *orderedmap.OrderedMap[string, []any]) error {
	kiwriter.logger.Debug("Writing metric", zap.String("Type", metricType))

	var errs []error
	errsChan := make(chan error, tableDataMap.Len())

	wg := &sync.WaitGroup{}
	for pair := tableDataMap.Oldest(); pair != nil; pair = pair.Next() {
		tableName := pair.Key
		data := pair.Value

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

func (kiwriter *kiWriter) persistGaugeRecord(gaugeRecords []kineticaGaugeRecord) error {
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

	tableDataMap := orderedmap.New[string, []any]()

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

func (kiwriter *kiWriter) persistSumRecord(sumRecords []kineticaSumRecord) error {
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

	tableDataMap := orderedmap.New[string, []any]()

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

func (kiwriter *kiWriter) persistHistogramRecord(histogramRecords []kineticaHistogramRecord) error {
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

		for _, dpattr := range histogramrecord.histogramDatapointAttribute {
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

	tableDataMap := orderedmap.New[string, []any]()

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

func (kiwriter *kiWriter) persistExponentialHistogramRecord(exponentialHistogramRecords []kineticaExponentialHistogramRecord) error {
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

	tableDataMap := orderedmap.New[string, []any]()

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

func (kiwriter *kiWriter) persistSummaryRecord(summaryRecords []kineticaSummaryRecord) error {
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

	tableDataMap := orderedmap.New[string, []any]()

	tableDataMap.Set(SummaryTable, summaries)
	tableDataMap.Set(SummaryDatapointTable, datapoints)
	tableDataMap.Set(SummaryDatapointAttributeTable, datapointAttributes)
	tableDataMap.Set(SummaryDatapointQuantileValueTable, datapointQuantiles)
	tableDataMap.Set(SummaryResourceAttributeTable, resourceAttributes)
	tableDataMap.Set(SummaryScopeAttributeTable, scopeAttributes)

	errs = append(errs, kiwriter.writeMetric(pmetric.MetricTypeSummary.String(), tableDataMap))

	return multierr.Combine(errs...)
}

func (kiwriter *kiWriter) doChunkedInsert(_ context.Context, tableName string, records []any) error {
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
