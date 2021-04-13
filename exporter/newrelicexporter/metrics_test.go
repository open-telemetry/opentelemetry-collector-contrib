// Copyright The OpenTelemetry Authors
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

package newrelicexporter

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/consumer/pdata"
	"google.golang.org/grpc/codes"
)

func TestMetricViews(t *testing.T) {
	metricViews := MetricViews()

	assert.True(t, len(metricViews) > 0)
	for _, curView := range metricViews {
		assert.True(t, strings.HasPrefix(curView.Name, "newrelicexporter_"))
		assert.NotNil(t, curView.Aggregation)
		assert.NotNil(t, curView.Description)
		if curView.Name == "newrelicexporter_metric_metadata_count" {
			assert.Equal(t, metricMetadataTagKeys, curView.TagKeys)
		} else if curView.Name == "newrelicexporter_span_metadata_count" {
			assert.Equal(t, spanMetadataTagKeys, curView.TagKeys)
		} else if curView.Name == "newrelicexporter_attribute_metadata_count" {
			assert.Equal(t, attributeMetadataTagKeys, curView.TagKeys)
		} else {
			assert.Equal(t, tagKeys, curView.TagKeys)
		}
		assert.NotNil(t, curView.Aggregation)
	}
}

func TestRecordMetrics(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	details := []exportMetadata{
		// A request that completes normally
		{
			grpcResponseCode: codes.OK,
			httpStatusCode:   200,
			apiKey:           "shhh",
			userAgent:        "secret agent",
			dataType:         "data",
			dataInputCount:   2,
			exporterTime:     100,
			dataOutputCount:  20,
			externalDuration: 50,
		},
		// A request that receives 403 status code from the HTTP API
		{
			grpcResponseCode: codes.Unauthenticated,
			httpStatusCode:   403,
			apiKey:           "shhh",
			userAgent:        "secret agent",
			dataType:         "data",
			dataInputCount:   2,
			exporterTime:     100,
			dataOutputCount:  20,
			externalDuration: 50,
		},
		// A request experiences a url.Error while sending to the HTTP API
		{
			grpcResponseCode: codes.DataLoss,
			httpStatusCode:   0,
			apiKey:           "shhh",
			userAgent:        "secret agent",
			dataType:         "data",
			dataInputCount:   2,
			exporterTime:     100,
			dataOutputCount:  20,
			externalDuration: 50,
		},
	}

	for _, traceDetails := range details {
		if err := traceDetails.recordMetrics(context.TODO()); err != nil {
			t.Fail()
		}
	}

	measurements := []stats.Measure{
		statRequestCount,
		statOutputDatapointCount,
		statExporterTime,
		statExternalTime,
	}

	for _, measurement := range measurements {
		rows, err := view.RetrieveData(measurement.Name())
		if err != nil {
			t.Fail()
		}
		// Check that each measurement has a number of rows corresponding to the tag set produced by the interactions
		assert.Equal(t, len(details), len(rows))
		for _, row := range rows {
			// Confirm each row has data and has the required tag keys
			assert.True(t, row.Data != nil)
			assert.Equal(t, len(tagKeys), len(row.Tags))
			for _, rowTag := range row.Tags {
				assert.Contains(t, tagKeys, rowTag.Key)
			}
		}
	}
}

func TestRecordMetricMetadata(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	detail := exportMetadata{
		grpcResponseCode: codes.OK,
		httpStatusCode:   200,
		apiKey:           "shhh",
		userAgent:        "secret agent",
		dataType:         "metric",
		dataInputCount:   2,
		exporterTime:     100,
		dataOutputCount:  20,
		externalDuration: 50,
		metricMetadataCount: map[metricStatsKey]int{
			{MetricType: pdata.MetricDataTypeSummary}:                                                              1,
			{MetricType: pdata.MetricDataTypeHistogram}:                                                            1,
			{MetricType: pdata.MetricDataTypeDoubleSum, MetricTemporality: pdata.AggregationTemporalityDelta}:      2,
			{MetricType: pdata.MetricDataTypeDoubleSum, MetricTemporality: pdata.AggregationTemporalityCumulative}: 3,
		},
	}

	if err := detail.recordMetrics(context.TODO()); err != nil {
		t.Fail()
	}

	rows, err := view.RetrieveData(statMetricMetadata.Name())
	if err != nil {
		t.Fail()
	}
	// Check that the measurement has the right number of results recorded
	assert.Equal(t, len(detail.metricMetadataCount), len(rows))
	for _, row := range rows {
		// Confirm each row has data and has the required tag keys
		assert.True(t, row.Data != nil)
		assert.Equal(t, len(metricMetadataTagKeys), len(row.Tags))
		for _, rowTag := range row.Tags {
			assert.Contains(t, metricMetadataTagKeys, rowTag.Key)
		}
	}
}

func TestDoesNotRecordMetricMetadata(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	detail := exportMetadata{
		grpcResponseCode: codes.OK,
		httpStatusCode:   200,
		apiKey:           "shhh",
		userAgent:        "secret agent",
		dataType:         "metric",
		dataInputCount:   2,
		exporterTime:     100,
		dataOutputCount:  20,
		externalDuration: 50,
	}

	if err := detail.recordMetrics(context.TODO()); err != nil {
		t.Fail()
	}

	rows, err := view.RetrieveData(statMetricMetadata.Name())
	if err != nil {
		t.Fail()
	}
	// No results should have been recorded
	assert.Equal(t, 0, len(rows))
}

