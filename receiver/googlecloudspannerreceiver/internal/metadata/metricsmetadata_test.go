// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
		rowColumnValues []any
		errorRequired   bool
	}{
		"Happy path":               {&MetricsMetadata{TimestampColumnName: timestampColumnName}, []string{timestampColumnName}, []any{time.Now().UTC()}, false},
		"No timestamp column name": {&MetricsMetadata{}, []string{}, []any{}, false},
		"With error":               {&MetricsMetadata{TimestampColumnName: "nonExistingColumn"}, []string{timestampColumnName}, []any{time.Now().UTC()}, true},
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
	rowColumnNames := []string{labelColumnName}
	testCases := map[string]struct {
		valueType                ValueType
		expectedType             LabelValue
		expectedValue            any
		expectedTransformedValue any
	}{
		"String label value metadata":             {StringValueType, stringLabelValue{}, stringValue, nil},
		"Int64 label value metadata":              {IntValueType, int64LabelValue{}, int64Value, nil},
		"Bool label value metadata":               {BoolValueType, boolLabelValue{}, boolValue, nil},
		"String slice label value metadata":       {StringSliceValueType, stringSliceLabelValue{}, []string{stringValue, stringValue}, stringValue + "," + stringValue},
		"Byte slice label value metadata":         {ByteSliceValueType, byteSliceLabelValue{}, []byte(stringValue), stringValue},
		"Lock request slice label value metadata": {LockRequestSliceValueType, lockRequestSliceLabelValue{}, []*lockRequest{{"lockMode", "column", "transactionTag"}}, "{lockMode,column,transactionTag}"},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			row, _ := spanner.NewRow(rowColumnNames, []any{testCase.expectedValue})
			metadata, _ := NewLabelValueMetadata(labelName, labelColumnName, testCase.valueType)

			labelValue, _ := toLabelValue(metadata, row)

			assert.IsType(t, testCase.expectedType, labelValue)
			assert.Equal(t, labelName, labelValue.Metadata().Name())
			assert.Equal(t, labelColumnName, labelValue.Metadata().ColumnName())
			if testCase.expectedTransformedValue != nil {
				assert.Equal(t, testCase.expectedTransformedValue, labelValue.Value())
			} else {
				assert.Equal(t, testCase.expectedValue, labelValue.Value())
			}
		})
	}
}

func TestMetricsMetadata_ToLabelValues_AllPossibleMetadata(t *testing.T) {
	stringLabelValueMetadata, _ := NewLabelValueMetadata("stringLabelName", "stringLabelColumnName", StringValueType)
	boolLabelValueMetadata, _ := NewLabelValueMetadata("boolLabelName", "boolLabelColumnName", BoolValueType)
	int64LabelValueMetadata, _ := NewLabelValueMetadata("int64LabelName", "int64LabelColumnName", IntValueType)
	stringSliceLabelValueMetadata, _ := NewLabelValueMetadata("stringSliceLabelName", "stringSliceLabelColumnName", StringSliceValueType)
	byteSliceLabelValueMetadata, _ := NewLabelValueMetadata("byteSliceLabelName", "byteSliceLabelColumnName", ByteSliceValueType)
	lockRequestSliceLabelValueMetadata, _ := NewLabelValueMetadata("lockRequestSliceLabelName", "lockRequestSliceLabelColumnName", LockRequestSliceValueType)
	queryLabelValuesMetadata := []LabelValueMetadata{
		stringLabelValueMetadata,
		boolLabelValueMetadata,
		int64LabelValueMetadata,
		stringSliceLabelValueMetadata,
		byteSliceLabelValueMetadata,
		lockRequestSliceLabelValueMetadata,
	}
	metadata := MetricsMetadata{QueryLabelValuesMetadata: queryLabelValuesMetadata}
	row, _ := spanner.NewRow(
		[]string{
			stringLabelValueMetadata.ColumnName(),
			boolLabelValueMetadata.ColumnName(),
			int64LabelValueMetadata.ColumnName(),
			stringSliceLabelValueMetadata.ColumnName(),
			byteSliceLabelValueMetadata.ColumnName(),
			lockRequestSliceLabelValueMetadata.ColumnName(),
		},
		[]any{
			stringValue,
			boolValue,
			int64Value,
			[]string{stringValue, stringValue},
			[]byte(stringValue),
			[]*lockRequest{{}},
		})

	labelValues, _ := metadata.toLabelValues(row)

	assert.Equal(t, len(queryLabelValuesMetadata), len(labelValues))

	expectedTypes := []LabelValue{
		stringLabelValue{},
		boolLabelValue{},
		int64LabelValue{},
		stringSliceLabelValue{},
		byteSliceLabelValue{},
		lockRequestSliceLabelValue{},
	}

	for i, expectedType := range expectedTypes {
		assert.IsType(t, expectedType, labelValues[i])
	}
}

func TestMetricsMetadata_ToLabelValues_Error(t *testing.T) {
	stringLabelValueMetadata, _ := NewLabelValueMetadata("nonExisting", "nonExistingColumn", StringValueType)
	queryLabelValuesMetadata := []LabelValueMetadata{stringLabelValueMetadata}
	metadata := MetricsMetadata{QueryLabelValuesMetadata: queryLabelValuesMetadata}
	row, _ := spanner.NewRow([]string{}, []any{})

	labelValues, err := metadata.toLabelValues(row)

	assert.Nil(t, labelValues)
	require.Error(t, err)
}

