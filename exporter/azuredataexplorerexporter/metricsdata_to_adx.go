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
	hostkey = "host.key"
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
	Convert the pMetric to the type ADXMetric , this matches the scheme in the RawMetric table in the database
*/

func mapToAdxMetric(res pcommon.Resource, md pmetric.Metric, scopeattrs map[string]interface{}, logger *zap.Logger) []*AdxMetric {
	logger.Debug("Entering processing of toAdxMetric function")
	// default to collectors host name. Ignore the error here. This should not cause the failure of the process
	host, _ := os.Hostname()
	commonFields := map[string]interface{}{}
	resourceattrs := res.Attributes().AsRaw()
	res.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case hostkey:
			host = v.StringVal()
		default:
			commonFields[k] = v.AsString()
		}
		return true
	})
	switch md.DataType() {
	case pmetric.MetricDataTypeGauge:
		dataPoints := md.Gauge().DataPoints()
		adxMetrics := make([]*AdxMetric, dataPoints.Len())
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			clonedscopeattrs := cloneMap(scopeattrs)
			copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())
			var metricValue float64
			switch dataPoint.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				metricValue = float64(dataPoint.IntVal())
			case pmetric.NumberDataPointValueTypeDouble:
				metricValue = float64(dataPoint.DoubleVal())
			}
			adxMetrics[gi] = &AdxMetric{
				Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
				MetricName:         md.Name(),
				MetricType:         pmetric.MetricDataTypeGauge.String(),
				MetricUnit:         md.Unit(),
				MetricDescription:  md.Description(),
				MetricValue:        metricValue,
				MetricAttributes:   clonedscopeattrs,
				Host:               host,
				ResourceAttributes: resourceattrs,
			}

		}
		return adxMetrics
	case pmetric.MetricDataTypeHistogram:
		dataPoints := md.Histogram().DataPoints()
		var adxMetrics []*AdxMetric
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			bounds := dataPoint.MExplicitBounds()
			counts := dataPoint.MBucketCounts()
			clonedscopeattrs := cloneMap(scopeattrs)
			copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())
			// first, add one event for sum, and one for count
			{
				metricValue := dataPoint.Sum()
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName:         fmt.Sprintf("%s_%s", md.Name(), sumsuffix),
					MetricType:         pmetric.MetricDataTypeHistogram.String(),
					MetricUnit:         md.Unit(),
					MetricDescription:  fmt.Sprintf("%s%s", md.Description(), sumdescription),
					MetricValue:        metricValue,
					MetricAttributes:   clonedscopeattrs,
					Host:               host,
					ResourceAttributes: resourceattrs,
				})
			}
			{
				// Change int to float. The value is a float64 in the table
				metricValue := float64(dataPoint.Count())
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName:         fmt.Sprintf("%s_%s", md.Name(), countsuffix),
					MetricType:         pmetric.MetricDataTypeHistogram.String(), // A count does not have a unit
					MetricDescription:  fmt.Sprintf("%s%s", md.Description(), countdescription),
					MetricUnit:         md.Unit(),
					MetricValue:        metricValue,
					MetricAttributes:   clonedscopeattrs,
					Host:               host,
					ResourceAttributes: resourceattrs,
				})
			}
			// Spec says counts is optional but if present it must have one more
			// element than the bounds array.
			if len(counts) == 0 || len(counts) != len(bounds)+1 {
				continue
			}
			value := uint64(0)
			// now create buckets for each bound.
			for bi := 0; bi < len(bounds); bi++ {
				clonedscopeattrs := cloneMap(scopeattrs)
				copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())
				// Add the LE field for the bucket's bound
				clonedscopeattrs["le"] = float64ToDimValue(bounds[bi])
				value += counts[bi]

				// Change int to float. The value is a float64 in the table
				metricValue := float64(value)
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName:         fmt.Sprintf("%s_bucket", md.Name()),
					MetricType:         pmetric.MetricDataTypeHistogram.String(),
					MetricUnit:         md.Unit(),
					MetricDescription:  md.Description(),
					MetricValue:        metricValue,
					MetricAttributes:   clonedscopeattrs,
					Host:               host,
					ResourceAttributes: resourceattrs,
				})
			}
			// add an upper bound for +Inf
			{

				clonedscopeattrs := cloneMap(scopeattrs)
				copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())
				// Add the LE field for the bucket's bound
				clonedscopeattrs["le"] = float64ToDimValue(math.Inf(1))
				metricValue := float64(value + counts[len(counts)-1])
				// Change int to float. The value is a float64 in the table
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName:         fmt.Sprintf("%s_bucket", md.Name()),
					MetricType:         pmetric.MetricDataTypeHistogram.String(),
					MetricValue:        metricValue,
					MetricUnit:         md.Unit(),
					MetricDescription:  md.Description(),
					MetricAttributes:   clonedscopeattrs,
					Host:               host,
					ResourceAttributes: resourceattrs,
				})
			}
		}
		return adxMetrics
	case pmetric.MetricDataTypeSum:
		dataPoints := md.Sum().DataPoints()
		adxMetrics := make([]*AdxMetric, dataPoints.Len())
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			clonedscopeattrs := cloneMap(scopeattrs)
			copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())
			var metricValue float64
			switch dataPoint.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				metricValue = float64(dataPoint.IntVal())
			case pmetric.NumberDataPointValueTypeDouble:
				metricValue = float64(dataPoint.DoubleVal())
			}
			adxMetrics[gi] = &AdxMetric{
				Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
				MetricName:         md.Name(),
				MetricType:         pmetric.MetricDataTypeSum.String(),
				MetricUnit:         md.Unit(),
				MetricDescription:  md.Description(),
				MetricValue:        metricValue,
				Host:               host,
				MetricAttributes:   clonedscopeattrs,
				ResourceAttributes: resourceattrs,
			}
		}
		return adxMetrics
	case pmetric.MetricDataTypeSummary:
		dataPoints := md.Summary().DataPoints()
		var adxMetrics []*AdxMetric
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			// first, add one event for sum, and one for count
			{
				clonedscopeattrs := cloneMap(scopeattrs)
				copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())

				metricValue := float64(dataPoint.Sum())
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName:         fmt.Sprintf("%s_%s", md.Name(), sumsuffix),
					MetricType:         pmetric.MetricDataTypeSummary.String(),
					MetricUnit:         md.Unit(), // Sum of all the samples
					MetricDescription:  fmt.Sprintf("%s%s", md.Description(), sumdescription),
					MetricValue:        metricValue,
					Host:               host,
					MetricAttributes:   clonedscopeattrs,
					ResourceAttributes: resourceattrs,
				})
			}
			//counts
			{
				clonedscopeattrs := cloneMap(scopeattrs)
				copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())
				metricValue := float64(dataPoint.Count())
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:          dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName:         fmt.Sprintf("%s_%s", md.Name(), countsuffix),
					MetricType:         pmetric.MetricDataTypeSummary.String(), // There is no metric unit for number of / count of
					MetricDescription:  fmt.Sprintf("%s%s", md.Description(), countdescription),
					MetricValue:        metricValue,
					MetricAttributes:   clonedscopeattrs,
					Host:               host,
					ResourceAttributes: resourceattrs,
				})
			}
			// now create values for each quantile.
			for bi := 0; bi < dataPoint.QuantileValues().Len(); bi++ {
				dp := dataPoint.QuantileValues().At(bi)
				clonedscopeattrs := cloneMap(scopeattrs)
				copyAttributes(clonedscopeattrs, dataPoint.Attributes().AsRaw())
				clonedscopeattrs["qt"] = float64ToDimValue(dp.Quantile())
				quantilename := fmt.Sprintf("%s_%s", md.Name(), strconv.FormatFloat(dp.Quantile(), 'f', -1, 64))
				clonedscopeattrs[quantilename] = sanitizeFloat(dp.Value())
				metricValue := dp.Value()
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: quantilename,
					MetricType: pmetric.MetricDataTypeSummary.String(),
					// There is no unit for a quantile. Will be empty
					MetricDescription:  fmt.Sprintf("%s%s", md.Description(), countdescription),
					MetricValue:        metricValue,
					MetricAttributes:   clonedscopeattrs,
					Host:               host,
					ResourceAttributes: resourceattrs,
				})
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
	var transformedadxmetrics []*AdxMetric
	resourceMetric := metrics.ResourceMetrics()
	for i := 0; i < resourceMetric.Len(); i++ {
		res := resourceMetric.At(i).Resource()
		scopeMetrics := resourceMetric.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			scopemetric := scopeMetrics.At(j)
			metrics := scopemetric.Metrics()
			// get details of the scope from the scope metric
			scopeAttr := getScopeMap(scopemetric.Scope())
			for k := 0; k < metrics.Len(); k++ {
				transformedadxmetrics = append(transformedadxmetrics, mapToAdxMetric(res, metrics.At(k), scopeAttr, logger)...)
			}
		}
	}
	return transformedadxmetrics, nil
}

func copyAttributes(scopeattrmap map[string]interface{}, metricattributes map[string]interface{}) {
	for k, v := range metricattributes {
		scopeattrmap[k] = v
	}
}

func cloneMap(fields map[string]interface{}) map[string]interface{} {
	newFields := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		newFields[k] = v
	}
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
