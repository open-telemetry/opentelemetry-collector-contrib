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
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/mapwithexpiry"
)

const (
	CleanInterval = 5 * time.Minute
	MinTimeDiff   = 50 // We assume 50 micro-seconds is the minimal gap between two collected data sample to be valid to calculate delta

	//The following constants indicate the existence of resource attribute service.name and service.namespace
	ServiceNameOnly = iota
	ServiceNamespaceOnly
	ServiceNameAndNamespace
	ServiceNotDefined

	OtlibDimensionKey            = "OTLib"
	defaultNameSpace             = "default"
	noInstrumentationLibraryName = "Undefined"

	// See: http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	maximumLogEventsPerPut = 10000
)

var currentState = mapwithexpiry.NewMapWithExpiry(CleanInterval)

type rateState struct {
	value     interface{}
	timestamp int64
}

// CWMetrics defines
type CWMetrics struct {
	Measurements []CwMeasurement
	Timestamp    int64
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
	Count *wrappers.Int64Value
	Sum   *wrappers.DoubleValue
}

// TranslateOtToCWMetric converts OT metrics to CloudWatch Metric format
func TranslateOtToCWMetric(md *consumerdata.MetricsData) ([]*CWMetrics, int) {
	var cwMetricLists []*CWMetrics
	namespace := defaultNameSpace
	svcAttrMode := ServiceNotDefined
	totalDroppedMetrics := 0

	if attributes := md.Resource.GetLabels(); attributes != nil {
		serviceName, svcNameOk := attributes[conventions.AttributeServiceName]
		serviceNamespace, svcNsOk := attributes[conventions.AttributeServiceNamespace]

		if svcNameOk {
			svcAttrMode = ServiceNameOnly
		}
		if svcNsOk {
			if ServiceNameOnly == svcAttrMode {
				svcAttrMode = ServiceNameAndNamespace
			} else {
				svcAttrMode = ServiceNamespaceOnly
			}
		}
		switch svcAttrMode {
		case ServiceNameAndNamespace:
			namespace = fmt.Sprintf("%s/%s", serviceNamespace, serviceName)
		case ServiceNameOnly:
			namespace = serviceName
		case ServiceNamespaceOnly:
			namespace = serviceNamespace
		case ServiceNotDefined:
		default:
		}
	}
	for _, metric := range md.Metrics {
		if metric == nil || metric.MetricDescriptor == nil {
			totalDroppedMetrics += len(metric.GetTimeseries())
			continue
		}
		//TODO: Handle OTLib as a dimension when it's supported
		OTLib := noInstrumentationLibraryName
		cwMetricList := getMeasurements(metric, namespace, OTLib)
		cwMetricLists = append(cwMetricLists, cwMetricList...)
	}
	return cwMetricLists, totalDroppedMetrics
}

func TranslateCWMetricToEMF(cwMetricLists []*CWMetrics) []*LogEvent {
	// convert CWMetric into map format for compatible with PLE input
	ples := make([]*LogEvent, 0, maximumLogEventsPerPut)
	for _, met := range cwMetricLists {
		cwmMap := make(map[string]interface{})
		fieldMap := met.Fields
		cwmMap["CloudWatchMetrics"] = met.Measurements
		cwmMap["Timestamp"] = met.Timestamp
		fieldMap["_aws"] = cwmMap

		pleMsg, err := json.Marshal(fieldMap)
		if err != nil {
			continue
		}
		metricCreationTime := met.Timestamp

		logEvent := NewLogEvent(
			metricCreationTime,
			string(pleMsg),
			"",
			0,
		)
		logEvent.LogGeneratedTime = time.Unix(0, metricCreationTime*int64(time.Millisecond))
		ples = append(ples, logEvent)
	}
	return ples
}