func TestToMetricValue(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	rowColumnNames := []string{metricColumnName}
	testCases := map[string]struct {
		valueType     ValueType
		expectedType  MetricValue
		expectedValue any
	}{
		"Int64 metric value metadata":   {IntValueType, int64MetricValue{}, int64Value},
		"Float64 metric value metadata": {FloatValueType, float64MetricValue{}, float64Value},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			row, _ := spanner.NewRow(rowColumnNames, []any{testCase.expectedValue})
			metadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, testCase.valueType)

			metricValue, _ := toMetricValue(metadata, row)

			assert.IsType(t, testCase.expectedType, metricValue)
			assert.Equal(t, metricName, metricValue.Metadata().Name())
			assert.Equal(t, metricColumnName, metricValue.Metadata().ColumnName())
			assert.Equal(t, metricDataType, metricValue.Metadata().DataType())
			assert.Equal(t, metricUnit, metricValue.Metadata().Unit())
			assert.Equal(t, testCase.expectedValue, metricValue.Value())
		})
	}
}

func TestMetricsMetadata_ToMetricValues_AllPossibleMetadata(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	int64MetricValueMetadata, _ := NewMetricValueMetadata("int64MetricName",
		"int64MetricColumnName", metricDataType, metricUnit, IntValueType)
	float64MetricValueMetadata, _ := NewMetricValueMetadata("float64MetricName",
		"float64MetricColumnName", metricDataType, metricUnit, FloatValueType)
	queryMetricValuesMetadata := []MetricValueMetadata{
		int64MetricValueMetadata,
		float64MetricValueMetadata,
	}
	metadata := MetricsMetadata{QueryMetricValuesMetadata: queryMetricValuesMetadata}
	row, _ := spanner.NewRow(
		[]string{int64MetricValueMetadata.ColumnName(), float64MetricValueMetadata.ColumnName()},
		[]any{int64Value, float64Value})

	metricValues, _ := metadata.toMetricValues(row)

	assert.Equal(t, len(queryMetricValuesMetadata), len(metricValues))

	expectedTypes := []MetricValue{int64MetricValue{}, float64MetricValue{}}

	for i, expectedType := range expectedTypes {
		assert.IsType(t, expectedType, metricValues[i])
	}
}

func TestMetricsMetadata_ToMetricValues_Error(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	int64MetricValueMetadata, _ := NewMetricValueMetadata("nonExistingMetricName",
		"nonExistingMetricColumnName", metricDataType, metricUnit, IntValueType)
	queryMetricValuesMetadata := []MetricValueMetadata{int64MetricValueMetadata}
	metadata := MetricsMetadata{QueryMetricValuesMetadata: queryMetricValuesMetadata}
	row, _ := spanner.NewRow([]string{}, []any{})

	metricValues, err := metadata.toMetricValues(row)

	assert.Nil(t, metricValues)
	require.Error(t, err)
}

func TestMetricsMetadata_RowToMetricsDataPoints(t *testing.T) {
	metricDataType := metricValueDataType{dataType: metricDataType}
	timestamp := time.Now().UTC()
	labelValueMetadata, _ := NewLabelValueMetadata(labelName, labelColumnName, StringValueType)
	metricValueMetadata, _ := NewMetricValueMetadata(metricName, metricColumnName, metricDataType, metricUnit, IntValueType)
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
		rowColumnValues []any
		expectError     bool
	}{
		"Happy path":            {[]string{labelColumnName, metricColumnName, timestampColumnName}, []any{stringValue, int64Value, timestamp}, false},
		"Error on timestamp":    {[]string{labelColumnName, metricColumnName}, []any{stringValue, int64Value}, true},
		"Error on label value":  {[]string{metricColumnName, timestampColumnName}, []any{int64Value, timestamp}, true},
		"Error on metric value": {[]string{labelColumnName, timestampColumnName}, []any{stringValue, timestamp}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			row, _ := spanner.NewRow(testCase.rowColumnNames, testCase.rowColumnValues)
			dataPoints, err := metadata.RowToMetricsDataPoints(databaseID, row)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, len(dataPoints))
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

func TestMetricsMetadata_ToMetricsDataPoints(t *testing.T) {
	timestamp := time.Now().UTC()
	labelValues := allPossibleLabelValues()
	metricValues := allPossibleMetricValues(metricDataType)
	databaseID := databaseID()
	metadata := MetricsMetadata{MetricNamePrefix: metricNamePrefix}

	dataPoints := metadata.toMetricsDataPoints(databaseID, timestamp, labelValues, metricValues)

	assert.Equal(t, len(metricValues), len(dataPoints))

	for i, dataPoint := range dataPoints {
		assert.Equal(t, metadata.MetricNamePrefix+metricValues[i].Metadata().Name(), dataPoint.metricName)
		assert.Equal(t, timestamp, dataPoint.timestamp)
		assert.Equal(t, databaseID, dataPoint.databaseID)
		assert.Equal(t, labelValues, dataPoint.labelValues)
		assert.Equal(t, metricValues[i].Value(), dataPoint.metricValue.Value())
	}
}
