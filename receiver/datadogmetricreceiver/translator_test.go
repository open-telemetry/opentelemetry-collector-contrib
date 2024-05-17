// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogmetricreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver"

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"

	metricsV2 "github.com/DataDog/agent-payload/v5/gogen"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	metricsV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

func TestGetOtelMetricsFromDatadogV1Metrics(t *testing.T) {
	tests := []struct {
		name            string
		origin          string
		key             string
		ddReq           metricsV1.MetricsPayload
		expectedOtlpReq pmetricotlp.ExportRequest
		err             error
	}{
		{
			name:   "valid test",
			origin: "example.com",
			key:    "12345",
			ddReq: metricsV1.MetricsPayload{
				Series: []metricsV1.Series{
					{
						Host: func() *string {
							s := "example.com"
							return &s
						}(),
						Type: func() *string {
							s := "rate"
							return &s
						}(),
						Metric: "requests",
						Points: func() [][]*float64 {
							var s1 float64 = 1619737200
							var t1 float64 = 10
							var s2 float64 = 1619737210
							var t2 float64 = 15

							return [][]*float64{{&s1, &t1}, {&s2, &t2}}
						}(),
						Interval: func() datadog.NullableInt64 {
							var i int64 = 10
							s := datadog.NewNullableInt64(&i)
							return *s
						}(),
						Tags: []string{"key1:value1", "key2:value2"},
					},
				},
			},
			expectedOtlpReq: func() pmetricotlp.ExportRequest {
				metrics := pmetric.NewMetrics()
				resourceMetrics := metrics.ResourceMetrics()
				rm := resourceMetrics.AppendEmpty()
				resourceAttributes := rm.Resource().Attributes()

				resourceAttributes.PutStr("mw.client_origin", "example.com")
				resourceAttributes.PutStr("mw.account_key", "12345")
				resourceAttributes.PutStr("mw_source", "datadog")
				resourceAttributes.PutStr("host.id", "example.com")
				resourceAttributes.PutStr("host.name", "example.com")

				scopeMetrics := rm.ScopeMetrics().AppendEmpty()
				instrumentationScope := scopeMetrics.Scope()
				instrumentationScope.SetName("mw")
				instrumentationScope.SetVersion("v0.0.1")

				scopeMetric := scopeMetrics.Metrics().AppendEmpty()
				scopeMetric.SetName("requests")
				metricAttributes := pcommon.NewMap()

				metricAttributes.PutStr("key1", "value1")
				metricAttributes.PutStr("key2", "value2")

				var dataPoints pmetric.NumberDataPointSlice

				sum := scopeMetric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(false)
				dataPoints = sum.DataPoints()

				unixNano := 1619737200 * math.Pow(10, 9)
				dp1 := dataPoints.AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(unixNano))

				dp1.SetDoubleValue(10 * 10)
				attributeMap := dp1.Attributes()
				metricAttributes.CopyTo(attributeMap)

				unixNano = 1619737210 * math.Pow(10, 9)
				dp2 := dataPoints.AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(unixNano))
				dp2.SetDoubleValue(15 * 10)
				attributeMap = dp2.Attributes()
				metricAttributes.CopyTo(attributeMap)

				return pmetricotlp.NewExportRequestFromMetrics(metrics)
			}(),
			err: nil,
		},
		{
			name:   "no metrics in payload",
			origin: "example.com",
			key:    "12345",
			ddReq: metricsV1.MetricsPayload{
				Series: []metricsV1.Series{},
			},
			expectedOtlpReq: func() pmetricotlp.ExportRequest {
				return pmetricotlp.ExportRequest{}
			}(),
			err: ErrNoMetricsInPayload,
		},
	}

	for _, test := range tests {
		gotOtlpReq, err := getOtlpExportReqFromDatadogV1Metrics(test.origin, test.key, test.ddReq)
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		// assert.Equal(t, test.expectedMetrics, metrics)
		gotJSON, err := gotOtlpReq.MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}

		expectedJSON, err := test.expectedOtlpReq.MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		assert.True(t, assert.Equal(t, gotJSON, expectedJSON))
	}
}