func getMeasurements(metric *metricspb.Metric, namespace string, OTLib string) []*CWMetrics {
	var result []*CWMetrics
	// metric measure data from OT
	metricMeasure := make(map[string]string)

	descriptor := metric.MetricDescriptor

	// metric measure slice could include multiple metric measures
	metricSlice := []map[string]string{}
	metricMeasure["Name"] = descriptor.Name
	metricMeasure["Unit"] = descriptor.Unit
	metricSlice = append(metricSlice, metricMeasure)

	for _, series := range metric.Timeseries {
		// fields contains metric and dimensions key/value pairs
		fieldsPairs := make(map[string]interface{})
		var dimensionArray [][]string
		// Dimensions Slice
		var dimensionSlice []string

		labelCount := len(descriptor.LabelKeys)
		if len(series.LabelValues) < len(descriptor.LabelKeys) {
			labelCount = len(series.LabelValues)
		}
		for i := 0; i < labelCount; i++ {
			fieldsPairs[descriptor.LabelKeys[i].GetKey()] = series.LabelValues[i].GetValue()
			dimensionSlice = append(dimensionSlice, descriptor.LabelKeys[i].GetKey())
		}
		// add OTLib as an additional dimension
		fieldsPairs[OtlibDimensionKey] = OTLib
		dimensionArray = append(dimensionArray, append(dimensionSlice, OtlibDimensionKey))

		for _, dp := range series.Points {
			timestamp := time.Now().UnixNano() / int64(time.Millisecond)
			switch pv := dp.Value.(type) {
			case *metricspb.Point_Int64Value:
				fieldsPairs[descriptor.Name] = pv.Int64Value
				metricVal := calculateRate(fieldsPairs, pv.Int64Value, timestamp)
				fieldsPairs[descriptor.Name] = metricVal
			case *metricspb.Point_DoubleValue:
				fieldsPairs[descriptor.Name] = pv.DoubleValue
				metricVal := calculateRate(fieldsPairs, pv.DoubleValue, timestamp)
				fieldsPairs[descriptor.Name] = metricVal
			case *metricspb.Point_DistributionValue:
				//TODO: need to support distribution value in next phase
				continue
			case *metricspb.Point_SummaryValue:
				snapShot := pv.SummaryValue.GetSnapshot()
				summaryValueAtPercentile := snapShot.GetPercentileValues()
				metricStats := &CWMetricStats{
					Min:   summaryValueAtPercentile[0].GetValue(),
					Max:   summaryValueAtPercentile[len(summaryValueAtPercentile)-1].GetValue(),
					Count: snapShot.GetCount(),
					Sum:   snapShot.GetSum(),
				}
				fieldsPairs[descriptor.Name] = metricStats
				continue
			default:
				continue
			}

			// EMF dimension attr takes list of list on dimensions. Including single/zero dimension rollup
			//"Zero" dimension rollup
			dimensionZero := []string{OtlibDimensionKey}
			if len(dimensionSlice) > 0 {
				dimensionArray = append(dimensionArray, dimensionZero)
			}
			//"One" dimension rollup
			for _, dimensionKey := range dimensionSlice {
				dimensionArray = append(dimensionArray, append(dimensionZero, dimensionKey))
			}

			cwMeasurement := &CwMeasurement{
				Namespace:  namespace,
				Dimensions: dimensionArray,
				Metrics:    metricSlice,
			}
			metricList := make([]CwMeasurement, 1)
			metricList[0] = *cwMeasurement
			cwMetric := &CWMetrics{
				Measurements: metricList,
				Timestamp:    timestamp,
				Fields:       fieldsPairs,
			}
			result = append(result, cwMetric)
		}
	}
	return result
}

// rate is calculated by valDelta / timeDelta
func calculateRate(fields map[string]interface{}, val interface{}, timestamp int64) interface{} {
	keys := make([]string, 0, len(fields))
	var b bytes.Buffer
	var metricRate interface{}
	// hash the key of str: metric + dimension key/value pairs (sorted alpha)
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch v := fields[k].(type) {
		case int64:
			b.WriteString(k)
			continue
		case string:
			b.WriteString(k)
			b.WriteString(v)
		default:
			continue
		}
	}
	h := sha1.New()
	h.Write(b.Bytes())
	bs := h.Sum(nil)
	hashStr := string(bs)

	// get previous Metric content from map. Need to lock the map until set the new state
	currentState.Lock()
	if state, ok := currentState.Get(hashStr); ok {
		prevStats := state.(*rateState)
		deltaTime := timestamp - prevStats.timestamp
		var deltaVal interface{}
		if _, ok := val.(float64); ok {
			deltaVal = val.(float64) - prevStats.value.(float64)
			if deltaTime > MinTimeDiff && deltaVal.(float64) >= 0 {
				metricRate = deltaVal.(float64) * 1e3 / float64(deltaTime)
			}
		} else {
			deltaVal = val.(int64) - prevStats.value.(int64)
			if deltaTime > MinTimeDiff && deltaVal.(int64) >= 0 {
				metricRate = deltaVal.(int64) * 1e3 / deltaTime
			}
		}
	}
	content := &rateState{
		value:     val,
		timestamp: timestamp,
	}
	currentState.Set(hashStr, content)
	currentState.Unlock()
	if metricRate == nil {
		metricRate = 0
	}
	return metricRate
}
