// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"hash/fnv"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
)

const (
	// Value was generated using the same library. Intent is to detect that something changed in library implementation
	// in case we received different value here. For more details inspect tests where this value is used.
	expectedHashValue = "29282762c26450b7"
)

func TestMetricsDataPoint_GroupingKey(t *testing.T) {
	dataPoint := metricsDataPointForTests()

	groupingKey := dataPoint.GroupingKey()

	assert.NotNil(t, groupingKey)
	assert.Equal(t, dataPoint.metricName, groupingKey.MetricName)
	assert.Equal(t, dataPoint.metricValue.Metadata().Unit(), groupingKey.MetricUnit)
	assert.Equal(t, dataPoint.metricValue.Metadata().DataType(), groupingKey.MetricType)
}

func TestMetricsDataPoint_ToItem(t *testing.T) {
	dataPoint := metricsDataPointForTests()

	item, err := dataPoint.ToItem()
	require.NoError(t, err)

	assert.Equal(t, expectedHashValue, item.SeriesKey)
	assert.Equal(t, dataPoint.timestamp, item.Timestamp)
}

func TestMetricsDataPoint_ToDataForHashing(t *testing.T) {
	dataPoint := metricsDataPointForTests()

	actual := dataPoint.toDataForHashing()

	assert.Equal(t, metricName, actual.MetricName)

	assertLabel(t, actual.Labels[0], projectIDLabelName, dataPoint.databaseID.ProjectID())
	assertLabel(t, actual.Labels[1], instanceIDLabelName, dataPoint.databaseID.InstanceID())
	assertLabel(t, actual.Labels[2], databaseLabelName, dataPoint.databaseID.DatabaseName())

	labelsIndex := 3
	for _, labelValue := range dataPoint.labelValues {
		assertLabel(t, actual.Labels[labelsIndex], labelValue.Metadata().Name(), labelValue.Value())
		labelsIndex++
	}
}

func TestMetricsDataPoint_Hash(t *testing.T) {
	dataPoint := metricsDataPointForTests()

	hashValue, err := dataPoint.hash()
	require.NoError(t, err)

	assert.Equal(t, expectedHashValue, hashValue)
}

func TestMetricsDataPoint_CopyTo(t *testing.T) {
	timestamp := time.Now().UTC()
	labelValues := allPossibleLabelValues()
	metricValues := allPossibleMetricValues(metricDataType)
	databaseID := databaseID()

	for _, metricValue := range metricValues {
		dataPoint := pmetric.NewNumberDataPoint()
		metricsDataPoint := &MetricsDataPoint{
			metricName:  metricName,
			timestamp:   timestamp,
			databaseID:  databaseID,
			labelValues: labelValues,
			metricValue: metricValue,
		}

		metricsDataPoint.CopyTo(dataPoint)

		assertMetricValue(t, metricValue, dataPoint)

		assert.Equal(t, pcommon.NewTimestampFromTime(timestamp), dataPoint.Timestamp())
		// Adding +3 here because we'll always have 3 labels added for each metric: project_id, instance_id, database
		assert.Equal(t, 3+len(labelValues), dataPoint.Attributes().Len())

		attributesMap := dataPoint.Attributes()

		assertDefaultLabels(t, attributesMap, databaseID)
		assertNonDefaultLabels(t, attributesMap, labelValues)
	}
}

func TestMetricsDataPoint_HideLockStatsRowrangestartkeyPII(t *testing.T) {
	btSliceLabelValueMetadata, _ := NewLabelValueMetadata("row_range_start_key", "byteSliceLabelColumnName", StringValueType)
	labelValue1 := byteSliceLabelValue{metadata: btSliceLabelValueMetadata, value: "table1.s(23,hello,23+)"}
	labelValue2 := byteSliceLabelValue{metadata: btSliceLabelValueMetadata, value: "table2(23,hello)"}
	metricValues := allPossibleMetricValues(metricDataType)
	labelValues := []LabelValue{labelValue1, labelValue2}
	timestamp := time.Now().UTC()
	metricsDataPoint := &MetricsDataPoint{
		metricName:  metricName,
		timestamp:   timestamp,
		databaseID:  databaseID(),
		labelValues: labelValues,
		metricValue: metricValues[0],
	}
	hashFunction := fnv.New32a()
	hashFunction.Reset()
	hashFunction.Write([]byte("23"))
	hashOf23 := strconv.FormatUint(uint64(hashFunction.Sum32()), 10)
	hashFunction.Reset()
	hashFunction.Write([]byte("hello"))
	hashOfHello := strconv.FormatUint(uint64(hashFunction.Sum32()), 10)

	metricsDataPoint.HideLockStatsRowrangestartkeyPII()

	assert.Len(t, metricsDataPoint.labelValues, 2)
	assert.Equal(t, metricsDataPoint.labelValues[0].Value(), "table1.s("+hashOf23+","+hashOfHello+","+hashOf23+"+)")
	assert.Equal(t, metricsDataPoint.labelValues[1].Value(), "table2("+hashOf23+","+hashOfHello+")")
}

