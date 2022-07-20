package azuredataexplorerexporter

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

/*
A converter package that converts and marshals data to be written to ADX metrics tables
*/
const (
	hostkey = "host.name"
	// Indicates the sum that is used in both summary and in histogram
	sumsuffix = "sum"
	// Count used in summary , histogram and also in exponential histogram
	countsuffix = "count"
	// Indicates the sum that is used in both summary and in histogram
	sumdescription = "(Sum total of samples)"
	// Count used in summary , histogram and also in exponential histogram
	countdescription = "(Count of samples)"
)

// This is derived from the specification https://opentelemetry.io/docs/reference/specification/metrics/datamodel/
type AdxMetric struct {
	Timestamp string //The timestamp of the occurance. A metric is measured at a point of time. Formatted into string as RFC3339
	//Including name, the Metric object is defined by the following properties:
	MetricName        string                 //Name of the metric field
	MetricType        string                 // The data point type (e.g. Sum, Gauge, Histogram ExponentialHistogram, Summary)
	MetricUnit        string                 // The metric stream’s unit
	MetricDescription string                 //The metric stream’s description
	MetricValue       float64                // the value of the metric
	MetricAttributes  map[string]interface{} // JSON attributes that can then be parsed. Extrinsic properties
	//Additional properties
	Host               string                 // the hostname for analysis of the metric. Extracted from https://opentelemetry.io/docs/reference/specification/resource/semantic_conventions/host/
	ResourceAttributes map[string]interface{} // The originating Resource attributes. Refer https://opentelemetry.io/docs/reference/specification/resource/sdk/
}

/*
	Convert the pMetric to the type ADXMetric , this matches the scheme in the OTELMetric table in the database
*/

