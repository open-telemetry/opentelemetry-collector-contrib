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
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
		} else if strings.HasSuffix(curView.Name, "_notag") {
			assert.Equal(t, []tag.Key{}, curView.TagKeys)
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
			{MetricType: pdata.MetricDataTypeSummary}:                                                        1,
			{MetricType: pdata.MetricDataTypeHistogram}:                                                      1,
			{MetricType: pdata.MetricDataTypeSum, MetricTemporality: pdata.AggregationTemporalityDelta}:      2,
			{MetricType: pdata.MetricDataTypeSum, MetricTemporality: pdata.AggregationTemporalityCumulative}: 3,
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
			{attributeType: pdata.AttributeValueTypeArray, location: attributeLocationResource}:   1,
			{attributeType: pdata.AttributeValueTypeBool, location: attributeLocationSpan}:        1,
			{attributeType: pdata.AttributeValueTypeMap, location: attributeLocationSpanEvent}:    1,
			{attributeType: pdata.AttributeValueTypeDouble, location: attributeLocationLog}:       1,
			{attributeType: pdata.AttributeValueTypeInt, location: attributeLocationResource}:     1,
			{attributeType: pdata.AttributeValueTypeNull, location: attributeLocationSpan}:        1,
			{attributeType: pdata.AttributeValueTypeString, location: attributeLocationSpanEvent}: 1,
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
	assert.Equal(t, "eu01xxfoobarba", sanitizeAPIKeyForLogging("eu01xxfoobarbazqux"))
}

func TestMetadataHasDefaultValuesSet(t *testing.T) {
	m := initMetadata(context.Background(), "testdatatype")

	assert.Equal(t, "not_present", m.userAgent)
	assert.Equal(t, "not_present", m.apiKey)
	assert.Equal(t, "testdatatype", m.dataType)
	assert.NotNil(t, m.metricMetadataCount)
	assert.NotNil(t, m.spanMetadataCount)
	assert.NotNil(t, m.attributeMetadataCount)
}

func TestMetadataHasUserAgentWhenAvailable(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"user-agent": []string{"testuseragent"}})

	m := initMetadata(ctx, "testdatatype")

	assert.Equal(t, "testuseragent", m.userAgent)
}

func TestErrorsAreCombinedIntoSingleError(t *testing.T) {
	view.Unregister(MetricViews()...)
	if err := view.Register(MetricViews()...); err != nil {
		t.Fail()
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{"user-agent": []string{"testuseragent"}})

	// Tag values with length > 255 will generate an error when recording metrics
	b := make([]byte, 300)
	for i := 0; i < 300; i++ {
		b[i] = 'a'
	}
	reallyLongDataType := string(b)
	m := initMetadata(ctx, reallyLongDataType)
	m.metricMetadataCount[metricStatsKey{}]++
	m.spanMetadataCount[spanStatsKey{}]++
	m.attributeMetadataCount[attributeStatsKey{}]++

	err := m.recordMetrics(ctx)

	require.Error(t, err)
	// The bad tag value should result in 4 errors for each metric and there are 4 metrics
	assert.Equal(t, 8, len(strings.Split(err.Error(), ";")))
}