func TestGetOtelMetricsFromDatadogV2Metrics(t *testing.T) {
	tests := []struct {
		name            string
		origin          string
		key             string
		ddReq           metricsV2.MetricPayload
		expectedOtlpReq pmetricotlp.ExportRequest
		err             error
	}{
		{
			name:   "valid test",
			origin: "example.com",
			key:    "12345",
			ddReq: metricsV2.MetricPayload{
				Series: []*metricsV2.MetricPayload_MetricSeries{
					{
						Resources: func() []*metricsV2.MetricPayload_Resource {
							v := metricsV2.MetricPayload_Resource{
								Type: "host",
								Name: "example.com",
							}
							return []*metricsV2.MetricPayload_Resource{
								&v,
							}
						}(),
						Type:   metricsV2.MetricPayload_RATE,
						Metric: "requests",
						Points: func() []*metricsV2.MetricPayload_MetricPoint {
							v1 := metricsV2.MetricPayload_MetricPoint{
								Value:     10,
								Timestamp: 1619737200,
							}

							v2 := metricsV2.MetricPayload_MetricPoint{
								Value:     15,
								Timestamp: 1619737210,
							}

							return []*metricsV2.MetricPayload_MetricPoint{&v1, &v2}
						}(),
						Interval: int64(10),
						Tags:     []string{"key1:value1", "key2:value2"},
					},
				},
			},
			expectedOtlpReq: func() pmetricotlp.ExportRequest {
				metrics := pmetric.NewMetrics()
				resourceMetrics := metrics.ResourceMetrics()
				rm := resourceMetrics.AppendEmpty()
				resourceAttributes := rm.Resource().Attributes()

				resourceAttributes.PutStr("mw.client_origin", "example.com")
				resourceAttributes.PutStr("mw.account_key", "12345")
				resourceAttributes.PutStr("mw_source", "datadog")
				resourceAttributes.PutStr("host.id", "example.com")
				resourceAttributes.PutStr("host.name", "example.com")

				scopeMetrics := rm.ScopeMetrics().AppendEmpty()
				instrumentationScope := scopeMetrics.Scope()
				instrumentationScope.SetName("mw")
				instrumentationScope.SetVersion("v0.0.1")

				scopeMetric := scopeMetrics.Metrics().AppendEmpty()
				scopeMetric.SetName("requests")
				metricAttributes := pcommon.NewMap()

				metricAttributes.PutStr("key1", "value1")
				metricAttributes.PutStr("key2", "value2")

				var dataPoints pmetric.NumberDataPointSlice

				sum := scopeMetric.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(false)
				dataPoints = sum.DataPoints()

				unixNano := 1619737200 * math.Pow(10, 9)
				dp1 := dataPoints.AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(unixNano))

				dp1.SetDoubleValue(10 * 10)
				attributeMap := dp1.Attributes()
				metricAttributes.CopyTo(attributeMap)

				unixNano = 1619737210 * math.Pow(10, 9)
				dp2 := dataPoints.AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(unixNano))
				dp2.SetDoubleValue(15 * 10)
				attributeMap = dp2.Attributes()
				metricAttributes.CopyTo(attributeMap)

				return pmetricotlp.NewExportRequestFromMetrics(metrics)
			}(),
			err: nil,
		},
		{
			name:   "no metrics in payload",
			origin: "example.com",
			key:    "12345",
			ddReq: metricsV2.MetricPayload{
				Series: []*metricsV2.MetricPayload_MetricSeries{},
			},
			expectedOtlpReq: func() pmetricotlp.ExportRequest {
				return pmetricotlp.ExportRequest{}
			}(),
			err: ErrNoMetricsInPayload,
		},
	}

	for _, test := range tests {
		gotOtlpReq, err := getOtlpExportReqFromDatadogV2Metrics(test.origin, test.key, test.ddReq)

		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		// assert.Equal(t, test.expectedMetrics, metrics)
		gotJSON, err := gotOtlpReq.MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}

		if err != nil {
			continue
		}

		expectedJSON, err := test.expectedOtlpReq.MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		assert.True(t, assert.Equal(t, gotJSON, expectedJSON))
	}
}