func TestMetricsDataPoint_HideLockStatsRowrangestartkeyPIIWithInvalidLabelValue(t *testing.T) {
	// We are checking that function HideLockStatsRowrangestartkeyPII() does not panic for invalid label values.
	btSliceLabelValueMetadata, _ := NewLabelValueMetadata("row_range_start_key", "byteSliceLabelColumnName", StringValueType)
	labelValue1 := byteSliceLabelValue{metadata: btSliceLabelValueMetadata, value: ""}
	labelValue2 := byteSliceLabelValue{metadata: btSliceLabelValueMetadata, value: "table22(hello"}
	labelValue3 := byteSliceLabelValue{metadata: btSliceLabelValueMetadata, value: "table22,hello"}
	labelValue4 := byteSliceLabelValue{metadata: btSliceLabelValueMetadata, value: "("}
	metricValues := allPossibleMetricValues(metricDataType)
	labelValues := []LabelValue{labelValue1, labelValue2, labelValue3, labelValue4}
	timestamp := time.Now().UTC()
	metricsDataPoint := &MetricsDataPoint{
		metricName:  metricName,
		timestamp:   timestamp,
		databaseID:  databaseID(),
		labelValues: labelValues,
		metricValue: metricValues[0],
	}
	metricsDataPoint.HideLockStatsRowrangestartkeyPII()
	assert.Len(t, metricsDataPoint.labelValues, 4)
}

func TestMetricsDataPoint_TruncateQueryText(t *testing.T) {
	strLabelValueMetadata, _ := NewLabelValueMetadata("query_text", "stringLabelColumnName", StringValueType)
	labelValue1 := stringLabelValue{metadata: strLabelValueMetadata, value: "SELECT 1"}
	metricValues := allPossibleMetricValues(metricDataType)
	labelValues := []LabelValue{labelValue1}
	timestamp := time.Now().UTC()
	metricsDataPoint := &MetricsDataPoint{
		metricName:  metricName,
		timestamp:   timestamp,
		databaseID:  databaseID(),
		labelValues: labelValues,
		metricValue: metricValues[0],
	}

	metricsDataPoint.TruncateQueryText(6)

	assert.Len(t, metricsDataPoint.labelValues, 1)
	assert.Equal(t, "SELECT", metricsDataPoint.labelValues[0].Value())
}

func allPossibleLabelValues() []LabelValue {
	strLabelValueMetadata, _ := NewLabelValueMetadata("stringLabelName", "stringLabelColumnName", StringValueType)
	strLabelValue := stringLabelValue{
		metadata: strLabelValueMetadata,
		value:    stringValue,
	}
	bLabelValueMetadata, _ := NewLabelValueMetadata("boolLabelName", "boolLabelColumnName", BoolValueType)
	bLabelValue := boolLabelValue{
		metadata: bLabelValueMetadata,
		value:    boolValue,
	}
	i64LabelValueMetadata, _ := NewLabelValueMetadata("int64LabelName", "int64LabelColumnName", StringValueType)
	i64LabelValue := int64LabelValue{
		metadata: i64LabelValueMetadata,
		value:    int64Value,
	}
	strSliceLabelValueMetadata, _ := NewLabelValueMetadata("stringSliceLabelName", "stringSliceLabelColumnName", StringValueType)
	strSliceLabelValue := stringSliceLabelValue{
		metadata: strSliceLabelValueMetadata,
		value:    stringValue,
	}
	btSliceLabelValueMetadata, _ := NewLabelValueMetadata("byteSliceLabelName", "byteSliceLabelColumnName", StringValueType)
	btSliceLabelValue := byteSliceLabelValue{
		metadata: btSliceLabelValueMetadata,
		value:    stringValue,
	}
	lckReqSliceLabelValueMetadata, _ := NewLabelValueMetadata("lockRequestSliceLabelName", "lockRequestSliceLabelColumnName", LockRequestSliceValueType)
	lckReqSliceLabelValue := lockRequestSliceLabelValue{
		metadata: lckReqSliceLabelValueMetadata,
		value:    stringValue,
	}

	return []LabelValue{
		strLabelValue,
		bLabelValue,
		i64LabelValue,
		strSliceLabelValue,
		btSliceLabelValue,
		lckReqSliceLabelValue,
	}
}

