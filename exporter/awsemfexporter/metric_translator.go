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
	"fmt"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	// OTel instrumentation lib name as dimension
	OTellibDimensionKey          = "OTelLib"
	defaultNamespace             = "default"
	noInstrumentationLibraryName = "Undefined"

	// DimensionRollupOptions
	ZeroAndSingleDimensionRollup = "ZeroAndSingleDimensionRollup"
	SingleDimensionRollupOnly    = "SingleDimensionRollupOnly"
)

// CWMetrics defines
type CWMetrics struct {
	Measurements []CWMeasurement
	Timestamp    int64
	Fields       map[string]interface{}
}

// CWMeasurement defines
type CWMeasurement struct {
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
	Timestamp                  int64
	LogGroup                   string
	LogStream                  string
	InstrumentationLibraryName string
}

// TranslateOtToGroupedMetric converts OT metrics to Grouped Metric format.
func TranslateOtToGroupedMetric(rm *pdata.ResourceMetrics, groupedMetrics map[string]*GroupedMetric, config *Config) {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	var instrumentationLibName string
	cWNamespace := getNamespace(rm, config.Namespace)
	logGroup, logStream := getLogInfo(rm, cWNamespace, config)

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
				Timestamp:                  timestamp,
				LogGroup:                   logGroup,
				LogStream:                  logStream,
				InstrumentationLibraryName: instrumentationLibName,
			}
			addToGroupedMetric(&metric, groupedMetrics, metadata, config.logger)
		}
	}
}

// TranslateCWMetricToEMF converts CloudWatch Metric format to EMF.
func TranslateCWMetricToEMF(cWMetric *CWMetrics) *LogEvent {
	// convert CWMetric into map format for compatible with PLE input
	cWMetricMap := make(map[string]interface{})
	fieldMap := cWMetric.Fields

	// Create `_aws` section only if there are measurements
	if len(cWMetric.Measurements) > 0 {
		// Create `_aws` section only if there are measurements
		cWMetricMap["CloudWatchMetrics"] = cWMetric.Measurements
		cWMetricMap["Timestamp"] = cWMetric.Timestamp
		fieldMap["_aws"] = cWMetricMap
	}

	pleMsg, err := json.Marshal(fieldMap)
	if err != nil {
		return nil
	}

	metricCreationTime := cWMetric.Timestamp
	logEvent := NewLogEvent(
		metricCreationTime,
		string(pleMsg),
	)
	logEvent.LogGeneratedTime = time.Unix(0, metricCreationTime*int64(time.Millisecond))

	return logEvent
}

// TranslateGroupedMetricToCWMetric converts Grouped Metric format to CloudWatch Metric format.
func TranslateGroupedMetricToCWMetric(groupedMetric *GroupedMetric, config *Config) *CWMetrics {
	labels := groupedMetric.Labels
	fields := make(map[string]interface{}, len(labels)+len(groupedMetric.Metrics))

	// Add labels to fields
	for k, v := range labels {
		fields[k] = v
	}

	// Add metrics to fields
	for metricName, metricInfo := range groupedMetric.Metrics {
		fields[metricName] = metricInfo.Value
	}

	var cWMeasurements []CWMeasurement
	if len(config.MetricDeclarations) == 0 {
		// If there are no metric declarations defined, translate grouped metric
		// into the corresponding CW Measurement
		cwm := groupedMetricToCWMeasurement(groupedMetric, config)
		cWMeasurements = []CWMeasurement{cwm}
	} else {
		// If metric declarations are defined, filter grouped metric's metrics using
		// metric declarations and translate into the corresponding list of CW Measurements
		cWMeasurements = groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
	}

	return &CWMetrics{
		Measurements: cWMeasurements,
		Timestamp:    groupedMetric.Metadata.Timestamp,
		Fields:       fields,
	}
}

