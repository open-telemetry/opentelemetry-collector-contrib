// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// Metrics handling

type kineticaGaugeRecord struct {
	gauge              *Gauge
	resourceAttribute  []GaugeResourceAttribute
	scopeAttribute     []GaugeScopeAttribute
	datapoint          []GaugeDatapoint
	datapointAttribute []GaugeDatapointAttribute
	exemplars          []GaugeDatapointExemplar
	exemplarAttribute  []GaugeDataPointExemplarAttribute
}

type kineticaSumRecord struct {
	sum                  *Sum
	sumResourceAttribute []SumResourceAttribute
	sumScopeAttribute    []SumScopeAttribute
	datapoint            []SumDatapoint
	datapointAttribute   []SumDataPointAttribute
	exemplars            []SumDatapointExemplar
	exemplarAttribute    []SumDataPointExemplarAttribute
}

type kineticaHistogramRecord struct {
	histogram                   *Histogram
	histogramResourceAttribute  []HistogramResourceAttribute
	histogramScopeAttribute     []HistogramScopeAttribute
	histogramDatapoint          []HistogramDatapoint
	histogramDatapointAttribute []HistogramDataPointAttribute
	histogramBucketCount        []HistogramDatapointBucketCount
	histogramExplicitBound      []HistogramDatapointExplicitBound
	exemplars                   []HistogramDatapointExemplar
	exemplarAttribute           []HistogramDataPointExemplarAttribute
}

type kineticaExponentialHistogramRecord struct {
	histogram                    *ExponentialHistogram
	histogramResourceAttribute   []ExponentialHistogramResourceAttribute
	histogramScopeAttribute      []ExponentialHistogramScopeAttribute
	histogramDatapoint           []ExponentialHistogramDatapoint
	histogramDatapointAttribute  []ExponentialHistogramDataPointAttribute
	histogramBucketNegativeCount []ExponentialHistogramBucketNegativeCount
	histogramBucketPositiveCount []ExponentialHistogramBucketPositiveCount
	exemplars                    []ExponentialHistogramDatapointExemplar
	exemplarAttribute            []ExponentialHistogramDataPointExemplarAttribute
}

type kineticaSummaryRecord struct {
	summary                        *Summary
	summaryDatapoint               []SummaryDatapoint
	summaryDatapointAttribute      []SummaryDataPointAttribute
	summaryResourceAttribute       []SummaryResourceAttribute
	summaryScopeAttribute          []SummaryScopeAttribute
	summaryDatapointQuantileValues []SummaryDatapointQuantileValues
}

var (
	gaugeTableDDLs = []string{
		CreateGauge,
		CreateGaugeDatapoint,
		CreateGaugeDatapointAttribute,
		CreateGaugeDatapointExemplar,
		CreateGaugeDatapointExemplarAttribute,
		CreateGaugeResourceAttribute,
		CreateGaugeScopeAttribute,
	}

	sumTableDDLs = []string{
		CreateSum,
		CreateSumDatapoint,
		CreateSumDatapointAttribute,
		CreateSumDatapointExemplar,
		CreateSumDatapointExemplarAttribute,
		CreateSumResourceAttribute,
		CreateSumScopeAttribute,
	}

	histogramTableDDLs = []string{
		CreateHistogram,
		CreateHistogramDatapoint,
		CreateHistogramDatapointBucketCount,
		CreateHistogramDatapointExplicitBound,
		CreateHistogramDatapointAttribute,
		CreateHistogramDatapointExemplar,
		CreateHistogramDatapointExemplarAttribute,
		CreateHistogramResourceAttribute,
		CreateHistogramScopeAttribute,
	}

	expHistogramTableDDLs = []string{
		CreateExpHistogram,
		CreateExpHistogramDatapoint,
		CreateExpHistogramDatapointBucketPositiveCount,
		CreateExpHistogramDatapointBucketNegativeCount,
		CreateExpHistogramDatapointAttribute,
		CreateExpHistogramDatapointExemplar,
		CreateExpHistogramDatapointExemplarAttribute,
		CreateExpHistogramResourceAttribute,
		CreateExpHistogramScopeAttribute,
	}

	summaryTableDDLs = []string{
		CreateSummary,
		CreateSummaryDatapoint,
		CreateSummaryDatapointAttribute,
		CreateSummaryDatapointQuantileValues,
		CreateSummaryResourceAttribute,
		CreateSummaryScopeAttribute,
	}
)

type kineticaMetricsExporter struct {
	logger *zap.Logger
	writer *KiWriter
}

func newMetricsExporter(logger *zap.Logger, cfg *Config) *kineticaMetricsExporter {
	kineticaLogger := logger
	writer := newKiWriter(context.TODO(), *cfg, kineticaLogger)
	metricsExp := &kineticaMetricsExporter{
		logger: kineticaLogger,
		writer: writer,
	}
	return metricsExp
}

func (e *kineticaMetricsExporter) start(ctx context.Context, _ component.Host) error {
	fmt.Println("SCHEMA NAME - ", e.writer.cfg.Schema)

	if e.writer.cfg.Schema != "" && len(e.writer.cfg.Schema) != 0 {
		// Config has a schema name
		if err := createSchema(ctx, e.writer, e.writer.cfg); err != nil {
			return err
		}
	}
	err := createMetricTables(ctx, e.writer)
	return err
}

// createMetricTables
//
//	@param ctx
//	@param kiWriter
//	@return error
func createMetricTables(ctx context.Context, kiWriter *KiWriter) error {
	var errs []error

	// create gauge tables
	err := createTablesForMetricType(ctx, gaugeTableDDLs, kiWriter)
	if err != nil {
		errs = append(errs, err)
	}

	// create sum tables
	err = createTablesForMetricType(ctx, sumTableDDLs, kiWriter)
	if err != nil {
		errs = append(errs, err)
	}

	// create histogram tables
	err = createTablesForMetricType(ctx, histogramTableDDLs, kiWriter)
	if err != nil {
		errs = append(errs, err)
	}

	// create exponential histogram tables
	err = createTablesForMetricType(ctx, expHistogramTableDDLs, kiWriter)
	if err != nil {
		errs = append(errs, err)
	}

	// create summary tables
	err = createTablesForMetricType(ctx, summaryTableDDLs, kiWriter)
	if err != nil {
		errs = append(errs, err)
	}

	return multierr.Combine(errs...)
}

func createTablesForMetricType(ctx context.Context, metricTypeDDLs []string, kiWriter *KiWriter) error {
	var errs []error

	var schema string
	schema = strings.Trim(kiWriter.cfg.Schema, " ")
	if len(schema) > 0 {
		schema += "."
	} else {
		schema = ""
	}

	lo.ForEach(metricTypeDDLs, func(ddl string, _ int) {
		stmt := strings.ReplaceAll(ddl, "%s", schema)
		kiWriter.logger.Debug("Creating Table - ", zap.String("DDL", stmt))

		_, err := kiWriter.Db.ExecuteSqlRaw(ctx, stmt, 0, 0, "", nil)
		if err != nil {
			kiWriter.logger.Error(err.Error())
			errs = append(errs, err)
		}
	})

	return multierr.Combine(errs...)
}