func allPossibleMetricValues(metricDataType pmetric.MetricType) []MetricValue {
	dataType := NewMetricType(metricDataType, pmetric.AggregationTemporalityDelta, true)
	int64Metadata, _ := NewMetricValueMetadata("int64MetricName", "int64MetricColumnName", dataType,
		metricUnit, IntValueType)
	float64Metadata, _ := NewMetricValueMetadata("float64MetricName", "float64MetricColumnName", dataType,
		metricUnit, FloatValueType)
	return []MetricValue{
		int64MetricValue{
			metadata: int64Metadata,
			value:    int64Value,
		},
		float64MetricValue{
			metadata: float64Metadata,
			value:    float64Value,
		},
	}
}

func assertDefaultLabels(t *testing.T, attributesMap pcommon.Map, databaseID *datasource.DatabaseID) {
	assertStringLabelValue(t, attributesMap, projectIDLabelName, databaseID.ProjectID())
	assertStringLabelValue(t, attributesMap, instanceIDLabelName, databaseID.InstanceID())
	assertStringLabelValue(t, attributesMap, databaseLabelName, databaseID.DatabaseName())
}

func assertNonDefaultLabels(t *testing.T, attributesMap pcommon.Map, labelValues []LabelValue) {
	for _, labelValue := range labelValues {
		assertLabelValue(t, attributesMap, labelValue)
	}
}

func assertLabelValue(t *testing.T, attributesMap pcommon.Map, labelValue LabelValue) {
	value, exists := attributesMap.Get(labelValue.Metadata().Name())

	assert.True(t, exists)
	switch labelValue.(type) {
	case stringLabelValue, stringSliceLabelValue, byteSliceLabelValue, lockRequestSliceLabelValue:
		assert.Equal(t, labelValue.Value(), value.Str())
	case boolLabelValue:
		assert.Equal(t, labelValue.Value(), value.Bool())
	case int64LabelValue:
		assert.Equal(t, labelValue.Value(), value.Int())
	default:
		assert.Fail(t, "Unknown label value type received")
	}
}

func assertStringLabelValue(t *testing.T, attributesMap pcommon.Map, labelName string, expectedValue any) {
	value, exists := attributesMap.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, expectedValue, value.Str())
}

func assertMetricValue(t *testing.T, metricValue MetricValue, dataPoint pmetric.NumberDataPoint) {
	switch metricValue.(type) {
	case int64MetricValue:
		assert.Equal(t, metricValue.Value(), dataPoint.IntValue())
	case float64MetricValue:
		assert.Equal(t, metricValue.Value(), dataPoint.DoubleValue())
	}
}

func assertLabel(t *testing.T, lbl label, expectedName string, expectedValue any) {
	assert.Equal(t, expectedName, lbl.Name)
	assert.Equal(t, expectedValue, lbl.Value)
}

func metricsDataPointForTests() *MetricsDataPoint {
	timestamp := time.Now().UTC()
	labelValues := allPossibleLabelValues()
	databaseID := databaseID()

	return &MetricsDataPoint{
		metricName:  metricName,
		timestamp:   timestamp,
		databaseID:  databaseID,
		labelValues: labelValues,
		metricValue: allPossibleMetricValues(metricDataType)[0],
	}
}