func TestGetOtlpExportReqFromDatadogV1MetaData(t *testing.T) {
	tests := []struct {
		name               string
		origin             string
		key                string
		ddReq              MetaDataPayload
		generateTestMetric pmetricotlp.ExportRequest
		expectedOtlpReq    func(payload MetaDataPayload) pmetricotlp.ExportRequest
		err                error
	}{
		{
			name:   "valid test",
			origin: "example.com",
			key:    "12345",
			ddReq: MetaDataPayload{
				Timestamp: 1714911197966125926,
				Hostname:  "example.com",
				Metadata: &hostMetadata{
					KernelRelease: "6.5.0-28-generic",
				},
			},
			expectedOtlpReq: func(payload MetaDataPayload) pmetricotlp.ExportRequest {
				metrics := pmetric.NewMetrics()
				resourceMetrics := metrics.ResourceMetrics()
				rm := resourceMetrics.AppendEmpty()
				resourceAttributes := rm.Resource().Attributes()

				resourceAttributes.PutStr("mw.client_origin", "example.com")
				resourceAttributes.PutStr("mw.account_key", "12345")
				resourceAttributes.PutStr("mw_source", "datadog")
				resourceAttributes.PutStr("host.id", "example.com")
				resourceAttributes.PutStr("host.name", "example.com")

				scopeMetrics := rm.ScopeMetrics().AppendEmpty()
				instrumentationScope := scopeMetrics.Scope()
				instrumentationScope.SetName("mw")
				instrumentationScope.SetVersion("v0.0.1")

				scopeMetric := scopeMetrics.Metrics().AppendEmpty()
				scopeMetric.SetName("system.host.metadata")
				metricAttributes := pcommon.NewMap()

				metaData := payload.Metadata
				v2 := reflect.ValueOf(*metaData)
				for i := 0; i < v2.NumField(); i++ {
					field := v2.Field(i)
					fieldType := v2.Type().Field(i)
					val := fmt.Sprintf("%v", field.Interface())
					metricAttributes.PutStr(fieldType.Name, val)
				}
				//metricAttributes.PutStr("KernelRelease", "6.value15.0-28-generic")
				//metricAttributes.PutStr("key2", "value2")

				var dataPoints pmetric.NumberDataPointSlice
				gauge := scopeMetric.SetEmptyGauge()
				dataPoints = gauge.DataPoints()

				dp := dataPoints.AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(1714911197966125926))
				dp.SetDoubleValue(float64(10.54) * 1.0)
				attributeMap := dp.Attributes()
				metricAttributes.CopyTo(attributeMap)
				return pmetricotlp.NewExportRequestFromMetrics(metrics)
			},
			err: nil,
		},
		{
			name:   "no metrics in payload",
			origin: "example.com",
			key:    "12345",
			ddReq: MetaDataPayload{
				Timestamp: 1714911197966125926,
				Hostname:  "example.com",
			},
			expectedOtlpReq: func(payload MetaDataPayload) pmetricotlp.ExportRequest {
				return pmetricotlp.ExportRequest{}
			},
			err: ErrNoMetricsInPayload,
		},
	}

	// testConfig := dummyMetricConfig{
	// 	pointVal:  float64(10.54) * 1.0,
	// 	timeStamp: 1714543980000,
	// }
	for _, test := range tests {
		gotOtlpReq, err := getOtlpExportReqFromDatadogV1MetaData(test.origin, test.key, test.ddReq)

		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		// assert.Equal(t, test.expectedMetrics, metrics)
		gotJSON, err := gotOtlpReq.MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}

		if err != nil {
			continue
		}

		expectedJSON, err := test.expectedOtlpReq(test.ddReq).MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		assert.True(t, assert.Equal(t, gotJSON, expectedJSON))
	}
}

