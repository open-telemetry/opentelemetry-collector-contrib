// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecreceiver

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

var (
	errSplunkNoDatumValue         = errors.New("no datum value for data-point")
	errSplunkStringDatumNotNumber = errors.New("datum string cannot be parsed to a number")
)

// SplunkHecToMetricsData converts Splunk HEC metric points to
// consumerdata.MetricsData. Returning the converted data and the number of
// dropped time series.
func SplunkHecToMetricsData(
	logger *zap.Logger,
	splunkHecDataPoints *splunk.Metric,
) (*consumerdata.MetricsData, int) {

	// TODO: not optimized at all, basically regenerating everything for each
	// 	data point.
	numDroppedTimeSeries := 0
	md := consumerdata.MetricsData{}
	metrics := make([]*metricspb.Metric, 0, len(splunkHecDataPoints.GetValues()))

	// TODO currently mapping all values to unspecified.
	metricType := metricspb.MetricDescriptor_UNSPECIFIED

	labelKeys, labelValues := buildLabelKeysAndValues(splunkHecDataPoints.Fields)

	values := splunkHecDataPoints.GetValues()
	metricNames := make([]string, 0, len(values))
	for k := range values {
		metricNames = append(metricNames, k)
	}
	sort.Strings(metricNames)

	for _, metricName := range metricNames {
		point, err := buildPoint(logger, splunkHecDataPoints, metricName, numDroppedTimeSeries)
		if err != nil {
			logger.Debug("Splunk data-point datum conversion error",
				zap.Error(err))
			continue
		}
		pointTimestamp := convertTimestamp(splunkHecDataPoints.Time)
		ts := &metricspb.TimeSeries{
			StartTimestamp: pointTimestamp,
			LabelValues:    labelValues,
			Points:         []*metricspb.Point{point},
		}
		descriptor := buildDescriptor(metricName, labelKeys, metricType)
		metric := &metricspb.Metric{
			MetricDescriptor: descriptor,
			Timeseries:       []*metricspb.TimeSeries{ts},
		}
		metrics = append(metrics, metric)
	}

	md.Metrics = metrics
	return &md, numDroppedTimeSeries
}

func buildPoint(logger *zap.Logger,
	splunkDataPoint *splunk.Metric,
	metricName string,
	numDroppedTimeSeries int) (*metricspb.Point, error) {
	pointTimestamp := convertTimestamp(splunkDataPoint.Time)

	metricValue := splunkDataPoint.GetValues()[metricName]
	point := &metricspb.Point{
		Timestamp: pointTimestamp,
	}
	if i, ok := metricValue.(int64); ok {
		point.Value = &metricspb.Point_Int64Value{Int64Value: i}
	} else if f, ok := metricValue.(float64); ok {
		point.Value = &metricspb.Point_DoubleValue{DoubleValue: f}
	} else if i, ok := metricValue.(*int64); ok {
		point.Value = &metricspb.Point_Int64Value{Int64Value: *i}
	} else if f, ok := metricValue.(*float64); ok {
		point.Value = &metricspb.Point_DoubleValue{DoubleValue: *f}
	} else if s, ok := metricValue.(*string); ok {
		// best effort, cast to string and turn into a number
		dbl, err := strconv.ParseFloat(*s, 64)
		if err != nil {
			return nil, errSplunkStringDatumNotNumber
		}
		point.Value = &metricspb.Point_DoubleValue{DoubleValue: dbl}
	} else {
		// drop this point as we do not know how to extract a value from it
		numDroppedTimeSeries++
		logger.Debug("Cannot convert metric",
			zap.String("metric", metricName))
		return nil, errSplunkNoDatumValue
	}
	// TODO add summary value mapping
	// TODO add distribution value mapping

	return point, nil
}

func convertTimestamp(sec float64) *timestamp.Timestamp {
	if sec == 0 {
		return nil
	}

	ts := &timestamp.Timestamp{
		Seconds: int64(sec),
		Nanos:   int32(int64(sec*1e3)%1e3) * 1e6,
	}
	return ts
}

func buildDescriptor(
	metricName string,
	labelKeys []*metricspb.LabelKey,
	metricType metricspb.MetricDescriptor_Type,
) *metricspb.MetricDescriptor {

	descriptor := &metricspb.MetricDescriptor{
		Name: metricName,
		// Description: no value to go here
		// Unit:        no value to go here
		Type:      metricType,
		LabelKeys: labelKeys,
	}

	return descriptor
}

func buildLabelKeysAndValues(
	dimensions map[string]interface{},
) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	keys := make([]*metricspb.LabelKey, 0, len(dimensions))
	values := make([]*metricspb.LabelValue, 0, len(dimensions))
	dimensionKeys := make([]string, 0, len(dimensions))
	for key := range dimensions {
		dimensionKeys = append(dimensionKeys, key)
	}
	sort.Strings(dimensionKeys)
	for _, key := range dimensionKeys {

		if strings.HasPrefix(key, "metric_name") {
			continue
		}
		if key == "" || dimensions[key] == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		lk := &metricspb.LabelKey{Key: key}
		keys = append(keys, lk)

		lv := &metricspb.LabelValue{}
		lv.Value = fmt.Sprintf("%v", dimensions[key])
		lv.HasValue = true
		values = append(values, lv)
	}
	return keys, values
}
