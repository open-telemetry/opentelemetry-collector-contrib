package azuredataexplorerexporter

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

/*
A converter package that converts and marshals data to be written to ADX metrics tables
*/
const (
	hostKey = "host.key"
	// Indicates the sum that is used in both summary and in histogram
	sumSuffix = "sum"
	// Count used in summary , histogram and also in exponential histogram
	countSuffix = "count"
)

type AdxMetric struct {
	Timestamp  string  //The timestamp of the occurance. A metric is measured at a point of time. Formatted into string as RFC3339
	MetricName string  //Name of the metric field
	MetricType string  // The type of the metric field
	Value      float64 // the value of the metric
	Host       string  // the hostname for analysis of the metric
	Attributes string  // JSON attributes that can then be parsed. Usually custom metrics
}

/*
Convert the pMetric to the type ADXMetric , this matches the scheme in the RawMetric table in the database
*/

func mapToAdxMetric(res pcommon.Resource, md pmetric.Metric, logger *zap.Logger) []*AdxMetric {
	logger.Debug("Entering processing of toAdxMetric function")
	// default to collectors host name. Ignore the error here. This should not cause the failure of the process
	host, _ := os.Hostname()
	commonFields := map[string]interface{}{}

	res.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case hostKey:
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
			fields := cloneMap(commonFields)
			populateAttributes(fields, dataPoint.Attributes())
			var metricValue float64

			switch dataPoint.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				metricValue = float64(dataPoint.IntVal())
			case pmetric.NumberDataPointValueTypeDouble:
				metricValue = float64(dataPoint.DoubleVal())
			}
			// there is an error case here as well! TODO need to atleast log it
			fieldsJson, err := jsoniter.MarshalToString(fields)
			if err != nil {
				logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
			}

			adxMetrics[gi] = &AdxMetric{
				Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
				MetricName: md.Name(),
				MetricType: pmetric.MetricDataTypeGauge.String(),
				Value:      metricValue,
				Host:       host,
				Attributes: fieldsJson,
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
			// first, add one event for sum, and one for count
			{
				fields := cloneMap(commonFields)
				// there is an error case here as well! TODO need to atleast log it
				fieldsJson, err := jsoniter.MarshalToString(fields)
				if err != nil {
					logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
				}
				populateAttributes(fields, dataPoint.Attributes())
				metricValue := dataPoint.Sum()
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: fmt.Sprintf("%s_%s", md.Name(), sumSuffix),
					MetricType: pmetric.MetricDataTypeHistogram.String(),
					Value:      metricValue,
					Host:       host,
					Attributes: fieldsJson,
				})
			}
			{
				fields := cloneMap(commonFields)
				// there is an error case here as well! TODO need to atleast log it
				fieldsJson, err := jsoniter.MarshalToString(fields)
				if err != nil {
					logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
				}
				populateAttributes(fields, dataPoint.Attributes())
				// Change int to float. The value is a float64 in the table
				metricValue := float64(dataPoint.Count())
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: fmt.Sprintf("%s_%s", md.Name(), countSuffix),
					MetricType: pmetric.MetricDataTypeHistogram.String(),
					Value:      metricValue,
					Host:       host,
					Attributes: fieldsJson,
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
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPoint.Attributes())
				// Add the LE field for the bucket's bound
				fields["le"] = float64ToDimValue(bounds[bi])
				value += counts[bi]

				// there is an error case here as well! TODO need to atleast log it
				fieldsJson, err := jsoniter.MarshalToString(fields)
				if err != nil {
					logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
				}
				// Change int to float. The value is a float64 in the table
				metricValue := float64(value)
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: fmt.Sprintf("%s_bucket", md.Name()),
					MetricType: pmetric.MetricDataTypeHistogram.String(),
					Value:      metricValue,
					Host:       host,
					Attributes: fieldsJson,
				})
			}
			// add an upper bound for +Inf
			{

				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPoint.Attributes())
				// Add the LE field for the bucket's bound
				fields["le"] = float64ToDimValue(math.Inf(1))
				metricValue := float64(value + counts[len(counts)-1])
				// there is an error case here as well! TODO need to atleast log it
				fieldsJson, err := jsoniter.MarshalToString(fields)
				if err != nil {
					logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
				}
				// Change int to float. The value is a float64 in the table
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: fmt.Sprintf("%s_bucket", md.Name()),
					MetricType: pmetric.MetricDataTypeHistogram.String(),
					Value:      metricValue,
					Host:       host,
					Attributes: fieldsJson,
				})
			}
		}
		return adxMetrics
	case pmetric.MetricDataTypeSum:
		dataPoints := md.Sum().DataPoints()
		adxMetrics := make([]*AdxMetric, dataPoints.Len())
		for gi := 0; gi < dataPoints.Len(); gi++ {
			dataPoint := dataPoints.At(gi)
			fields := cloneMap(commonFields)
			populateAttributes(fields, dataPoint.Attributes())
			// there is an error case here as well! TODO need to atleast log it
			fieldsJson, err := jsoniter.MarshalToString(fields)
			if err != nil {
				logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
			}

			var metricValue float64

			switch dataPoint.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				metricValue = float64(dataPoint.IntVal())
			case pmetric.NumberDataPointValueTypeDouble:
				metricValue = float64(dataPoint.DoubleVal())
			}

			adxMetrics[gi] = &AdxMetric{
				Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
				MetricName: md.Name(),
				MetricType: pmetric.MetricDataTypeSum.String(),
				Value:      metricValue,
				Host:       host,
				Attributes: fieldsJson,
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
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPoint.Attributes())
				// there is an error case here as well! TODO need to atleast log it
				fieldsJson, err := jsoniter.MarshalToString(fields)
				if err != nil {
					logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
				}
				metricValue := float64(dataPoint.Sum())
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: fmt.Sprintf("%s_%s", md.Name(), sumSuffix),
					MetricType: pmetric.MetricDataTypeSummary.String(),
					Value:      metricValue,
					Host:       host,
					Attributes: fieldsJson,
				})
			}

			{
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPoint.Attributes())
				// there is an error case here as well! TODO need to atleast log it
				fieldsJson, err := jsoniter.MarshalToString(fields)
				if err != nil {
					logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
				}
				metricValue := float64(dataPoint.Count())
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: fmt.Sprintf("%s_%s", md.Name(), countSuffix),
					MetricType: pmetric.MetricDataTypeSummary.String(),
					Value:      metricValue,
					Host:       host,
					Attributes: fieldsJson,
				})
			}

			// now create values for each quantile.
			for bi := 0; bi < dataPoint.QuantileValues().Len(); bi++ {
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPoint.Attributes())
				dp := dataPoint.QuantileValues().At(bi)
				fields["qt"] = float64ToDimValue(dp.Quantile())
				fields[md.Name()+"_"+strconv.FormatFloat(dp.Quantile(), 'f', -1, 64)] = sanitizeFloat(dp.Value())

				// there is an error case here as well! TODO need to atleast log it
				fieldsJson, err := jsoniter.MarshalToString(fields)
				if err != nil {
					logger.Warn("Error marshalling attribute fields ", zap.Any("Attributes", fields), zap.Error(err))
				}
				metricValue := dp.Value()
				adxMetrics = append(adxMetrics, &AdxMetric{
					Timestamp:  dataPoint.Timestamp().AsTime().Format(time.RFC3339),
					MetricName: fmt.Sprintf("%s_%s", md.Name(), strconv.FormatFloat(dp.Quantile(), 'f', -1, 64)),
					MetricType: pmetric.MetricDataTypeSummary.String(),
					Value:      metricValue,
					Host:       host,
					Attributes: fieldsJson,
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

func populateAttributes(fields map[string]interface{}, attributeMap pcommon.Map) {
	attributeMap.Range(func(k string, v pcommon.Value) bool {
		fields[k] = v.AsString()
		return true
	})
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