func TestRecordSpanMetadata(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	detail := exportMetadata{
		grpcResponseCode: codes.OK,
		httpStatusCode:   200,
		apiKey:           "shhh",
		userAgent:        "secret agent",
		dataType:         "metric",
		dataInputCount:   2,
		exporterTime:     100,
		dataOutputCount:  20,
		externalDuration: 50,
		spanMetadataCount: map[spanStatsKey]int{
			{hasEvents: false, hasLinks: false}: 1,
			{hasEvents: true, hasLinks: false}:  1,
			{hasEvents: false, hasLinks: true}:  2,
			{hasEvents: true, hasLinks: true}:   3,
		},
	}

	if err := detail.recordMetrics(context.TODO()); err != nil {
		t.Fail()
	}

	rows, err := view.RetrieveData(statSpanMetadata.Name())
	if err != nil {
		t.Fail()
	}
	// Check that the measurement has the right number of results recorded
	assert.Equal(t, len(detail.spanMetadataCount), len(rows))
	for _, row := range rows {
		// Confirm each row has data and has the required tag keys
		assert.True(t, row.Data != nil)
		assert.Equal(t, len(spanMetadataTagKeys), len(row.Tags))
		for _, rowTag := range row.Tags {
			assert.Contains(t, spanMetadataTagKeys, rowTag.Key)
		}
	}
}

func TestDoesNotRecordSpanMetadata(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	detail := exportMetadata{
		grpcResponseCode: codes.OK,
		httpStatusCode:   200,
		apiKey:           "shhh",
		userAgent:        "secret agent",
		dataType:         "metric",
		dataInputCount:   2,
		exporterTime:     100,
		dataOutputCount:  20,
		externalDuration: 50,
	}

	if err := detail.recordMetrics(context.TODO()); err != nil {
		t.Fail()
	}

	rows, err := view.RetrieveData(statSpanMetadata.Name())
	if err != nil {
		t.Fail()
	}
	// No results should have been recorded
	assert.Equal(t, 0, len(rows))
}

func TestRecordAttributeMetadata(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	detail := exportMetadata{
		grpcResponseCode: codes.OK,
		httpStatusCode:   200,
		apiKey:           "shhh",
		userAgent:        "secret agent",
		dataType:         "data",
		dataInputCount:   2,
		exporterTime:     100,
		dataOutputCount:  20,
		externalDuration: 50,
		attributeMetadataCount: map[attributeStatsKey]int{
			{attributeType: pdata.AttributeValueARRAY, location: attributeLocationResource}:   1,
			{attributeType: pdata.AttributeValueBOOL, location: attributeLocationSpan}:        1,
			{attributeType: pdata.AttributeValueMAP, location: attributeLocationSpanEvent}:    1,
			{attributeType: pdata.AttributeValueDOUBLE, location: attributeLocationLog}:       1,
			{attributeType: pdata.AttributeValueINT, location: attributeLocationResource}:     1,
			{attributeType: pdata.AttributeValueNULL, location: attributeLocationSpan}:        1,
			{attributeType: pdata.AttributeValueSTRING, location: attributeLocationSpanEvent}: 1,
		},
	}

	if err := detail.recordMetrics(context.TODO()); err != nil {
		t.Fail()
	}

	rows, err := view.RetrieveData(statAttributeMetadata.Name())
	if err != nil {
		t.Fail()
	}
	// Check that the measurement has the right number of results recorded
	assert.Equal(t, len(detail.attributeMetadataCount), len(rows))
	for _, row := range rows {
		// Confirm each row has data and has the required tag keys
		assert.True(t, row.Data != nil)
		assert.Equal(t, len(attributeMetadataTagKeys), len(row.Tags))
		for _, rowTag := range row.Tags {
			assert.Contains(t, attributeMetadataTagKeys, rowTag.Key)
		}
	}
}

func TestDoesNotRecordAttributeMetadata(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	detail := exportMetadata{
		grpcResponseCode: codes.OK,
		httpStatusCode:   200,
		apiKey:           "shhh",
		userAgent:        "secret agent",
		dataType:         "metric",
		dataInputCount:   2,
		exporterTime:     100,
		dataOutputCount:  20,
		externalDuration: 50,
	}

	if err := detail.recordMetrics(context.TODO()); err != nil {
		t.Fail()
	}

	rows, err := view.RetrieveData(statAttributeMetadata.Name())
	if err != nil {
		t.Fail()
	}
	// No results should have been recorded
	assert.Equal(t, 0, len(rows))
}

func TestAttributeLocationString(t *testing.T) {
	locations := []attributeLocation{
		attributeLocationResource,
		attributeLocationSpan,
		attributeLocationSpanEvent,
		attributeLocationLog,
		99,
	}

	expectedStrings := []string{
		"resource",
		"span",
		"span_event",
		"log",
		"",
	}

	for i := 0; i < len(locations); i++ {
		assert.Equal(t, expectedStrings[i], locations[i].String())
	}
}

func TestSanitizeApiKeyForLogging(t *testing.T) {
	assert.Equal(t, "", sanitizeAPIKeyForLogging(""))
	assert.Equal(t, "foo", sanitizeAPIKeyForLogging("foo"))
	assert.Equal(t, "foobarba", sanitizeAPIKeyForLogging("foobarbazqux"))
}
