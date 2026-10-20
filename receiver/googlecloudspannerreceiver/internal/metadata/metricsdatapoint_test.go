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
	expectedHashValue = "4a9c4b806ba1b00a"
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
	iSliceLabelValueMetadata, _ := NewLabelValueMetadata("intSliceLabelName", "intSliceLabelColumnName", IntSliceValueType)
	iSliceLabelValue := intSliceLabelValue{
		metadata: iSliceLabelValueMetadata,
		value:    stringValue,
	}
	timestampLabelValueMetadata, _ := NewLabelValueMetadata("timestampLabelName", "timestampLabelColumnName", TimestampValueType)
	tsLabelValue := timestampLabelValue{
		metadata: timestampLabelValueMetadata,
		value:    stringValue,
	}

	return []LabelValue{
		strLabelValue,
		bLabelValue,
		i64LabelValue,
		strSliceLabelValue,
		btSliceLabelValue,
		lckReqSliceLabelValue,
		iSliceLabelValue,
		tsLabelValue,
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
	case stringLabelValue, stringSliceLabelValue, byteSliceLabelValue, lockRequestSliceLabelValue, intSliceLabelValue, timestampLabelValue:
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
			name:          "user split points table key",
			originalValue: "T(90,153,4,2024-04-30T17:00:00-07:00,1,2024-05-01,a)",
			expectedValue: "T(1777703975,1644993587,1260550007,2986432455,1803989619,2400125060,3398926610)",
		},
		{
			name:          "user split points index key with primary table key",
			originalValue: "Index: T_IDX on T, Index Key: (9091), Primary Table Key: (4567,123,4.2,2024-04-30T17:00:00-07:00,1,2024-05-01,a)",
			expectedValue: "Index: T_IDX on T, Index Key: (2656406141), Primary Table Key: (3677257499,2791679065,689089138,2986432455,1803989619,2400125060,3398926610)",
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
			splitLimitMetadata, _ := NewLabelValueMetadata("split_limit", "splitLimitColumnName", StringValueType)
			splitKeyMetadata, _ := NewLabelValueMetadata("split_key", "splitKeyColumnName", StringValueType)

			labelValues := []LabelValue{
				stringLabelValue{metadata: splitStartMetadata, value: tc.originalValue},
				stringLabelValue{metadata: splitLimitMetadata, value: tc.originalValue},
				stringLabelValue{metadata: splitKeyMetadata, value: tc.originalValue},
			}
			metricsDataPoint := &MetricsDataPoint{
				metricName:  "test_metric",
				timestamp:   time.Now().UTC(),
				databaseID:  databaseID(),
				labelValues: labelValues,
			}

			metricsDataPoint.HideSplitStatsKeysPII()

			assert.Equal(t, tc.expectedValue, metricsDataPoint.labelValues[0].Value(), "Hashing failed for case %s", tc.name)
			assert.Equal(t, tc.expectedValue, metricsDataPoint.labelValues[1].Value(), "Hashing failed for case %s", tc.name)
			assert.Equal(t, tc.expectedValue, metricsDataPoint.labelValues[2].Value(), "Hashing failed for case %s", tc.name)
		})
	}
}