// getOtlpExportReqFromDatadogIntakeData
func TestGetOtlpExportReqFromDatadogIntakeData(t *testing.T) {
	tests := []struct {
		name               string
		origin             string
		key                string
		ddReq              GoHaiData
		generateTestMetric pmetricotlp.ExportRequest
		expectedOtlpReq    func(payload GoHaiData) pmetricotlp.ExportRequest
		err                error
	}{
		{
			name:   "valid test",
			origin: "example.com",
			key:    "12345",
			ddReq: GoHaiData{
				FileSystem: []FileInfo{
					{
						KbSize:    "545454",
						MountedOn: "nvme",
						Name:      "temp1",
					},
				},
			},
			expectedOtlpReq: func(payload GoHaiData) pmetricotlp.ExportRequest {
				metrics := pmetric.NewMetrics()
				resourceMetrics := metrics.ResourceMetrics()
				rm := resourceMetrics.AppendEmpty()
				resourceAttributes := rm.Resource().Attributes()

				resourceAttributes.PutStr("mw.client_origin", "example.com")
				resourceAttributes.PutStr("mw.account_key", "12345")
				resourceAttributes.PutStr("mw_source", "datadog")
				resourceAttributes.PutStr("host.id", "example.com")
				resourceAttributes.PutStr("host.name", "example.com")

				scopeMetrics := rm.ScopeMetrics().AppendEmpty()
				instrumentationScope := scopeMetrics.Scope()
				instrumentationScope.SetName("mw")
				instrumentationScope.SetVersion("v0.0.1")

				scopeMetric := scopeMetrics.Metrics().AppendEmpty()
				scopeMetric.SetName("system.intake.metadata")
				metricAttributes := pcommon.NewMap()

				fileData := payload.FileSystem[0]
				floatVal, err := strconv.ParseFloat(fileData.KbSize, 64)
				if err != nil {
					return pmetricotlp.ExportRequest{}
				}

				str := fileData.Name + " mounted on " + fileData.MountedOn + " " + convertSize(floatVal)
				metricAttributes.PutStr("FILESYSTEM", str)

				var dataPoints pmetric.NumberDataPointSlice
				gauge := scopeMetric.SetEmptyGauge()
				dataPoints = gauge.DataPoints()

				dp := dataPoints.AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(1000))
				dp.SetDoubleValue(1.0)
				attributeMap := dp.Attributes()
				metricAttributes.CopyTo(attributeMap)
				return pmetricotlp.NewExportRequestFromMetrics(metrics)
			},
			err: nil,
		},
		{
			name:   "no metrics in payload",
			origin: "example.com",
			key:    "12345",
			ddReq: GoHaiData{
				FileSystem: []FileInfo{},
			},
			expectedOtlpReq: func(payload GoHaiData) pmetricotlp.ExportRequest {
				return pmetricotlp.ExportRequest{}
			},
			err: ErrNoMetricsInPayload,
		},
	}

	// testConfig := dummyMetricConfig{
	// 	pointVal:  float64(10.54) * 1.0,
	// 	timeStamp: 1714543980000,
	// }
	for _, test := range tests {
		gotOtlpReq, err := getOtlpExportReqFromDatadogIntakeData(test.origin, test.key, test.ddReq, struct {
			hostname      string
			containerInfo map[string]string
			milliseconds  int64
		}{
			milliseconds: 1000,
			hostname: "example.com",
		})

		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		// assert.Equal(t, test.expectedMetrics, metrics)
		gotJSON, err := gotOtlpReq.MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}

		if err != nil {
			continue
		}

		expectedJSON, err := test.expectedOtlpReq(test.ddReq).MarshalJSON()
		if err != test.err {
			t.Fatalf("%s: got err %v, want err %v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		assert.True(t, assert.Equal(t, gotJSON, expectedJSON))
	}
}
