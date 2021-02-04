// Copyright 2020, OpenTelemetry Authors
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

package awsemfexporter

import (
	"encoding/json"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	// OTel instrumentation lib name as dimension
	oTellibDimensionKey          = "OTelLib"
	defaultNamespace             = "default"
	noInstrumentationLibraryName = "Undefined"

	// See: http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	maximumLogEventsPerPut = 10000

	// DimensionRollupOptions
	zeroAndSingleDimensionRollup = "ZeroAndSingleDimensionRollup"
	singleDimensionRollupOnly    = "SingleDimensionRollupOnly"
)

// CWMetrics defines
type CWMetrics struct {
	Measurements []CwMeasurement
	TimestampMs  int64
	Fields       map[string]interface{}
}

// CwMeasurement defines
type CwMeasurement struct {
	Namespace  string
	Dimensions [][]string
	Metrics    []map[string]string
}

// CWMetric stats defines
type CWMetricStats struct {
	Max   float64
	Min   float64
	Count uint64
	Sum   float64
}

// CWMetricMetadata represents the metadata associated with a given CloudWatch metric
type CWMetricMetadata struct {
	Namespace                  string
	TimestampMs                int64
	LogGroup                   string
	LogStream                  string
	InstrumentationLibraryName string
}

// TranslateOtToCWMetric converts OT metrics to CloudWatch Metric format
func TranslateOtToCWMetric(rm *pdata.ResourceMetrics, config *Config) ([]*CWMetrics, int) {
	var cwMetricList []*CWMetrics
	var instrumentationLibName string

	cWNamespace := getNamespace(rm, config.Namespace)
	logGroup, logStream := getLogInfo(rm, cWNamespace, config)
	timestampMs := time.Now().UnixNano() / int64(time.Millisecond)

	ilms := rm.InstrumentationLibraryMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j)
		if ilm.InstrumentationLibrary().Name() == "" {
			instrumentationLibName = noInstrumentationLibraryName
		} else {
			instrumentationLibName = ilm.InstrumentationLibrary().Name()
		}

		metrics := ilm.Metrics()
		for k := 0; k < metrics.Len(); k++ {
			metric := metrics.At(k)
			metadata := CWMetricMetadata{
				Namespace:                  cWNamespace,
				TimestampMs:                timestampMs,
				LogGroup:                   logGroup,
				LogStream:                  logStream,
				InstrumentationLibraryName: instrumentationLibName,
			}
			cwMetrics := getCWMetrics(&metric, metadata, instrumentationLibName, config)
			cwMetricList = append(cwMetricList, cwMetrics...)
		}
	}
	return cwMetricList, 0
}

// TranslateCWMetricToEMF converts CloudWatch Metric format to EMF.
func TranslateCWMetricToEMF(cwMetricLists []*CWMetrics, logger *zap.Logger) []*LogEvent {
	// convert CWMetric into map format for compatible with PLE input
	ples := make([]*LogEvent, 0, maximumLogEventsPerPut)
	for _, met := range cwMetricLists {
		cwmMap := make(map[string]interface{})
		fieldMap := met.Fields

		if len(met.Measurements) > 0 {
			// Create `_aws` section only if there are measurements
			cwmMap["CloudWatchMetrics"] = met.Measurements
			cwmMap["Timestamp"] = met.TimestampMs
			fieldMap["_aws"] = cwmMap
		} else {
			str, _ := json.Marshal(fieldMap)
			logger.Debug("Dropped metric due to no matching metric declarations", zap.String("labels", string(str)))
		}

		pleMsg, err := json.Marshal(fieldMap)
		if err != nil {
			continue
		}
		metricCreationTime := met.TimestampMs

		logEvent := NewLogEvent(
			metricCreationTime,
			string(pleMsg),
		)
		logEvent.LogGeneratedTime = time.Unix(0, metricCreationTime*int64(time.Millisecond))
		ples = append(ples, logEvent)
	}
	return ples
}

// getCWMetrics translates OTLP Metric to a list of CW Metrics
func getCWMetrics(metric *pdata.Metric, metadata CWMetricMetadata, instrumentationLibName string, config *Config) (cwMetrics []*CWMetrics) {
	if metric == nil {
		return
	}

	dps := getDataPoints(metric, metadata, config.logger)
	if dps == nil || dps.Len() == 0 {
		return
	}

	// metric measure data from OT
	metricMeasure := make(map[string]string)
	metricMeasure["Name"] = metric.Name()
	metricMeasure["Unit"] = metric.Unit()
	// metric measure slice could include multiple metric measures
	metricSlice := []map[string]string{metricMeasure}

	for m := 0; m < dps.Len(); m++ {
		dp := dps.At(m)
		cwMetric := buildCWMetric(dp, metric, metadata.Namespace, metricSlice, instrumentationLibName, config)
		if cwMetric != nil {
			cwMetrics = append(cwMetrics, cwMetric)
		}
	}
	return
}