// groupedMetricToCWMeasurement creates a single CW Measurement from a grouped metric.
func groupedMetricToCWMeasurement(groupedMetric *GroupedMetric, config *Config) CWMeasurement {
	labels := groupedMetric.Labels
	dimensionRollupOption := config.DimensionRollupOption

	// Create a dimension set containing list of label names
	dimSet := make([]string, len(labels))
	idx := 0
	for labelName := range labels {
		dimSet[idx] = labelName
		idx++
	}
	dimensions := [][]string{dimSet}

	// Apply single/zero dimension rollup to labels
	rollupDimensionArray := dimensionRollup(dimensionRollupOption, labels)

	if len(rollupDimensionArray) > 0 {
		// Perform duplication check for edge case with a single label and single dimension roll-up
		_, hasOTelLibKey := labels[OTellibDimensionKey]
		isSingleLabel := len(dimSet) <= 1 || (len(dimSet) == 2 && hasOTelLibKey)
		singleDimRollup := dimensionRollupOption == SingleDimensionRollupOnly ||
			dimensionRollupOption == ZeroAndSingleDimensionRollup
		if isSingleLabel && singleDimRollup {
			// Remove duplicated dimension set before adding on rolled-up dimensions
			dimensions = nil
		}
	}

	// Add on rolled-up dimensions
	dimensions = append(dimensions, rollupDimensionArray...)

	metrics := make([]map[string]string, len(groupedMetric.Metrics))
	idx = 0
	for metricName, metricInfo := range groupedMetric.Metrics {
		metrics[idx] = map[string]string{
			"Name": metricName,
			"Unit": metricInfo.Unit,
		}
		idx++
	}

	return CWMeasurement{
		Namespace:  groupedMetric.Metadata.Namespace,
		Dimensions: dimensions,
		Metrics:    metrics,
	}
}

// groupedMetricToCWMeasurementsWithFilters filters the grouped metric using the given list of metric
// declarations and returns the corresponding list of CW Measurements.
func groupedMetricToCWMeasurementsWithFilters(groupedMetric *GroupedMetric, config *Config) (cWMeasurements []CWMeasurement) {
	labels := groupedMetric.Labels

	// Filter metric declarations by labels
	metricDeclarations := make([]*MetricDeclaration, 0, len(config.MetricDeclarations))
	for _, metricDeclaration := range config.MetricDeclarations {
		if metricDeclaration.MatchesLabels(labels) {
			metricDeclarations = append(metricDeclarations, metricDeclaration)
		}
	}

	// If the whole batch of metrics don't match any metric declarations, drop them
	if len(metricDeclarations) == 0 {
		labelsStr, _ := json.Marshal(labels)
		metricNames := make([]string, 0)
		for metricName := range groupedMetric.Metrics {
			metricNames = append(metricNames, metricName)
		}
		config.logger.Debug(
			"Dropped batch of metrics: no metric declaration matched labels",
			zap.String("Labels", string(labelsStr)),
			zap.Strings("Metric Names", metricNames),
		)
		return
	}

	// Group metrics by matched metric declarations
	type metricDeclarationGroup struct {
		metricDeclIdxList []int
		metrics           []map[string]string
	}

	metricDeclGroups := make(map[string]*metricDeclarationGroup)
	for metricName, metricInfo := range groupedMetric.Metrics {
		// Filter metric declarations by metric name
		var metricDeclIdx []int
		for i, metricDeclaration := range metricDeclarations {
			if metricDeclaration.MatchesName(metricName) {
				metricDeclIdx = append(metricDeclIdx, i)
			}
		}

		if len(metricDeclIdx) == 0 {
			config.logger.Debug(
				"Dropped metric: no metric declaration matched metric name",
				zap.String("Metric name", metricName),
			)
			continue
		}

		metric := map[string]string{
			"Name": metricName,
			"Unit": metricInfo.Unit,
		}
		metricDeclKey := fmt.Sprint(metricDeclIdx)
		if group, ok := metricDeclGroups[metricDeclKey]; ok {
			group.metrics = append(group.metrics, metric)
		} else {
			metricDeclGroups[metricDeclKey] = &metricDeclarationGroup{
				metricDeclIdxList: metricDeclIdx,
				metrics:           []map[string]string{metric},
			}
		}
	}

	if len(metricDeclGroups) == 0 {
		return
	}

	// Apply single/zero dimension rollup to labels
	rollupDimensionArray := dimensionRollup(config.DimensionRollupOption, labels)

	// Translate each group into a CW Measurement
	cWMeasurements = make([]CWMeasurement, 0, len(metricDeclGroups))
	for _, group := range metricDeclGroups {
		var dimensions [][]string
		// Extract dimensions from matched metric declarations
		for _, metricDeclIdx := range group.metricDeclIdxList {
			dims := metricDeclarations[metricDeclIdx].ExtractDimensions(labels)
			dimensions = append(dimensions, dims...)
		}
		dimensions = append(dimensions, rollupDimensionArray...)

		// De-duplicate dimensions
		dimensions = dedupDimensions(dimensions)

		// Export metrics only with non-empty dimensions list
		if len(dimensions) > 0 {
			cwm := CWMeasurement{
				Namespace:  groupedMetric.Metadata.Namespace,
				Dimensions: dimensions,
				Metrics:    group.metrics,
			}
			cWMeasurements = append(cWMeasurements, cwm)
		}
	}

	return
}
