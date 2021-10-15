// Copyright  The OpenTelemetry Authors
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

package metadata

import (
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
)

const (
	projectID    = "ProjectID"
	instanceID   = "InstanceID"
	databaseName = "DatabaseName"
)

func databaseID() *datasource.DatabaseID {
	return datasource.NewDatabaseID(projectID, instanceID, databaseName)
}

func TestMetricsMetadata_Timestamp(t *testing.T) {
	testCases := map[string]struct {
		metadata        *MetricsMetadata
		rowColumnNames  []string
		rowColumnValues []interface{}
		errorRequired   bool
	}{
		"Happy path":               {&MetricsMetadata{TimestampColumnName: timestampColumnName}, []string{timestampColumnName}, []interface{}{time.Now().UTC()}, false},
		"No timestamp column name": {&MetricsMetadata{}, []string{}, []interface{}{}, false},
		"With error":               {&MetricsMetadata{TimestampColumnName: "nonExistingColumn"}, []string{timestampColumnName}, []interface{}{time.Now().UTC()}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			row, _ := spanner.NewRow(testCase.rowColumnNames, testCase.rowColumnValues)

			timestamp, err := testCase.metadata.timestamp(row)

			if testCase.errorRequired {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, timestamp)
				assert.False(t, timestamp.IsZero())

				if len(testCase.rowColumnValues) == 1 {
					assert.Equal(t, testCase.rowColumnValues[0], timestamp)
				}
			}
		})
	}
}

func TestToLabelValue(t *testing.T) {
	labelValueMetadata := newQueryLabelValueMetadata(labelName, labelColumnName)
	rowColumnNames := []string{labelColumnName}
	testCases := map[string]struct {
		metadata                 LabelValueMetadata
		expectedType             LabelValueMetadata
		expectedValue            interface{}
		expectedTransformedValue interface{}
	}{
		"String label value metadata":       {StringLabelValueMetadata{queryLabelValueMetadata: labelValueMetadata}, stringLabelValue{}, stringValue, nil},
		"Int64 label value metadata":        {Int64LabelValueMetadata{queryLabelValueMetadata: labelValueMetadata}, int64LabelValue{}, int64Value, nil},
		"Bool label value metadata":         {BoolLabelValueMetadata{queryLabelValueMetadata: labelValueMetadata}, boolLabelValue{}, boolValue, nil},
		"String slice label value metadata": {StringSliceLabelValueMetadata{queryLabelValueMetadata: labelValueMetadata}, stringSliceLabelValue{}, []string{stringValue, stringValue}, stringValue + "," + stringValue},
		"Byte slice label value metadata":   {ByteSliceLabelValueMetadata{queryLabelValueMetadata: labelValueMetadata}, byteSliceLabelValue{}, []byte(stringValue), stringValue},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			row, _ := spanner.NewRow(rowColumnNames, []interface{}{testCase.expectedValue})

			labelValue, _ := toLabelValue(testCase.metadata, row)

			assert.IsType(t, testCase.expectedType, labelValue)
			assert.Equal(t, labelName, labelValue.Name())
			assert.Equal(t, labelColumnName, labelValue.ColumnName())
			if testCase.expectedTransformedValue != nil {
				assert.Equal(t, testCase.expectedTransformedValue, labelValue.Value())
			} else {
				assert.Equal(t, testCase.expectedValue, labelValue.Value())
			}
		})
	}
}

func TestMetricsMetadata_ToLabelValues_AllPossibleMetadata(t *testing.T) {
	stringLabelValueMetadata := StringLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata("stringLabelName", "stringLabelColumnName"),
	}
	boolLabelValueMetadata := BoolLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata("boolLabelName", "boolLabelColumnName"),
	}
	int64LabelValueMetadata := Int64LabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata("int64LabelName", "int64LabelColumnName"),
	}
	stringSliceLabelValueMetadata := StringSliceLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata("stringSliceLabelName", "stringSliceLabelColumnName"),
	}
	byteSliceLabelValueMetadata := ByteSliceLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata("byteSliceLabelName", "byteSliceLabelColumnName"),
	}
	queryLabelValuesMetadata := []LabelValueMetadata{
		stringLabelValueMetadata,
		boolLabelValueMetadata,
		int64LabelValueMetadata,
		stringSliceLabelValueMetadata,
		byteSliceLabelValueMetadata,
	}
	metadata := MetricsMetadata{QueryLabelValuesMetadata: queryLabelValuesMetadata}
	row, _ := spanner.NewRow(
		[]string{
			stringLabelValueMetadata.columnName,
			boolLabelValueMetadata.columnName,
			int64LabelValueMetadata.columnName,
			stringSliceLabelValueMetadata.columnName,
			byteSliceLabelValueMetadata.columnName,
		},
		[]interface{}{
			stringValue,
			boolValue,
			int64Value,
			[]string{stringValue, stringValue},
			[]byte(stringValue),
		})

	labelValues, _ := metadata.toLabelValues(row)

	assert.Equal(t, len(queryLabelValuesMetadata), len(labelValues))

	expectedTypes := []LabelValue{
		stringLabelValue{},
		boolLabelValue{},
		int64LabelValue{},
		stringSliceLabelValue{},
		byteSliceLabelValue{},
	}

	for i, expectedType := range expectedTypes {
		assert.IsType(t, expectedType, labelValues[i])
	}
}

func TestMetricsMetadata_ToLabelValues_Error(t *testing.T) {
	stringLabelValueMetadata := StringLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata("nonExisting", "nonExistingColumn"),
	}
	queryLabelValuesMetadata := []LabelValueMetadata{stringLabelValueMetadata}
	metadata := MetricsMetadata{QueryLabelValuesMetadata: queryLabelValuesMetadata}
	row, _ := spanner.NewRow([]string{}, []interface{}{})

	labelValues, err := metadata.toLabelValues(row)

	assert.Nil(t, labelValues)
	require.Error(t, err)
}

func TestToMetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	metricValueMetadata := newQueryMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit)
	rowColumnNames := []string{metricColumnName}
	testCases := map[string]struct {
		metadata      MetricValueMetadata
		expectedType  MetricValueMetadata
		expectedValue interface{}
	}{
		"Int64 label value metadata": {Int64MetricValueMetadata{queryMetricValueMetadata: metricValueMetadata}, int64MetricValue{}, int64Value},
		"Bool label value metadata":  {Float64MetricValueMetadata{queryMetricValueMetadata: metricValueMetadata}, float64MetricValue{}, float64Value},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			row, _ := spanner.NewRow(rowColumnNames, []interface{}{testCase.expectedValue})

			metricValue, _ := toMetricValue(testCase.metadata, row)

			assert.IsType(t, testCase.expectedType, metricValue)
			assert.Equal(t, metricName, metricValue.Name())
			assert.Equal(t, metricColumnName, metricValue.ColumnName())
			assert.Equal(t, metricDataType, metricValue.DataType())
			assert.Equal(t, metricUnit, metricValue.Unit())
			assert.Equal(t, testCase.expectedValue, metricValue.Value())
		})
	}
}

func TestMetricsMetadata_ToMetricValues_AllPossibleMetadata(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	int64MetricValueMetadata := Int64MetricValueMetadata{
		queryMetricValueMetadata: newQueryMetricValueMetadata("int64MetricName",
			"int64MetricColumnName", metricDataType, metricUnit),
	}
	float64MetricValueMetadata := Float64MetricValueMetadata{
		queryMetricValueMetadata: newQueryMetricValueMetadata("float64MetricName",
			"float64MetricColumnName", metricDataType, metricUnit),
	}
	queryMetricValuesMetadata := []MetricValueMetadata{
		int64MetricValueMetadata,
		float64MetricValueMetadata,
	}
	metadata := MetricsMetadata{QueryMetricValuesMetadata: queryMetricValuesMetadata}
	row, _ := spanner.NewRow(
		[]string{int64MetricValueMetadata.columnName, float64MetricValueMetadata.columnName},
		[]interface{}{int64Value, float64Value})

	metricValues, _ := metadata.toMetricValues(row)

	assert.Equal(t, len(queryMetricValuesMetadata), len(metricValues))

	expectedTypes := []MetricValue{int64MetricValue{}, float64MetricValue{}}

	for i, expectedType := range expectedTypes {
		assert.IsType(t, expectedType, metricValues[i])
	}
}

func TestMetricsMetadata_ToMetricValues_Error(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	int64MetricValueMetadata := Int64MetricValueMetadata{
		queryMetricValueMetadata: newQueryMetricValueMetadata("nonExistingMetricName",
			"nonExistingMetricColumnName", metricDataType, metricUnit),
	}
	queryMetricValuesMetadata := []MetricValueMetadata{int64MetricValueMetadata}
	metadata := MetricsMetadata{QueryMetricValuesMetadata: queryMetricValuesMetadata}
	row, _ := spanner.NewRow([]string{}, []interface{}{})

	metricValues, err := metadata.toMetricValues(row)

	assert.Nil(t, metricValues)
	require.Error(t, err)
}

func TestMetricsMetadata_RowToMetrics(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	timestamp := time.Now().UTC()
	labelValueMetadata := StringLabelValueMetadata{
		queryLabelValueMetadata: newQueryLabelValueMetadata(labelName, labelColumnName),
	}
	metricValueMetadata := Int64MetricValueMetadata{
		queryMetricValueMetadata: newQueryMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit),
	}
	queryLabelValuesMetadata := []LabelValueMetadata{labelValueMetadata}
	queryMetricValuesMetadata := []MetricValueMetadata{metricValueMetadata}
	databaseID := databaseID()
	metadata := MetricsMetadata{
		MetricNamePrefix:          metricNamePrefix,
		TimestampColumnName:       timestampColumnName,
		QueryLabelValuesMetadata:  queryLabelValuesMetadata,
		QueryMetricValuesMetadata: queryMetricValuesMetadata,
	}
	testCases := map[string]struct {
		rowColumnNames  []string
		rowColumnValues []interface{}
		expectError     bool
	}{
		"Happy path":            {[]string{labelColumnName, metricColumnName, timestampColumnName}, []interface{}{stringValue, int64Value, timestamp}, false},
		"Error on timestamp":    {[]string{labelColumnName, metricColumnName}, []interface{}{stringValue, int64Value}, true},
		"Error on label value":  {[]string{metricColumnName, timestampColumnName}, []interface{}{int64Value, timestamp}, true},
		"Error on metric value": {[]string{labelColumnName, timestampColumnName}, []interface{}{stringValue, timestamp}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			row, _ := spanner.NewRow(testCase.rowColumnNames, testCase.rowColumnValues)
			metrics, err := metadata.RowToMetrics(databaseID, row)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, len(metrics))
			}
		})
	}
}

func TestMetricsMetadata_MetadataType(t *testing.T) {
	testCases := map[string]struct {
		timestampColumnName  string
		expectedMetadataType MetricsMetadataType
	}{
		"Current stats metadata":  {"", MetricsMetadataTypeCurrentStats},
		"Interval stats metadata": {"timestampColumnName", MetricsMetadataTypeIntervalStats},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metricsMetadata := &MetricsMetadata{TimestampColumnName: testCase.timestampColumnName}

			assert.Equal(t, testCase.expectedMetadataType, metricsMetadata.MetadataType())
		})
	}
}