// createSchema - Create a schema
//
//	@param ctx
//	@param kiWriter
//	@param config
//	@return bool - True if schema creation successful
func createSchema(ctx context.Context, kiWriter *KiWriter, config Config) error {
	stmt := fmt.Sprintf(CreateSchema, config.Schema)
	kiWriter.logger.Debug(stmt)
	_, err := kiWriter.Db.ExecuteSqlRaw(ctx, stmt, 0, 0, "", nil)
	if err != nil {
		kiWriter.logger.Error(err.Error())
	}
	return err
}

// shutdown will shut down the exporter.
func (e *kineticaMetricsExporter) shutdown(_ context.Context) error {
	return nil
}

// pushMetricsData - this method is called by the collector to feed the metrics data to the exporter
//
//	@receiver e
//	@param _ ctx unused
//	@param md
//	@return error
func (e *kineticaMetricsExporter) pushMetricsData(_ context.Context, md pmetric.Metrics) error {
	var metricType pmetric.MetricType
	var errs []error

	var gaugeRecords []kineticaGaugeRecord
	var sumRecords []kineticaSumRecord
	var histogramRecords []kineticaHistogramRecord
	var exponentialHistogramRecords []kineticaExponentialHistogramRecord
	var summaryRecords []kineticaSummaryRecord

	e.logger.Debug("Resource metrics ", zap.Int("count = ", md.ResourceMetrics().Len()))

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		metrics := md.ResourceMetrics().At(i)
		resAttr := metrics.Resource().Attributes()

		e.logger.Debug("Scope metrics ", zap.Int("count = ", metrics.ScopeMetrics().Len()))

		for j := 0; j < metrics.ScopeMetrics().Len(); j++ {
			metricSlice := metrics.ScopeMetrics().At(j).Metrics()
			scopeInstr := metrics.ScopeMetrics().At(j).Scope()
			scopeURL := metrics.ScopeMetrics().At(j).SchemaUrl()

			e.logger.Debug("metrics ", zap.Int("count = ", metricSlice.Len()))

			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				metricType = metric.Type()
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					gaugeRecord, err := e.createGaugeRecord(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, metric.Gauge(), metric.Name(), metric.Description(), metric.Unit())
					if err == nil {
						gaugeRecords = append(gaugeRecords, *gaugeRecord)
						e.logger.Debug("Added gauge")
					} else {
						e.logger.Error(err.Error())
						errs = append(errs, err)
					}
				case pmetric.MetricTypeSum:
					sumRecord, err := e.createSumRecord(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, metric.Sum(), metric.Name(), metric.Description(), metric.Unit())
					if err == nil {
						sumRecords = append(sumRecords, *sumRecord)
						e.logger.Debug("Added sum")
					} else {
						e.logger.Error(err.Error())
						errs = append(errs, err)
					}
				case pmetric.MetricTypeHistogram:
					histogramRecord, err := e.createHistogramRecord(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, metric.Histogram(), metric.Name(), metric.Description(), metric.Unit())
					if err == nil {
						histogramRecords = append(histogramRecords, *histogramRecord)
						e.logger.Debug("Added histogram")
					} else {
						e.logger.Error(err.Error())
						errs = append(errs, err)
					}
				case pmetric.MetricTypeExponentialHistogram:
					exponentialHistogramRecord, err := e.createExponentialHistogramRecord(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, metric.ExponentialHistogram(), metric.Name(), metric.Description(), metric.Unit())
					if err == nil {
						exponentialHistogramRecords = append(exponentialHistogramRecords, *exponentialHistogramRecord)
						e.logger.Debug("Added exp histogram")
					} else {
						e.logger.Error(err.Error())
						errs = append(errs, err)
					}
				case pmetric.MetricTypeSummary:
					summaryRecord, err := e.createSummaryRecord(resAttr, metrics.SchemaUrl(), scopeInstr, scopeURL, metric.Summary(), metric.Name(), metric.Description(), metric.Unit())
					if err == nil {
						summaryRecords = append(summaryRecords, *summaryRecord)
						e.logger.Debug("Added summary")
					} else {
						e.logger.Error(err.Error())
						errs = append(errs, err)
					}
				default:
					return fmt.Errorf("Unsupported metrics type")
				}

				e.logger.Debug("Gauge ", zap.Int("count = ", len(gaugeRecords)))
				e.logger.Debug("Sum ", zap.Int("count = ", len(sumRecords)))
				e.logger.Debug("Histogram ", zap.Int("count = ", len(histogramRecords)))
				e.logger.Debug("Exp Histogram ", zap.Int("count = ", len(exponentialHistogramRecords)))
				e.logger.Debug("Summary ", zap.Int("count = ", len(summaryRecords)))

				if len(errs) > 0 {
					e.logger.Error(multierr.Combine(errs...).Error())
					return multierr.Combine(errs...)
				}
			}
		}
	}

	e.logger.Debug("Before writing metrics into Kinetica")

	switch metricType {
	case pmetric.MetricTypeGauge:
		if err := e.writer.persistGaugeRecord(gaugeRecords); err != nil {
			errs = append(errs, err)
			e.logger.Error(err.Error())
		}
	case pmetric.MetricTypeSum:
		if err := e.writer.persistSumRecord(sumRecords); err != nil {
			errs = append(errs, err)
			e.logger.Error(err.Error())
		}
	case pmetric.MetricTypeHistogram:
		if err := e.writer.persistHistogramRecord(histogramRecords); err != nil {
			errs = append(errs, err)
			e.logger.Error(err.Error())
		}
	case pmetric.MetricTypeExponentialHistogram:
		if err := e.writer.persistExponentialHistogramRecord(exponentialHistogramRecords); err != nil {
			errs = append(errs, err)
			e.logger.Error(err.Error())
		}
	case pmetric.MetricTypeSummary:
		if err := e.writer.persistSummaryRecord(summaryRecords); err != nil {
			errs = append(errs, err)
			e.logger.Error(err.Error())
		}
	default:
		return fmt.Errorf("Unsupported metrics type")
	}
	return multierr.Combine(errs...)
}