func TestMetricsDataPoint_HideSplitStatsKeysPII(t *testing.T) {
	testCases := []struct {
		name          string
		originalValue string
		expectedValue string
	}{
		{
			name:          "flat string",
			originalValue: "test_db_user_123",
			expectedValue: "test_db_user_123",
		},
		{
			name:          "boundary begin",
			originalValue: "<begin>",
			expectedValue: "<begin>",
		},
		{
			name:          "boundary end",
			originalValue: "<end>",
			expectedValue: "<end>",
		},
		{
			name:          "boundary infinity",
			originalValue: "<infinity>",
			expectedValue: "<infinity>",
		},
		{
			name:          "simple table key",
			originalValue: "Users(3)",
			expectedValue: "Users(1309098117)",
		},
		{
			name:          "composite table key",
			originalValue: "Messages(3,\"a\",1)",
			expectedValue: "Messages(1309098117,2894955330,1803989619)",
		},
		{
			name:          "nested brackets key",
			originalValue: "UsersTable(Accounts(12345, \"bob\"), Settings(true, \"US\"))",
			expectedValue: "UsersTable(Accounts(1502889754,2388175350), Settings(3049169947,3244586957))",
		},
		{
			name:          "quoted comma key",
			originalValue: "Table(3,\"John, Doe\")",
			expectedValue: "Table(1309098117,145129574)",
		},
		{
			name:          "escaped quotes",
			originalValue: "Table(\"O\\\"Connor\")",
			expectedValue: "Table(1624458470)",
		},
		{
			name:          "complex index",
			originalValue: "UserEmailIndex(\"alice@gmail.com\", 987654)",
			expectedValue: "UserEmailIndex(2147054059,1145901250)",
		},
		{
			name:          "empty brackets",
			originalValue: "Table()",
			expectedValue: "Table()",
		},
		{
			name:          "single quoted uuid",
			originalValue: "INDEX Order ('5b8bac71-0cb2-95e9-e1b0-89a027525460')",
			expectedValue: "INDEX Order (1576611093)",
		},
		{
			name:          "multi index with single quotes and escaped comma",
			originalValue: "INDEX IndexABC (0, '2020-06-18T17:24:53Z', '2020-06-18T17:24:53Z') TableKey (123,'ab\\,c')",
			expectedValue: "INDEX IndexABC (1609362278,1478541813,1478541813) TableKey (2791679065,4005005524)",
		},
		{
			name:          "verbose index key with begin",
			originalValue: "Index: T_IDX on T, Index Key: (10), Primary Table Key: (<begin>,<begin>)",
			expectedValue: "Index: T_IDX on T, Index Key: (1246026773), Primary Table Key: (<begin>,<begin>)",
		},
		{
			name:          "index with NULL values",
			originalValue: "INDEX IndexXYZ ('8762203435012030000',NULL,NULL)",
			expectedValue: "INDEX IndexXYZ (3755976169,NULL,NULL)",
		},
		{
			name:          "emoji unicode",
			originalValue: "UsersTable('bob😊')",
			expectedValue: "UsersTable(1331076972)",
		},
		{
			name:          "date of birth with hyphens",
			originalValue: "UsersTable(\"1990-01-01\")",
			expectedValue: "UsersTable(25106544)",
		},
		{
			name:          "date of birth with slashes",
			originalValue: "UsersTable(\"05/12/1985\")",
			expectedValue: "UsersTable(1994209726)",
		},
		{
			name:          "split stats empty brackets",
			originalValue: "TableA()",
			expectedValue: "TableA()",
		},
		{
			name:          "split stats interleaved",
			originalValue: "TableA().TableB",
			expectedValue: "TableA().TableB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			splitStartMetadata, _ := NewLabelValueMetadata("split_start", "splitStartColumnName", StringValueType)
			labelValue1 := stringLabelValue{metadata: splitStartMetadata, value: tc.originalValue}

			labelValues := []LabelValue{labelValue1}
			metricsDataPoint := &MetricsDataPoint{
				metricName:  "test_metric",
				timestamp:   time.Now().UTC(),
				databaseID:  databaseID(),
				labelValues: labelValues,
			}

			metricsDataPoint.HideSplitStatsKeysPII()

			assert.Equal(t, tc.expectedValue, metricsDataPoint.labelValues[0].Value(), "Hashing failed for case %s", tc.name)
		})
	}
}