// buildCWMetric builds CWMetric from DataPoint
func buildCWMetric(dp DataPoint, pmd *pdata.Metric, namespace string, metricSlice []map[string]string, instrumentationLibName string, config *Config) *CWMetrics {
	dimensionRollupOption := config.DimensionRollupOption
	metricDeclarations := config.MetricDeclarations

	labelsMap := dp.Labels
	labelsSlice := make([]string, len(labelsMap), len(labelsMap)+1)
	// `labels` contains label key/value pairs
	labels := make(map[string]string, len(labelsMap)+1)
	// `fields` contains metric and dimensions key/value pairs
	fields := make(map[string]interface{}, len(labelsMap)+2)
	idx := 0
	for k, v := range labelsMap {
		fields[k] = v
		labels[k] = v
		labelsSlice[idx] = k
		idx++
	}

	// Apply single/zero dimension rollup to labels
	rollupDimensionArray := dimensionRollup(dimensionRollupOption, labelsSlice, instrumentationLibName)

	// Add OTel instrumentation lib name as an additional dimension if it is defined
	if instrumentationLibName != noInstrumentationLibraryName {
		labels[oTellibDimensionKey] = instrumentationLibName
		fields[oTellibDimensionKey] = instrumentationLibName
	}

	// Create list of dimension sets
	var dimensions [][]string
	if len(metricDeclarations) > 0 {
		// If metric declarations are defined, extract dimension sets from them
		dimensions = processMetricDeclarations(metricDeclarations, pmd, labels, rollupDimensionArray)
	} else {
		// If no metric declarations defined, create a single dimension set containing
		// the list of labels
		dims := labelsSlice
		if instrumentationLibName != noInstrumentationLibraryName {
			// If OTel instrumentation lib name is defined, add instrumentation lib
			// name as a dimension
			dims = append(dims, oTellibDimensionKey)
		}

		if len(rollupDimensionArray) > 0 {
			// Perform de-duplication check for edge case with a single label and single roll-up
			// is activated
			if len(labelsSlice) > 1 || (dimensionRollupOption != singleDimensionRollupOnly &&
				dimensionRollupOption != zeroAndSingleDimensionRollup) {
				dimensions = [][]string{dims}
			}
			dimensions = append(dimensions, rollupDimensionArray...)
		} else {
			dimensions = [][]string{dims}
		}
	}

	// Build list of CW Measurements
	var cwMeasurements []CwMeasurement
	if len(dimensions) > 0 {
		cwMeasurements = []CwMeasurement{
			{
				Namespace:  namespace,
				Dimensions: dimensions,
				Metrics:    metricSlice,
			},
		}
	}

	timestampMs := time.Now().UnixNano() / int64(time.Millisecond)
	if dp.TimestampMs > 0 {
		timestampMs = dp.TimestampMs
	}

	metricVal := dp.Value
	if metricVal == nil {
		return nil
	}
	fields[pmd.Name()] = metricVal

	cwMetric := &CWMetrics{
		Measurements: cwMeasurements,
		TimestampMs:  timestampMs,
		Fields:       fields,
	}
	return cwMetric
}

// dimensionRollup creates rolled-up dimensions from the metric's label set.
func dimensionRollup(dimensionRollupOption string, originalDimensionSlice []string, instrumentationLibName string) [][]string {
	var rollupDimensionArray [][]string
	dimensionZero := make([]string, 0)
	if instrumentationLibName != noInstrumentationLibraryName {
		dimensionZero = append(dimensionZero, oTellibDimensionKey)
	}
	if dimensionRollupOption == zeroAndSingleDimensionRollup {
		//"Zero" dimension rollup
		if len(originalDimensionSlice) > 0 {
			rollupDimensionArray = append(rollupDimensionArray, dimensionZero)
		}
	}
	if dimensionRollupOption == zeroAndSingleDimensionRollup || dimensionRollupOption == singleDimensionRollupOnly {
		//"One" dimension rollup
		for _, dimensionKey := range originalDimensionSlice {
			rollupDimensionArray = append(rollupDimensionArray, append(dimensionZero, dimensionKey))
		}
	}

	return rollupDimensionArray
}