// createSummaryRecord - creates a Summary type record
//
//	@receiver e - Method applicable to [kineticaMetricsExporter]
//	@param resAttr - a map of key to value of resource attributes
//	@param _ schemaURL - unused
//	@param scopeInstr - the instrumentation scope
//	@param _ scopeURL - unused
//	@param summaryRecord - the Summary [pmetric.Summary] record
//	@param name
//	@param description
//	@param unit
//	@return *kineticaSummaryRecord
//	@return error
func (e *kineticaMetricsExporter) createSummaryRecord(resAttr pcommon.Map, _ string, scopeInstr pcommon.InstrumentationScope, _ string, summaryRecord pmetric.Summary, name, description, unit string) (*kineticaSummaryRecord, error) {
	var errs []error

	kiSummaryRecord := new(kineticaSummaryRecord)

	summary := &Summary{
		SummaryID:   uuid.New().String(),
		MetricName:  name,
		Description: description,
		Unit:        unit,
	}

	kiSummaryRecord.summary = summary

	// Handle data points
	var datapointAttribute []SummaryDataPointAttribute
	datapointAttributes := make(map[string]ValueTypePair)

	for i := 0; i < summaryRecord.DataPoints().Len(); i++ {
		datapoint := summaryRecord.DataPoints().At(i)
		summaryDatapoint := &SummaryDatapoint{
			SummaryID:     summary.SummaryID,
			ID:            uuid.New().String(),
			StartTimeUnix: datapoint.StartTimestamp().AsTime().UnixMilli(),
			TimeUnix:      datapoint.Timestamp().AsTime().UnixMilli(),
			Count:         int64(datapoint.Count()),
			Sum:           datapoint.Sum(),
			Flags:         int(datapoint.Flags()),
		}
		kiSummaryRecord.summaryDatapoint = append(kiSummaryRecord.summaryDatapoint, *summaryDatapoint)

		// Handle summary datapoint attribute
		for k, v := range datapoint.Attributes().All() {
			if k == "" {
				e.logger.Debug("Sum record attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				datapointAttributes[k] = v
			} else {
				e.logger.Debug("Invalid sum record attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range datapointAttributes {
			vtPair := datapointAttributes[key]
			sa, err := e.newSummaryDatapointAttributeValue(summary.SummaryID, summaryDatapoint.ID, key, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				datapointAttribute = append(datapointAttribute, *sa)
			}
		}
		kiSummaryRecord.summaryDatapointAttribute = append(kiSummaryRecord.summaryDatapointAttribute, datapointAttribute...)

		for k := range datapointAttributes {
			delete(datapointAttributes, k)
		}

		// Handle quantile values
		quantileValues := datapoint.QuantileValues()
		for i := 0; i < quantileValues.Len(); i++ {
			quantileValue := quantileValues.At(i)
			summaryQV := &SummaryDatapointQuantileValues{
				SummaryID:   summary.SummaryID,
				DatapointID: summaryDatapoint.ID,
				QuantileID:  uuid.New().String(),
				Quantile:    quantileValue.Quantile(),
				Value:       quantileValue.Value(),
			}
			kiSummaryRecord.summaryDatapointQuantileValues = append(kiSummaryRecord.summaryDatapointQuantileValues, *summaryQV)
		}
	}

	// Handle Resource attribute
	var resourceAttribute []SummaryResourceAttribute
	resourceAttributes := make(map[string]ValueTypePair)

	for k, v := range resAttr.All() {
		if k == "" {
			e.logger.Debug("Resource attribute key is empty")
		} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
			resourceAttributes[k] = v
		} else {
			e.logger.Debug("Invalid resource attribute value", zap.String("Error", err.Error()))
			errs = append(errs, err)
		}
	}

	for key := range resourceAttributes {
		vtPair := resourceAttributes[key]
		ga, err := e.newSummaryResourceAttributeValue(summary.SummaryID, key, vtPair)
		if err != nil {
			e.logger.Error(err.Error())
		} else {
			resourceAttribute = append(resourceAttribute, *ga)
		}
	}

	copy(kiSummaryRecord.summaryResourceAttribute, resourceAttribute)

	// Handle Scope attribute
	var scopeAttribute []SummaryScopeAttribute
	scopeAttributes := make(map[string]ValueTypePair)
	scopeName := scopeInstr.Name()
	scopeVersion := scopeInstr.Version()

	for k, v := range scopeInstr.Attributes().All() {
		if k == "" {
			e.logger.Debug("Scope attribute key is empty")
		} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
			scopeAttributes[k] = v
		} else {
			e.logger.Debug("Invalid scope attribute value", zap.String("Error", err.Error()))
			errs = append(errs, err)
		}
	}

	for key := range scopeAttributes {
		vtPair := scopeAttributes[key]
		sa, err := e.newSummaryScopeAttributeValue(summary.SummaryID, key, scopeName, scopeVersion, vtPair)
		if err != nil {
			e.logger.Error(err.Error())
		} else {
			scopeAttribute = append(scopeAttribute, *sa)
		}
	}

	copy(kiSummaryRecord.summaryScopeAttribute, scopeAttribute)

	return kiSummaryRecord, multierr.Combine(errs...)
}

// createExponentialHistogramRecord - creates an exponential histogram type record
//
//	@receiver e
//	@param resAttr
//	@param _ schemaURL - unused
//	@param scopeInstr
//	@param _ scopeURL - unused
//	@param exponentialHistogramRecord
//	@param name
//	@param description
//	@param unit
//	@return *kineticaExponentialHistogramRecord
//	@return error
func (e *kineticaMetricsExporter) createExponentialHistogramRecord(resAttr pcommon.Map, _ string, scopeInstr pcommon.InstrumentationScope, _ string, exponentialHistogramRecord pmetric.ExponentialHistogram, name, description, unit string) (*kineticaExponentialHistogramRecord, error) {
	var errs []error

	kiExpHistogramRecord := new(kineticaExponentialHistogramRecord)

	histogram := &ExponentialHistogram{
		HistogramID:            uuid.New().String(),
		MetricName:             name,
		Description:            description,
		Unit:                   unit,
		AggregationTemporality: exponentialHistogramRecord.AggregationTemporality(),
	}

	kiExpHistogramRecord.histogram = histogram

	// Handle data points
	var datapointAttribute []ExponentialHistogramDataPointAttribute
	datapointAttributes := make(map[string]ValueTypePair)

	var exemplarAttribute []ExponentialHistogramDataPointExemplarAttribute
	exemplarAttributes := make(map[string]ValueTypePair)

	var datapointBucketPositiveCount []ExponentialHistogramBucketPositiveCount
	var datapointBucketNegativeCount []ExponentialHistogramBucketNegativeCount

	for i := 0; i < exponentialHistogramRecord.DataPoints().Len(); i++ {
		datapoint := exponentialHistogramRecord.DataPoints().At(i)

		expHistogramDatapoint := ExponentialHistogramDatapoint{
			HistogramID:           histogram.HistogramID,
			ID:                    uuid.New().String(),
			StartTimeUnix:         datapoint.StartTimestamp().AsTime().UnixMilli(),
			TimeUnix:              datapoint.Timestamp().AsTime().UnixMilli(),
			Count:                 int64(datapoint.Count()),
			Sum:                   datapoint.Sum(),
			Min:                   datapoint.Min(),
			Max:                   datapoint.Max(),
			Flags:                 int(datapoint.Flags()),
			Scale:                 int(datapoint.Scale()),
			ZeroCount:             int64(datapoint.ZeroCount()),
			BucketsPositiveOffset: int(datapoint.Positive().Offset()),
			BucketsNegativeOffset: int(datapoint.Negative().Offset()),
		}
		kiExpHistogramRecord.histogramDatapoint = append(kiExpHistogramRecord.histogramDatapoint, expHistogramDatapoint)

		// Handle histogram datapoint attribute
		for k, v := range datapoint.Attributes().All() {
			if k == "" {
				e.logger.Debug("Sum record attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				datapointAttributes[k] = v
			} else {
				e.logger.Debug("Invalid sum record attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range datapointAttributes {
			vtPair := datapointAttributes[key]
			sa, err := e.newExponentialHistogramDatapointAttributeValue(histogram.HistogramID, expHistogramDatapoint.ID, key, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				datapointAttribute = append(datapointAttribute, *sa)
			}
		}
		kiExpHistogramRecord.histogramDatapointAttribute = append(kiExpHistogramRecord.histogramDatapointAttribute, datapointAttribute...)

		for k := range datapointAttributes {
			delete(datapointAttributes, k)
		}

		// Handle datapoint exemplars
		exemplars := datapoint.Exemplars()

		for i := 0; i < exemplars.Len(); i++ {
			exemplar := exemplars.At(i)
			sumDatapointExemplar := ExponentialHistogramDatapointExemplar{
				HistogramID:    histogram.HistogramID,
				DatapointID:    expHistogramDatapoint.ID,
				ExemplarID:     uuid.New().String(),
				TimeUnix:       exemplar.Timestamp().AsTime().UnixMilli(),
				HistogramValue: exemplar.DoubleValue(),
				TraceID:        exemplar.TraceID().String(),
				SpanID:         exemplar.SpanID().String(),
			}
			kiExpHistogramRecord.exemplars = append(kiExpHistogramRecord.exemplars, sumDatapointExemplar)

			// Handle Exemplar attribute
			for k, v := range exemplar.FilteredAttributes().All() {
				if k == "" {
					e.logger.Debug("Sum record attribute key is empty")
				} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
					exemplarAttributes[k] = v
				} else {
					e.logger.Debug("Invalid sum record attribute value", zap.String("Error", err.Error()))
					errs = append(errs, err)
				}
			}

			for key := range exemplarAttributes {
				vtPair := exemplarAttributes[key]
				ea, err := e.newExponentialHistogramDatapointExemplarAttributeValue(expHistogramDatapoint.HistogramID, expHistogramDatapoint.ID, sumDatapointExemplar.ExemplarID, key, vtPair)
				if err != nil {
					e.logger.Error(err.Error())
				} else {
					exemplarAttribute = append(exemplarAttribute, *ea)
				}
			}

			kiExpHistogramRecord.exemplarAttribute = append(kiExpHistogramRecord.exemplarAttribute, exemplarAttribute...)

			for k := range exemplarAttributes {
				delete(exemplarAttributes, k)
			}
		}

		// Handle positive and negative bucket counts
		for i := 0; i < datapoint.Positive().BucketCounts().Len(); i++ {
			positiveBucketCount := datapoint.Positive().BucketCounts().At(i)
			datapointBucketPositiveCount = append(datapointBucketPositiveCount, ExponentialHistogramBucketPositiveCount{
				HistogramID: expHistogramDatapoint.HistogramID,
				DatapointID: expHistogramDatapoint.ID,
				CountID:     uuid.New().String(),
				Count:       int64(positiveBucketCount),
			})
		}
		kiExpHistogramRecord.histogramBucketPositiveCount = append(kiExpHistogramRecord.histogramBucketPositiveCount, datapointBucketPositiveCount...)

		for i := 0; i < datapoint.Negative().BucketCounts().Len(); i++ {
			negativeBucketCount := datapoint.Negative().BucketCounts().At(i)
			datapointBucketNegativeCount = append(datapointBucketNegativeCount, ExponentialHistogramBucketNegativeCount{
				HistogramID: expHistogramDatapoint.HistogramID,
				DatapointID: expHistogramDatapoint.ID,
				CountID:     uuid.New().String(),
				Count:       negativeBucketCount,
			})
		}
		kiExpHistogramRecord.histogramBucketNegativeCount = append(kiExpHistogramRecord.histogramBucketNegativeCount, datapointBucketNegativeCount...)
	}

	// Handle Resource attribute
	var resourceAttribute []ExponentialHistogramResourceAttribute
	resourceAttributes := make(map[string]ValueTypePair)

	for k, v := range resAttr.All() {
		if k == "" {
			e.logger.Debug("Resource attribute key is empty")
		} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
			resourceAttributes[k] = v
		} else {
			e.logger.Debug("Invalid resource attribute value", zap.String("Error", err.Error()))
			errs = append(errs, err)
		}
	}

	for key := range resourceAttributes {
		vtPair := resourceAttributes[key]
		ga, err := e.newExponentialHistogramResourceAttributeValue(histogram.HistogramID, key, vtPair)
		if err != nil {
			e.logger.Error(err.Error())
		} else {
			resourceAttribute = append(resourceAttribute, *ga)
		}
	}

	copy(kiExpHistogramRecord.histogramResourceAttribute, resourceAttribute)

	// Handle Scope attribute
	var scopeAttribute []ExponentialHistogramScopeAttribute
	scopeAttributes := make(map[string]ValueTypePair)
	scopeName := scopeInstr.Name()
	scopeVersion := scopeInstr.Version()

	for k, v := range scopeInstr.Attributes().All() {
		if k == "" {
			e.logger.Debug("Scope attribute key is empty")
		} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
			scopeAttributes[k] = v
		} else {
			e.logger.Debug("Invalid scope attribute value", zap.String("Error", err.Error()))
			errs = append(errs, err)
		}
	}

	for key := range scopeAttributes {
		vtPair := scopeAttributes[key]
		sa, err := e.newExponentialHistogramScopeAttributeValue(histogram.HistogramID, key, scopeName, scopeVersion, vtPair)
		if err != nil {
			e.logger.Error(err.Error())
		} else {
			scopeAttribute = append(scopeAttribute, *sa)
		}
	}

	copy(kiExpHistogramRecord.histogramScopeAttribute, scopeAttribute)

	return kiExpHistogramRecord, multierr.Combine(errs...)
}

// createHistogramRecord - creates a Histogram type record
//
//	@receiver e
//	@param resAttr
//	@param _ schemaURL - unused
//	@param scopeInstr
//	@param _ scopeURL - unused
//	@param histogramRecord
//	@param name
//	@param description
//	@param unit
//	@return *kineticaHistogramRecord
//	@return error
func (e *kineticaMetricsExporter) createHistogramRecord(resAttr pcommon.Map, _ string, scopeInstr pcommon.InstrumentationScope, _ string, histogramRecord pmetric.Histogram, name, description, unit string) (*kineticaHistogramRecord, error) {
	e.logger.Debug("In createHistogramRecord ...")

	var errs []error

	kiHistogramRecord := new(kineticaHistogramRecord)

	histogram := &Histogram{
		HistogramID:            uuid.New().String(),
		MetricName:             name,
		Description:            description,
		Unit:                   unit,
		AggregationTemporality: histogramRecord.AggregationTemporality(),
	}

	kiHistogramRecord.histogram = histogram

	// Handle data points
	var datapointAttribute []HistogramDataPointAttribute
	datapointAttributes := make(map[string]ValueTypePair)

	var exemplarAttribute []HistogramDataPointExemplarAttribute
	exemplarAttributes := make(map[string]ValueTypePair)

	// Handle data points
	for i := 0; i < histogramRecord.DataPoints().Len(); i++ {
		datapoint := histogramRecord.DataPoints().At(i)

		histogramDatapoint := &HistogramDatapoint{
			HistogramID:   histogram.HistogramID,
			ID:            uuid.New().String(),
			StartTimeUnix: datapoint.StartTimestamp().AsTime().UnixMilli(),
			TimeUnix:      datapoint.Timestamp().AsTime().UnixMilli(),
			Count:         int64(datapoint.Count()),
			Sum:           datapoint.Sum(),
			Min:           datapoint.Min(),
			Max:           datapoint.Max(),
			Flags:         int(datapoint.Flags()),
		}
		kiHistogramRecord.histogramDatapoint = append(kiHistogramRecord.histogramDatapoint, *histogramDatapoint)

		// Handle histogram datapoint attribute
		for k, v := range datapoint.Attributes().All() {
			if k == "" {
				e.logger.Debug("Histogram record attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				datapointAttributes[k] = v
			} else {
				e.logger.Debug("Invalid histogram record attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range datapointAttributes {
			vtPair := datapointAttributes[key]
			sa, err := e.newHistogramDatapointAttributeValue(histogram.HistogramID, histogramDatapoint.ID, key, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				datapointAttribute = append(datapointAttribute, *sa)
			}
		}
		kiHistogramRecord.histogramDatapointAttribute = append(kiHistogramRecord.histogramDatapointAttribute, datapointAttribute...)

		for k := range datapointAttributes {
			delete(datapointAttributes, k)
		}

		// Handle data point exemplars
		exemplars := datapoint.Exemplars()

		for i := 0; i < exemplars.Len(); i++ {
			exemplar := exemplars.At(i)
			histogramDatapointExemplar := HistogramDatapointExemplar{
				HistogramID:    histogram.HistogramID,
				DatapointID:    histogramDatapoint.ID,
				ExemplarID:     uuid.New().String(),
				TimeUnix:       exemplar.Timestamp().AsTime().UnixMilli(),
				HistogramValue: exemplar.DoubleValue(),
				TraceID:        exemplar.TraceID().String(),
				SpanID:         exemplar.SpanID().String(),
			}
			kiHistogramRecord.exemplars = append(kiHistogramRecord.exemplars, histogramDatapointExemplar)

			// Handle Exemplar attribute
			for k, v := range exemplar.FilteredAttributes().All() {
				if k == "" {
					e.logger.Debug("Histogram record attribute key is empty")
				} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
					exemplarAttributes[k] = v
				} else {
					e.logger.Debug("Invalid histogram record attribute value", zap.String("Error", err.Error()))
					errs = append(errs, err)
				}
			}

			for key := range exemplarAttributes {
				vtPair := exemplarAttributes[key]
				ea, err := e.newHistogramDatapointExemplarAttributeValue(histogramDatapoint.HistogramID, histogramDatapoint.ID, histogramDatapointExemplar.ExemplarID, key, vtPair)
				if err != nil {
					e.logger.Error(err.Error())
				} else {
					exemplarAttribute = append(exemplarAttribute, *ea)
				}
			}

			kiHistogramRecord.exemplarAttribute = append(kiHistogramRecord.exemplarAttribute, exemplarAttribute...)

			for k := range exemplarAttributes {
				delete(exemplarAttributes, k)
			}
		}

		histogramBucketCounts := datapoint.BucketCounts()
		for i := 0; i < histogramBucketCounts.Len(); i++ {
			bucketCount := HistogramDatapointBucketCount{
				HistogramID: histogramDatapoint.HistogramID,
				DatapointID: histogramDatapoint.ID,
				CountID:     uuid.New().String(),
				Count:       int64(histogramBucketCounts.At(i)),
			}
			kiHistogramRecord.histogramBucketCount = append(kiHistogramRecord.histogramBucketCount, bucketCount)
		}

		histogramExplicitBounds := datapoint.ExplicitBounds()
		for i := 0; i < histogramExplicitBounds.Len(); i++ {
			explicitBound := HistogramDatapointExplicitBound{
				HistogramID:   histogramDatapoint.HistogramID,
				DatapointID:   histogramDatapoint.ID,
				BoundID:       uuid.New().String(),
				ExplicitBound: histogramExplicitBounds.At(i),
			}
			kiHistogramRecord.histogramExplicitBound = append(kiHistogramRecord.histogramExplicitBound, explicitBound)
		}
	}

	// Handle Resource attribute
	var resourceAttribute []HistogramResourceAttribute
	resourceAttributes := make(map[string]ValueTypePair)

	if resAttr.Len() > 0 {
		for k, v := range resAttr.All() {
			if k == "" {
				e.logger.Debug("Resource attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				resourceAttributes[k] = v
			} else {
				e.logger.Debug("Invalid resource attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range resourceAttributes {
			vtPair := resourceAttributes[key]
			ga, err := e.newHistogramResourceAttributeValue(histogram.HistogramID, key, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				resourceAttribute = append(resourceAttribute, *ga)
			}
		}

		kiHistogramRecord.histogramResourceAttribute = make([]HistogramResourceAttribute, len(resourceAttribute))
		copy(kiHistogramRecord.histogramResourceAttribute, resourceAttribute)
	}

	// Handle Scope attribute
	var scopeAttribute []HistogramScopeAttribute
	scopeAttributes := make(map[string]ValueTypePair)
	scopeName := scopeInstr.Name()
	scopeVersion := scopeInstr.Version()

	if scopeInstr.Attributes().Len() > 0 {
		for k, v := range scopeInstr.Attributes().All() {
			if k == "" {
				e.logger.Debug("Scope attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				scopeAttributes[k] = v
			} else {
				e.logger.Debug("Invalid scope attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range scopeAttributes {
			vtPair := scopeAttributes[key]
			sa, err := e.newHistogramScopeAttributeValue(histogram.HistogramID, key, scopeName, scopeVersion, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				scopeAttribute = append(scopeAttribute, *sa)
			}
		}

		kiHistogramRecord.histogramScopeAttribute = make([]HistogramScopeAttribute, len(scopeAttribute))
		copy(kiHistogramRecord.histogramScopeAttribute, scopeAttribute)
	}

	return kiHistogramRecord, multierr.Combine(errs...)
}

// createSumRecord - creates a SUM type record
//
//	@receiver e
//	@param resAttr
//	@param schemaURL
//	@param scopeInstr
//	@param scopeURL
//	@param sumRecord
//	@param name
//	@param description
//	@param unit
//	@return *kineticaSumRecord
//	@return error
func (e *kineticaMetricsExporter) createSumRecord(resAttr pcommon.Map, _ string, scopeInstr pcommon.InstrumentationScope, _ string, sumRecord pmetric.Sum, name, description, unit string) (*kineticaSumRecord, error) {
	var errs []error

	kiSumRecord := new(kineticaSumRecord)
	var isMonotonic int8
	if sumRecord.IsMonotonic() {
		isMonotonic = 1
	}

	sum := &Sum{
		SumID:                  uuid.New().String(),
		MetricName:             name,
		Description:            description,
		Unit:                   unit,
		AggregationTemporality: sumRecord.AggregationTemporality(),
		IsMonotonic:            isMonotonic,
	}

	kiSumRecord.sum = sum

	// Handle data points
	var sumDatapointAttribute []SumDataPointAttribute
	sumDatapointAttributes := make(map[string]ValueTypePair)

	var exemplarAttribute []SumDataPointExemplarAttribute
	exemplarAttributes := make(map[string]ValueTypePair)

	for i := 0; i < sumRecord.DataPoints().Len(); i++ {
		datapoint := sumRecord.DataPoints().At(i)

		sumDatapoint := SumDatapoint{
			SumID:         sum.SumID,
			ID:            uuid.New().String(),
			StartTimeUnix: datapoint.StartTimestamp().AsTime().UnixMilli(),
			TimeUnix:      datapoint.Timestamp().AsTime().UnixMilli(),
			SumValue:      datapoint.DoubleValue(),
			Flags:         int(datapoint.Flags()),
		}
		kiSumRecord.datapoint = append(kiSumRecord.datapoint, sumDatapoint)

		// Handle Sum attribute
		for k, v := range datapoint.Attributes().All() {
			if k == "" {
				e.logger.Debug("Sum record attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				sumDatapointAttributes[k] = v
			} else {
				e.logger.Debug("Invalid sum record attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range sumDatapointAttributes {
			vtPair := sumDatapointAttributes[key]
			sa, err := e.newSumDatapointAttributeValue(sum.SumID, sumDatapoint.ID, key, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				sumDatapointAttribute = append(sumDatapointAttribute, *sa)
			}
		}
		kiSumRecord.datapointAttribute = append(kiSumRecord.datapointAttribute, sumDatapointAttribute...)

		for k := range sumDatapointAttributes {
			delete(sumDatapointAttributes, k)
		}

		// Handle data point exemplars
		exemplars := datapoint.Exemplars()

		for i := 0; i < exemplars.Len(); i++ {
			exemplar := exemplars.At(i)
			sumDatapointExemplar := SumDatapointExemplar{
				SumID:       sum.SumID,
				DatapointID: sumDatapoint.ID,
				ExemplarID:  uuid.New().String(),
				TimeUnix:    exemplar.Timestamp().AsTime().UnixMilli(),
				SumValue:    exemplar.DoubleValue(),
				TraceID:     exemplar.TraceID().String(),
				SpanID:      exemplar.SpanID().String(),
			}
			kiSumRecord.exemplars = append(kiSumRecord.exemplars, sumDatapointExemplar)

			// Handle Exemplar attribute
			for k, v := range exemplar.FilteredAttributes().All() {
				if k == "" {
					e.logger.Debug("Sum record attribute key is empty")
				} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
					exemplarAttributes[k] = v
				} else {
					e.logger.Debug("Invalid sum record attribute value", zap.String("Error", err.Error()))
					errs = append(errs, err)
				}
			}

			for key := range exemplarAttributes {
				vtPair := exemplarAttributes[key]
				ea, err := e.newSumDatapointExemplarAttributeValue(sum.SumID, sumDatapoint.ID, sumDatapointExemplar.ExemplarID, key, vtPair)
				if err != nil {
					e.logger.Error(err.Error())
				} else {
					exemplarAttribute = append(exemplarAttribute, *ea)
				}
			}

			kiSumRecord.exemplarAttribute = append(kiSumRecord.exemplarAttribute, exemplarAttribute...)

			for k := range exemplarAttributes {
				delete(exemplarAttributes, k)
			}
		}
	}

	// Handle Resource attribute
	var resourceAttribute []SumResourceAttribute
	resourceAttributes := make(map[string]ValueTypePair)

	if resAttr.Len() > 0 {
		for k, v := range resAttr.All() {
			if k == "" {
				e.logger.Debug("Resource attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				resourceAttributes[k] = v
			} else {
				e.logger.Debug("Invalid resource attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range resourceAttributes {
			vtPair := resourceAttributes[key]
			ga, err := e.newSumResourceAttributeValue(sum.SumID, key, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				resourceAttribute = append(resourceAttribute, *ga)
			}
		}

		kiSumRecord.sumResourceAttribute = make([]SumResourceAttribute, len(resourceAttribute))
		copy(kiSumRecord.sumResourceAttribute, resourceAttribute)
	}

	// Handle Scope attribute
	var scopeAttribute []SumScopeAttribute
	scopeAttributes := make(map[string]ValueTypePair)
	scopeName := scopeInstr.Name()
	scopeVersion := scopeInstr.Version()

	if scopeInstr.Attributes().Len() > 0 {
		for k, v := range scopeInstr.Attributes().All() {
			if k == "" {
				e.logger.Debug("Scope attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				scopeAttributes[k] = v
			} else {
				e.logger.Debug("Invalid scope attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range scopeAttributes {
			vtPair := scopeAttributes[key]
			sa, err := e.newSumScopeAttributeValue(sum.SumID, key, scopeName, scopeVersion, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				scopeAttribute = append(scopeAttribute, *sa)
			}
		}

		copy(kiSumRecord.sumScopeAttribute, scopeAttribute)
	} else {
		// No attributes found - just basic scope
		kiSumRecord.sumScopeAttribute = append(kiSumRecord.sumScopeAttribute, SumScopeAttribute{
			SumID:          sum.SumID,
			ScopeName:      scopeName,
			ScopeVersion:   scopeVersion,
			Key:            "",
			AttributeValue: AttributeValue{},
		})
	}

	return kiSumRecord, multierr.Combine(errs...)
}

// createGaugeRecord - creates a Gauge type record
//
//	@receiver e
//	@param resAttr
//	@param _ schemaURL unused
//	@param scopeInstr
//	@param _ scopeURL unused
//	@param gaugeRecord
//	@param name
//	@param description
//	@param unit
//	@return *kineticaGaugeRecord
//	@return error
func (e *kineticaMetricsExporter) createGaugeRecord(resAttr pcommon.Map, _ string, scopeInstr pcommon.InstrumentationScope, _ string, gaugeRecord pmetric.Gauge, name, description, unit string) (*kineticaGaugeRecord, error) {
	var errs []error

	kiGaugeRecord := new(kineticaGaugeRecord)

	gauge := &Gauge{
		GaugeID:     uuid.New().String(),
		MetricName:  name,
		Description: description,
		Unit:        unit,
	}

	kiGaugeRecord.gauge = gauge

	// Handle data points
	var gaugeDatapointAttribute []GaugeDatapointAttribute
	gaugeDatapointAttributes := make(map[string]ValueTypePair)

	var exemplarAttribute []GaugeDataPointExemplarAttribute
	exemplarAttributes := make(map[string]ValueTypePair)

	for i := 0; i < gaugeRecord.DataPoints().Len(); i++ {
		datapoint := gaugeRecord.DataPoints().At(i)

		gaugeDatapoint := GaugeDatapoint{
			GaugeID:       gauge.GaugeID,
			ID:            uuid.New().String(),
			StartTimeUnix: datapoint.StartTimestamp().AsTime().UnixMilli(),
			TimeUnix:      datapoint.Timestamp().AsTime().UnixMilli(),
			GaugeValue:    datapoint.DoubleValue(),
			Flags:         int(datapoint.Flags()),
		}
		kiGaugeRecord.datapoint = append(kiGaugeRecord.datapoint, gaugeDatapoint)

		for k, v := range datapoint.Attributes().All() {
			if k == "" {
				e.logger.Debug("Gauge record attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				gaugeDatapointAttributes[k] = v
			} else {
				e.logger.Debug("Invalid gauge record attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		for key := range gaugeDatapointAttributes {
			vtPair := gaugeDatapointAttributes[key]
			ga, err := e.newGaugeDatapointAttributeValue(gauge.GaugeID, gaugeDatapoint.ID, key, vtPair)
			if err != nil {
				e.logger.Error(err.Error())
			} else {
				gaugeDatapointAttribute = append(gaugeDatapointAttribute, *ga)
			}
		}

		kiGaugeRecord.datapointAttribute = append(kiGaugeRecord.datapointAttribute, gaugeDatapointAttribute...)

		for k := range gaugeDatapointAttributes {
			delete(gaugeDatapointAttributes, k)
		}

		// Handle data point exemplars
		exemplars := datapoint.Exemplars()
		for i := 0; i < exemplars.Len(); i++ {
			exemplar := exemplars.At(i)
			gaugeDatapointExemplar := GaugeDatapointExemplar{
				GaugeID:     gauge.GaugeID,
				DatapointID: gaugeDatapoint.ID,
				ExemplarID:  uuid.New().String(),
				TimeUnix:    exemplar.Timestamp().AsTime().UnixMilli(),
				GaugeValue:  exemplar.DoubleValue(),
				TraceID:     exemplar.TraceID().String(),
				SpanID:      exemplar.SpanID().String(),
			}
			kiGaugeRecord.exemplars = append(kiGaugeRecord.exemplars, gaugeDatapointExemplar)

			// Handle Exemplar attribute
			for k, v := range exemplar.FilteredAttributes().All() {
				if k == "" {
					e.logger.Debug("Sum record attribute key is empty")
				} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
					exemplarAttributes[k] = v
				} else {
					e.logger.Debug("Invalid sum record attribute value", zap.String("Error", err.Error()))
					errs = append(errs, err)
				}
			}

			for key := range exemplarAttributes {
				vtPair := exemplarAttributes[key]
				ea, err := e.newGaugeDatapointExemplarAttributeValue(gauge.GaugeID, gaugeDatapoint.ID, gaugeDatapointExemplar.ExemplarID, key, vtPair)
				if err != nil {
					e.logger.Error(err.Error())
				} else {
					exemplarAttribute = append(exemplarAttribute, *ea)
				}
			}

			kiGaugeRecord.exemplarAttribute = append(kiGaugeRecord.exemplarAttribute, exemplarAttribute...)

			for k := range exemplarAttributes {
				delete(exemplarAttributes, k)
			}
		}
	}

	// Handle Resource attribute
	e.logger.Debug("Resource Attributes received ->", zap.Any("Attributes", resAttr))

	var resourceAttribute []GaugeResourceAttribute
	resourceAttributes := make(map[string]ValueTypePair)

	if resAttr.Len() > 0 {
		for k, v := range resAttr.All() {
			if k == "" {
				e.logger.Debug("Resource attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				resourceAttributes[k] = v
			} else {
				e.logger.Debug("Invalid resource attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		e.logger.Debug("Resource Attributes to be added ->", zap.Any("Attributes", resourceAttributes))
		for key := range resourceAttributes {
			vtPair := resourceAttributes[key]
			ga, err := e.newGaugeResourceAttributeValue(gauge.GaugeID, key, vtPair)

			e.logger.Debug("New resource attribute ->", zap.Any("Attribute", ga))

			if err != nil {
				e.logger.Error(err.Error())
			} else {
				resourceAttribute = append(resourceAttribute, *ga)
			}
		}

		kiGaugeRecord.resourceAttribute = make([]GaugeResourceAttribute, len(resourceAttribute))
		copy(kiGaugeRecord.resourceAttribute, resourceAttribute)
		e.logger.Debug("Resource Attributes actually added ->", zap.Any("Attributes", kiGaugeRecord.resourceAttribute))
	}

	// Handle Scope attribute
	e.logger.Debug("Scope Attributes received ->", zap.Any("Attributes", scopeInstr.Attributes()))

	var scopeAttribute []GaugeScopeAttribute
	scopeAttributes := make(map[string]ValueTypePair)
	scopeName := scopeInstr.Name()
	scopeVersion := scopeInstr.Version()

	if scopeInstr.Attributes().Len() > 0 {
		for k, v := range scopeInstr.Attributes().All() {
			if k == "" {
				e.logger.Debug("Scope attribute key is empty")
			} else if v, err := attributeValueToKineticaFieldValue(v); err == nil {
				scopeAttributes[k] = v
			} else {
				e.logger.Debug("Invalid scope attribute value", zap.String("Error", err.Error()))
				errs = append(errs, err)
			}
		}

		e.logger.Debug("Scope Attributes to be added ->", zap.Any("Attributes", scopeAttributes))
		for key := range scopeAttributes {
			vtPair := scopeAttributes[key]
			ga, err := e.newGaugeScopeAttributeValue(gauge.GaugeID, key, scopeName, scopeVersion, vtPair)

			e.logger.Debug("New scope attribute ->", zap.Any("Attribute", ga))

			if err != nil {
				e.logger.Error(err.Error())
			} else {
				scopeAttribute = append(scopeAttribute, *ga)
			}
		}

		kiGaugeRecord.scopeAttribute = make([]GaugeScopeAttribute, len(scopeAttribute))
		copy(kiGaugeRecord.scopeAttribute, scopeAttribute)
		e.logger.Debug("Scope Attributes actually added ->", zap.Any("Attributes", kiGaugeRecord.scopeAttribute))
	} else {
		// No attributes found - just basic scope
		kiGaugeRecord.scopeAttribute = append(kiGaugeRecord.scopeAttribute, GaugeScopeAttribute{
			GaugeID:        gauge.GaugeID,
			ScopeName:      scopeName,
			ScopeVersion:   scopeVersion,
			Key:            "",
			AttributeValue: AttributeValue{},
		})
	}

	return kiGaugeRecord, multierr.Combine(errs...)
}

// Utility functions
func (e *kineticaMetricsExporter) newGaugeResourceAttributeValue(gaugeID string, key string, vtPair ValueTypePair) (*GaugeResourceAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ra := &GaugeResourceAttribute{gaugeID, key, *av}
	return ra, nil
}

func (e *kineticaMetricsExporter) newGaugeDatapointAttributeValue(gaugeID string, datapointID string, key string, vtPair ValueTypePair) (*GaugeDatapointAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ga := &GaugeDatapointAttribute{gaugeID, datapointID, key, *av}
	return ga, nil
}

func (e *kineticaMetricsExporter) newGaugeScopeAttributeValue(gaugeID string, key string, scopeName string, scopeVersion string, vtPair ValueTypePair) (*GaugeScopeAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &GaugeScopeAttribute{gaugeID, key, scopeName, scopeVersion, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newSumDatapointAttributeValue(sumID string, datapointID string, key string, vtPair ValueTypePair) (*SumDataPointAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ga := &SumDataPointAttribute{sumID, datapointID, key, *av}
	return ga, nil
}

func (e *kineticaMetricsExporter) newSumResourceAttributeValue(sumID string, key string, vtPair ValueTypePair) (*SumResourceAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ra := &SumResourceAttribute{sumID, key, *av}
	return ra, nil
}

func (e *kineticaMetricsExporter) newSumScopeAttributeValue(sumID string, key string, scopeName string, scopeVersion string, vtPair ValueTypePair) (*SumScopeAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &SumScopeAttribute{sumID, key, scopeName, scopeVersion, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newSumDatapointExemplarAttributeValue(sumID string, sumDatapointID string, sumDatapointExemplarID string, key string, vtPair ValueTypePair) (*SumDataPointExemplarAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &SumDataPointExemplarAttribute{sumID, sumDatapointID, sumDatapointExemplarID, key, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newExponentialHistogramDatapointExemplarAttributeValue(histogramID string, histogramDatapointID string, histogramDatapointExemplarID string, key string, vtPair ValueTypePair) (*ExponentialHistogramDataPointExemplarAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &ExponentialHistogramDataPointExemplarAttribute{histogramID, histogramDatapointID, histogramDatapointExemplarID, key, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newExponentialHistogramDatapointAttributeValue(histogramID string, datapointID string, key string, vtPair ValueTypePair) (*ExponentialHistogramDataPointAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ga := &ExponentialHistogramDataPointAttribute{histogramID, datapointID, key, *av}
	return ga, nil
}

func (e *kineticaMetricsExporter) newHistogramDatapointExemplarAttributeValue(histogramID string, datapointID string, exemplarID string, key string, vtPair ValueTypePair) (*HistogramDataPointExemplarAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ga := &HistogramDataPointExemplarAttribute{histogramID, datapointID, exemplarID, key, *av}
	return ga, nil
}

func (e *kineticaMetricsExporter) newGaugeDatapointExemplarAttributeValue(gaugeID string, gaugeDatapointID string, gaugeDatapointExemplarID string, key string, vtPair ValueTypePair) (*GaugeDataPointExemplarAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &GaugeDataPointExemplarAttribute{gaugeID, gaugeDatapointID, gaugeDatapointExemplarID, key, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newHistogramResourceAttributeValue(histogramID string, key string, vtPair ValueTypePair) (*HistogramResourceAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ra := &HistogramResourceAttribute{histogramID, key, *av}
	return ra, nil
}

func (e *kineticaMetricsExporter) newHistogramScopeAttributeValue(histogramID string, key string, scopeName string, scopeVersion string, vtPair ValueTypePair) (*HistogramScopeAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &HistogramScopeAttribute{histogramID, key, scopeName, scopeVersion, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newExponentialHistogramResourceAttributeValue(histogramID string, key string, vtPair ValueTypePair) (*ExponentialHistogramResourceAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ra := &ExponentialHistogramResourceAttribute{histogramID, key, *av}
	return ra, nil
}

func (e *kineticaMetricsExporter) newExponentialHistogramScopeAttributeValue(histogramID string, key string, scopeName string, scopeVersion string, vtPair ValueTypePair) (*ExponentialHistogramScopeAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &ExponentialHistogramScopeAttribute{histogramID, key, scopeName, scopeVersion, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newSummaryResourceAttributeValue(summaryID string, key string, vtPair ValueTypePair) (*SummaryResourceAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ra := &SummaryResourceAttribute{summaryID, key, *av}
	return ra, nil
}

func (e *kineticaMetricsExporter) newSummaryScopeAttributeValue(summaryID string, key string, scopeName string, scopeVersion string, vtPair ValueTypePair) (*SummaryScopeAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &SummaryScopeAttribute{summaryID, key, scopeName, scopeVersion, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newSummaryDatapointAttributeValue(summaryID string, summaryDatapointID string, key string, vtPair ValueTypePair) (*SummaryDataPointAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	sa := &SummaryDataPointAttribute{summaryID, summaryDatapointID, key, *av}
	return sa, nil
}

func (e *kineticaMetricsExporter) newHistogramDatapointAttributeValue(histogramID string, datapointID string, key string, vtPair ValueTypePair) (*HistogramDataPointAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)
	if err != nil {
		return nil, err
	}

	ga := &HistogramDataPointAttribute{histogramID, datapointID, key, *av}
	return ga, nil
}