func TestMetricsDataPoint_HideClientIPAddressPII(t *testing.T) {
	testCases := []struct {
		name          string
		originalValue string
		expectedValue string
	}{
		// Non-IP / Redacted / Placeholder strings (no '.' or ':')
		{
			name:          "empty string",
			originalValue: "",
			expectedValue: "",
		},
		{
			name:          "private audit log placeholder",
			originalValue: "private",
			expectedValue: "private",
		},
		{
			name:          "gce-internal-ip audit log placeholder",
			originalValue: "gce-internal-ip",
			expectedValue: "gce-internal-ip",
		},
		{
			name:          "error placeholder",
			originalValue: "<error>",
			expectedValue: "<error>",
		},
		{
			name:          "unknown placeholder",
			originalValue: "unknown",
			expectedValue: "unknown",
		},
		{
			name:          "NULL string",
			originalValue: "NULL",
			expectedValue: "",
		},
		{
			name:          "plain hostname without dots",
			originalValue: "localhost",
			expectedValue: "localhost",
		},
		{
			name:          "plain numeric string without delimiters",
			originalValue: "123456789",
			expectedValue: "",
		},

		// IPv4 standard & boundary cases
		{
			name:          "ipv4 private address",
			originalValue: "192.168.1.1",
			expectedValue: "14261591930808458459",
		},
		{
			name:          "ipv4 loopback address",
			originalValue: "127.0.0.1",
			expectedValue: "1353920702439368852",
		},
		{
			name:          "ipv4 all zeros boundary",
			originalValue: "0.0.0.0",
			expectedValue: "1865442790408783828",
		},
		{
			name:          "ipv4 broadcast max boundary",
			originalValue: "255.255.255.255",
			expectedValue: "17605805416796728994",
		},
		{
			name:          "ipv4 address with port",
			originalValue: "10.128.0.5:54321",
			expectedValue: "6670765634391460772",
		},

		// IPv6 standard, compressed, bracketed & boundary cases
		{
			name:          "ipv6 full address",
			originalValue: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expectedValue: "5784436558048342182",
		},
		{
			name:          "ipv6 unique local address",
			originalValue: "fda3:e722:ac3:10:138:587f:ac11:5469",
			expectedValue: "17752314877368136085",
		},
		{
			name:          "ipv6 compressed middle zeros",
			originalValue: "2001:db8::1",
			expectedValue: "6556425118201004429",
		},
		{
			name:          "ipv6 loopback leading double colon",
			originalValue: "::1",
			expectedValue: "17291825625407301630",
		},
		{
			name:          "ipv6 trailing double colon",
			originalValue: "fe80::",
			expectedValue: "11812272640781785914",
		},
		{
			name:          "ipv6 unspecified all zeros double colon",
			originalValue: "::",
			expectedValue: "8166266742203981559",
		},
		{
			name:          "ipv6 uppercase hex",
			originalValue: "2001:DB8:ABCD:EF01::F",
			expectedValue: "17583487279216968028",
		},
		{
			name:          "ipv4-mapped ipv6 address",
			originalValue: "::ffff:192.168.1.1",
			expectedValue: "6901447465218037910",
		},
		{
			name:          "ipv6 nat64 embedded ipv4 address",
			originalValue: "64:ff9b::192.0.2.33",
			expectedValue: "3070413415546920759",
		},
		{
			name:          "bracketed ipv6 with port",
			originalValue: "[2001:db8::1]:8080",
			expectedValue: "11397108773292763715",
		},
		{
			name:          "bracketed ipv6 loopback with port",
			originalValue: "[::1]:443",
			expectedValue: "15343493414107352352",
		},

		// Invalid IP / port / degenerate delimiter edge cases (should return empty string)
		{
			name:          "single dot only",
			originalValue: ".",
			expectedValue: "",
		},
		{
			name:          "multiple dots only",
			originalValue: "...",
			expectedValue: "",
		},
		{
			name:          "single colon only",
			originalValue: ":",
			expectedValue: "",
		},
		{
			name:          "leading and trailing dots",
			originalValue: ".192.168.1.1.",
			expectedValue: "",
		},
		{
			name:          "empty brackets with colon",
			originalValue: "[]:",
			expectedValue: "",
		},
		{
			name:          "invalid ipv4 octet out of range",
			originalValue: "999.999.999.999",
			expectedValue: "",
		},
		{
			name:          "invalid port out of range",
			originalValue: "192.168.1.1:99999",
			expectedValue: "",
		},
		{
			name:          "invalid non-numeric port",
			originalValue: "192.168.1.1:abc",
			expectedValue: "",
		},
		{
			name:          "domain name with port",
			originalValue: "example.com:8080",
			expectedValue: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientIPMetadata, _ := NewLabelValueMetadata("client_ip_address", "CLIENT_IP_ADDRESS", StringValueType)
			otherMetadata, _ := NewLabelValueMetadata("other_label", "OTHER_COLUMN", StringValueType)
			labelValue1 := stringLabelValue{metadata: clientIPMetadata, value: tc.originalValue}
			labelValue2 := stringLabelValue{metadata: otherMetadata, value: "unchanged_value"}

			labelValues := []LabelValue{labelValue1, labelValue2}
			metricsDataPoint := &MetricsDataPoint{
				metricName:  "test_metric",
				timestamp:   time.Now().UTC(),
				databaseID:  databaseID(),
				labelValues: labelValues,
			}

			metricsDataPoint.HideClientIPAddressPII()

			assert.Equal(t, tc.expectedValue, metricsDataPoint.labelValues[0].Value(), "Hashing failed for case %s", tc.name)
			assert.Equal(t, "unchanged_value", metricsDataPoint.labelValues[1].Value())
		})
	}

	t.Run("no client_ip_address label present", func(t *testing.T) {
		otherMetadata, _ := NewLabelValueMetadata("other_label", "OTHER_COLUMN", StringValueType)
		metricsDataPoint := &MetricsDataPoint{
			metricName:  "test_metric",
			timestamp:   time.Now().UTC(),
			databaseID:  databaseID(),
			labelValues: []LabelValue{stringLabelValue{metadata: otherMetadata, value: "192.168.1.1"}},
		}

		metricsDataPoint.HideClientIPAddressPII()

		assert.Equal(t, "192.168.1.1", metricsDataPoint.labelValues[0].Value())
	})

	t.Run("empty labelValues slice", func(t *testing.T) {
		metricsDataPoint := &MetricsDataPoint{
			metricName:  "test_metric",
			timestamp:   time.Now().UTC(),
			databaseID:  databaseID(),
			labelValues: []LabelValue{},
		}

		metricsDataPoint.HideClientIPAddressPII()

		assert.Empty(t, metricsDataPoint.labelValues)
	})
}
