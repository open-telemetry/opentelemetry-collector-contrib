// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package normalizesumsprocessor

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type NormalizeSumsProcessor struct {
	logger     *zap.Logger
	transforms []SumMetrics

	history map[string]*startPoint
}

type startPoint struct {
	dataType pdata.MetricDataType

	intDataPoint    *pdata.IntDataPoint
	doubleDataPoint *pdata.DoubleDataPoint
	lastIntValue    int64
	lastDoubleValue float64
}

func newNormalizeSumsProcessor(logger *zap.Logger, transforms []SumMetrics) *NormalizeSumsProcessor {
	return &NormalizeSumsProcessor{
		logger:     logger,
		transforms: transforms,
		history:    make(map[string]*startPoint),
	}
}

// ProcessMetrics implements the MProcessor interface.
func (nsp *NormalizeSumsProcessor) ProcessMetrics(ctx context.Context, metrics pdata.Metrics) (pdata.Metrics, error) {
	var errors []error

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rms := metrics.ResourceMetrics().At(i)
		if nsp.transforms == nil {
			processingErrors := nsp.transformAllSumMetrics(rms)
			errors = append(errors, processingErrors...)
		} else {
			for _, transform := range nsp.transforms {
				metric, slice := findMetric(transform.MetricName, rms)

				if metric == nil {
					continue
				}

				err := nsp.transformMetric(transform, rms.Resource(), metric, slice)
				if err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	if len(errors) > 0 {
		return metrics, consumererror.Combine(errors)
	}

	return metrics, nil
}

func (nsp *NormalizeSumsProcessor) transformAllSumMetrics(rms pdata.ResourceMetrics) []error {
	var errors []error

	ilms := rms.InstrumentationLibraryMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j).Metrics()
		for k := 0; k < ilm.Len(); k++ {
			metric := ilm.At(k)
			if metric.DataType() == pdata.MetricDataTypeIntSum || metric.DataType() == pdata.MetricDataTypeDoubleSum {
				err := nsp.transformMetric(SumMetrics{MetricName: metric.Name()}, rms.Resource(), &metric, &ilm)
				if err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	return errors
}

//func findMetric(name string, rms pdata.ResourceMetrics) (*pdata.Metric, *pdata.MetricSlice) {
func (nsp *NormalizeSumsProcessor) transformMetric(transform SumMetrics, resource pdata.Resource, metric *pdata.Metric, slice *pdata.MetricSlice) error {
	switch t := metric.DataType(); t {
	case pdata.MetricDataTypeIntSum:
		dataPointsRemaining, processingErr := nsp.processIntSumMetric(slice, transform, resource, metric)
		if processingErr != nil {
			return processingErr
		} else {
			cleanUpMetricSlice(transform, metric, slice, dataPointsRemaining)
		}
	case pdata.MetricDataTypeDoubleSum:
		dataPointsRemaining, processingErr := nsp.processDoubleSumMetric(slice, transform, resource, metric)
		if processingErr != nil {
			return processingErr
		} else {
			cleanUpMetricSlice(transform, metric, slice, dataPointsRemaining)
		}
	default:
		return fmt.Errorf("Data Type not supported %s", metric.DataType())
	}

	return nil
}

func cleanUpMetricSlice(transform SumMetrics, metric *pdata.Metric, slice *pdata.MetricSlice, dataPointsRemaining int) {
	// If there is meaningful data to send and we are renaming the metric,
	// add the already renamed metric to the slice
	if dataPointsRemaining > 0 && transform.NewName != "" {
		newMetric := slice.AppendEmpty()
		metric.CopyTo(newMetric)
		newMetric.SetName(transform.NewName)
	}

	// If there are no remaining data points after removing restart/start
	// points, or this metric was renamed, remove this metric from the slice
	if dataPointsRemaining == 0 || transform.NewName != "" {
		metricSliceRemoveElement(slice, transform.MetricName)
	}
}

func (nsp *NormalizeSumsProcessor) processIntSumMetric(slice *pdata.MetricSlice, transform SumMetrics, resource pdata.Resource, metric *pdata.Metric) (int, error) {
	dps := metric.IntSum().DataPoints()
	for i := 0; i < dps.Len(); {
		reportData := nsp.processIntSumDataPoint(dps.At(i), resource, metric)

		if reportData == false {
			intRemoveAt(&dps, i)
			continue
		}
		i++
	}

	return dps.Len(), nil
}

func (nsp *NormalizeSumsProcessor) processIntSumDataPoint(dp pdata.IntDataPoint, resource pdata.Resource, metric *pdata.Metric) bool {
	metricIdentifier := dataPointIdentifier(resource, metric, dp.LabelsMap())

	start := nsp.history[metricIdentifier]
	// If this is the first time we've observed this unique metric,
	// record it as the start point and do not report this data point
	if start == nil {
		dps := metric.IntSum().DataPoints()
		newDP := pdata.NewIntDataPoint()
		dps.At(0).CopyTo(newDP)

		newStart := startPoint{
			dataType:     pdata.MetricDataTypeIntSum,
			intDataPoint: &newDP,
			lastIntValue: newDP.Value(),
		}
		nsp.history[metricIdentifier] = &newStart

		return false
	}

	// If this data is older than the start point, we can't
	// meaningfully report this point
	if dp.Timestamp() <= start.intDataPoint.Timestamp() {
		return false
	}

	// If data has rolled over or the counter has been restarted for
	// any other reason, grab a new start point and do not report this data
	if dp.Value() < start.lastIntValue {
		dp.CopyTo(*start.intDataPoint)
		start.lastIntValue = dp.Value()

		return false
	}

	start.lastIntValue = dp.Value()
	dp.SetValue(dp.Value() - start.intDataPoint.Value())
	dp.SetStartTimestamp(start.intDataPoint.Timestamp())

	return true
}

func (nsp *NormalizeSumsProcessor) processDoubleSumMetric(slice *pdata.MetricSlice, transform SumMetrics, resource pdata.Resource, metric *pdata.Metric) (int, error) {
	dps := metric.DoubleSum().DataPoints()
	for i := 0; i < dps.Len(); {
		reportData := nsp.processDoubleSumDataPoint(dps.At(i), resource, metric)

		if reportData == false {
			doubleRemoveAt(&dps, i)
			continue
		}
		i++
	}

	return dps.Len(), nil
}

func (nsp *NormalizeSumsProcessor) processDoubleSumDataPoint(dp pdata.DoubleDataPoint, resource pdata.Resource, metric *pdata.Metric) bool {
	metricIdentifier := dataPointIdentifier(resource, metric, dp.LabelsMap())

	start := nsp.history[metricIdentifier]
	// If this is the first time we've observed this unique metric,
	// record it as the start point and do not report this data point
	if start == nil {
		dps := metric.DoubleSum().DataPoints()
		newDP := pdata.NewDoubleDataPoint()
		dps.At(0).CopyTo(newDP)

		newStart := startPoint{
			dataType:        pdata.MetricDataTypeIntSum,
			doubleDataPoint: &newDP,
			lastDoubleValue: newDP.Value(),
		}
		nsp.history[metricIdentifier] = &newStart

		return false
	}

	// If this data is older than the start point, we can't
	// meaningfully report this point
	if dp.Timestamp() <= start.doubleDataPoint.Timestamp() {
		return false
	}

	// If data has rolled over or the counter has been restarted for
	// any other reason, grab a new start point and do not report this data
	if dp.Value() < start.lastDoubleValue {
		dp.CopyTo(*start.doubleDataPoint)
		start.lastDoubleValue = dp.Value()

		return false
	}

	start.lastDoubleValue = dp.Value()
	dp.SetValue(dp.Value() - start.doubleDataPoint.Value())
	dp.SetStartTimestamp(start.doubleDataPoint.Timestamp())

	return true
}

func findMetric(name string, rms pdata.ResourceMetrics) (*pdata.Metric, *pdata.MetricSlice) {
	ilms := rms.InstrumentationLibraryMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j).Metrics()
		for k := 0; k < ilm.Len(); k++ {
			metric := ilm.At(k)
			if name == metric.Name() {
				return &metric, &ilm
			}
		}
	}

	return nil, nil
}

func dataPointIdentifier(resource pdata.Resource, metric *pdata.Metric, labels pdata.StringMap) string {
	var b strings.Builder

	// Resource identifiers
	resource.Attributes().Sort().Range(func(k string, v pdata.AttributeValue) bool {
		fmt.Fprintf(&b, "%s=", k)
		addAttributeToIdentityBuilder(&b, v)
		b.WriteString("|")
		return true
	})

	// Metric identifiers
	fmt.Fprintf(&b, " - %s", metric.Name())
	labels.Sort().Range(func(k, v string) bool {
		fmt.Fprintf(&b, " %s=%s", k, v)
		return true
	})
	return b.String()
}

func addAttributeToIdentityBuilder(b *strings.Builder, v pdata.AttributeValue) {
	switch v.Type() {
	case pdata.AttributeValueTypeArray:
		b.WriteString("[")
		arr := v.ArrayVal()
		for i := 0; i < arr.Len(); i++ {
			addAttributeToIdentityBuilder(b, arr.At(i))
			b.WriteString(",")
		}
		b.WriteString("]")
	case pdata.AttributeValueTypeBool:
		fmt.Fprintf(b, "%t", v.BoolVal())
	case pdata.AttributeValueTypeDouble:
		// TODO - Double attribute values could be problematic for use in
		// forming an identify due to floating point math. Consider how to best
		// handle this case
		fmt.Fprintf(b, "%f", v.DoubleVal())
	case pdata.AttributeValueTypeInt:
		fmt.Fprintf(b, "%d", v.IntVal())
	case pdata.AttributeValueTypeMap:
		b.WriteString("{")
		v.MapVal().Sort().Range(func(k string, mapVal pdata.AttributeValue) bool {
			fmt.Fprintf(b, "%s:", k)
			addAttributeToIdentityBuilder(b, mapVal)
			b.WriteString(",")
			return true
		})
		b.WriteString("}")
	case pdata.AttributeValueTypeNull:
		b.WriteString("NULL")
	case pdata.AttributeValueTypeString:
		fmt.Fprintf(b, "'%s'", v.StringVal())
	}
}

func intRemoveAt(slice *pdata.IntDataPointSlice, idx int) {
	newSlice := pdata.NewIntDataPointSlice()
	newSlice.Resize(slice.Len() - 1)
	j := 0
	for i := 0; i < slice.Len(); i++ {
		if i != idx {
			slice.At(i).CopyTo(newSlice.At(j))
			j++
		}
	}

	newSlice.CopyTo(*slice)
}

func doubleRemoveAt(slice *pdata.DoubleDataPointSlice, idx int) {
	newSlice := pdata.NewDoubleDataPointSlice()
	newSlice.Resize(slice.Len() - 1)
	j := 0
	for i := 0; i < slice.Len(); i++ {
		if i != idx {
			slice.At(i).CopyTo(newSlice.At(j))
			j++
		}
	}
	newSlice.CopyTo(*slice)
}

func metricSliceRemoveElement(slice *pdata.MetricSlice, name string) {
	newSlice := pdata.NewMetricSlice()
	newSlice.Resize(slice.Len() - 1)
	j := 0
	for i := 0; i < slice.Len(); i++ {
		if slice.At(i).Name() != name {
			slice.At(i).CopyTo(newSlice.At(j))
			j++
		}
	}
	newSlice.CopyTo(*slice)
}