func mapToAdxMetric(res pcommon.Resource, md pmetric.Metric, scopeattrs map[string]interface{}, logger *zap.Logger) []*AdxMetric {
	logger.Debug("Entering processing of toAdxMetric function")
	// default to collectors host name. Ignore the error here. This should not cause the failure of the process
	host, err := os.Hostname()
	if err != nil {
		logger.Warn("Default collector hostname could not be retrieved", zap.Error(err))
	}
	resourceAttrs := res.Attributes().AsRaw()
	if h := resourceAttrs[hostkey]; h != nil {
		host = h.(string)
	}
	createMetric := func(times time.Time, attr pcommon.Map, value func() float64, name string, desc string, mt pmetric.MetricDataType) *AdxMetric {
		clonedScopedAttributes := cloneMap(scopeattrs)
		copyMap(clonedScopedAttributes, attr.AsRaw())
		if isEmpty(name) {
			name = md.Name()
		}
		if isEmpty(desc) {
			desc = md.Description()
		}
		return &AdxMetric{
			Timestamp:          times.Format(time.RFC3339),
			MetricName:         name,
			MetricType:         mt.String(),
			MetricUnit:         md.Unit(),
			MetricDescription:  desc,
			MetricValue:        value(),
			MetricAttributes:   clonedScopedAttributes,
			Host:               host,
			ResourceAttributes: resourceAttrs,
		}
	}
	switch md.DataType() {
	case pmetric.MetricDataTypeGauge:
		dataPoints := md.Gauge().DataPoints()
		adxMetrics := make([]*AdxMetric, dataPoints.Len())
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			adxMetrics[gi] = createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
				var metricValue float64
				switch dataPoint.ValueType() {
				case pmetric.NumberDataPointValueTypeInt:
					metricValue = float64(dataPoint.IntVal())
				case pmetric.NumberDataPointValueTypeDouble:
					metricValue = float64(dataPoint.DoubleVal())
				}
				return metricValue
			}, "", "", pmetric.MetricDataTypeGauge)
		}
		return adxMetrics
	case pmetric.MetricDataTypeHistogram:
		dataPoints := md.Histogram().DataPoints()
		var adxMetrics []*AdxMetric
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			bounds := dataPoint.MExplicitBounds()
			counts := dataPoint.MBucketCounts()
			// first, add one event for sum, and one for count
			{
				adxMetrics = append(adxMetrics,
					createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
						return dataPoint.Sum()
					},
						fmt.Sprintf("%s_%s", md.Name(), sumsuffix),
						fmt.Sprintf("%s%s", md.Description(), sumdescription),
						pmetric.MetricDataTypeHistogram))
			}
			{
				adxMetrics = append(adxMetrics,
					createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
						// Change int to float. The value is a float64 in the table
						return float64(dataPoint.Count())
					},
						fmt.Sprintf("%s_%s", md.Name(), countsuffix),
						fmt.Sprintf("%s%s", md.Description(), countdescription),
						pmetric.MetricDataTypeHistogram))
			}
			// Spec says counts is optional but if present it must have one more
			// element than the bounds array.
			if len(counts) == 0 || len(counts) != len(bounds)+1 {
				continue
			}
			value := uint64(0)
			// now create buckets for each bound.
			for bi := 0; bi < len(bounds); bi++ {
				var customMap pcommon.Map
				copyMap(customMap.AsRaw(), dataPoint.Attributes().AsRaw())
				// Add the LE field for the bucket's bound
				customMap.InsertString("le", float64ToDimValue(bounds[bi]))
				value += counts[bi]

				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(), customMap, func() float64 {
					// Change int to float. The value is a float64 in the table
					return float64(value)
				},
					fmt.Sprintf("%s_bucket", md.Name()),
					"",
					pmetric.MetricDataTypeHistogram))
			}
			// add an upper bound for +Inf
			{

				var customMap pcommon.Map
				copyMap(customMap.AsRaw(), dataPoint.Attributes().AsRaw())
				// Add the LE field for the bucket's bound
				customMap.InsertString("le", float64ToDimValue(math.Inf(1)))

				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(), customMap, func() float64 {
					// Change int to float. The value is a float64 in the table
					return float64(value + counts[len(counts)-1])
				},
					fmt.Sprintf("%s_bucket", md.Name()),
					"",
					pmetric.MetricDataTypeHistogram))
			}
		}
		return adxMetrics
	case pmetric.MetricDataTypeSum:
		dataPoints := md.Sum().DataPoints()
		adxMetrics := make([]*AdxMetric, dataPoints.Len())
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			adxMetrics[gi] = createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
				var metricValue float64
				switch dataPoint.ValueType() {
				case pmetric.NumberDataPointValueTypeInt:
					metricValue = float64(dataPoint.IntVal())
				case pmetric.NumberDataPointValueTypeDouble:
					metricValue = float64(dataPoint.DoubleVal())
				}
				return metricValue
			}, "", "", pmetric.MetricDataTypeSum)

		}
		return adxMetrics
	case pmetric.MetricDataTypeSummary:
		dataPoints := md.Summary().DataPoints()
		var adxMetrics []*AdxMetric
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			// first, add one event for sum, and one for count
			{
				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(), dataPoint.Attributes(), func() float64 {
					return float64(dataPoint.Sum())
				},
					fmt.Sprintf("%s_%s", md.Name(), sumsuffix),
					fmt.Sprintf("%s%s", md.Description(), sumdescription),
					pmetric.MetricDataTypeSummary))
			}
			//counts
			{
				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(),
					dataPoint.Attributes(),
					func() float64 {
						return float64(dataPoint.Count())
					},
					fmt.Sprintf("%s_%s", md.Name(), countsuffix),
					fmt.Sprintf("%s%s", md.Description(), countdescription),
					pmetric.MetricDataTypeSummary))

			}
			// now create values for each quantile.
			for bi := 0; bi < dataPoint.QuantileValues().Len(); bi++ {
				dp := dataPoint.QuantileValues().At(bi)
				var customMap pcommon.Map
				copyMap(customMap.AsRaw(), dataPoint.Attributes().AsRaw())
				customMap.InsertString("qt", float64ToDimValue(dp.Quantile()))
				quantileName := fmt.Sprintf("%s_%s", md.Name(), strconv.FormatFloat(dp.Quantile(), 'f', -1, 64))
				customMap.InsertDouble(quantileName, sanitizeFloat(dp.Value()).(float64))
				adxMetrics = append(adxMetrics, createMetric(dataPoint.Timestamp().AsTime(),
					customMap,
					func() float64 {
						return dp.Value()
					},
					quantileName,
					fmt.Sprintf("%s%s", md.Description(), countdescription),
					pmetric.MetricDataTypeSummary))
			}
		}
		return adxMetrics
	case pmetric.MetricDataTypeNone:
		fallthrough
	default:
		logger.Warn(
			"Unsupported metric type : ",
			zap.Any("metric", md))
		return nil
	}
}

// Given all the metrics , transform that to the representative structure
func rawMetricsToAdxMetrics(_ context.Context, metrics pmetric.Metrics, logger *zap.Logger) ([]*AdxMetric, error) {
	var transformedAdxMetrics []*AdxMetric
	resourceMetric := metrics.ResourceMetrics()
	for i := 0; i < resourceMetric.Len(); i++ {
		res := resourceMetric.At(i).Resource()
		scopeMetrics := resourceMetric.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			scopeMetric := scopeMetrics.At(j)
			metrics := scopeMetric.Metrics()
			// get details of the scope from the scope metric
			scopeAttr := getScopeMap(scopeMetric.Scope())
			for k := 0; k < metrics.Len(); k++ {
				transformedAdxMetrics = append(transformedAdxMetrics, mapToAdxMetric(res, metrics.At(k), scopeAttr, logger)...)
			}
		}
	}
	return transformedAdxMetrics, nil
}

func copyMap(toAttrib map[string]interface{}, fromAttrib map[string]interface{}) {
	for k, v := range fromAttrib {
		toAttrib[k] = v
	}
}

func cloneMap(fields map[string]interface{}) map[string]interface{} {
	newFields := make(map[string]interface{}, len(fields))
	copyMap(newFields, fields)
	return newFields
}

func float64ToDimValue(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func sanitizeFloat(value float64) interface{} {
	if math.IsNaN(value) {
		return math.NaN()
	}
	if math.IsInf(value, 1) {
		return math.Inf(1)
	}
	if math.IsInf(value, -1) {
		return math.Inf(-1)
	}
	return value
}
