// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const jsonBytes = `{
  "resourceMetrics": [
    {
      "resource": {
        "attributes": [
          {
            "key": "cloud.provider",
            "value": {
              "stringValue": "huawei_cloud"
            }
          },
          {
            "key": "project.id",
            "value": {
              "stringValue": "project_1"
            }
          },
          {
            "key": "region.id",
            "value": {
              "stringValue": "eu-west-101"
            }
          },
          {
            "key": "service.namespace",
            "value": {
              "stringValue": "SYS.ECS"
            }
          }
        ]
      },
      "scopeMetrics": [
        {
          "metrics": [
            {
              "gauge": {
                "dataPoints": [
                  {
                    "asDouble": 0.5,
                    "timeUnixNano": "1056625610000000000"
                  },
                  {
                    "asDouble": 0.7,
                    "timeUnixNano": "1236625715000000000"
                  }
                ]
              },
              "metadata": [
                {
                  "key": "instance_id",
                  "value": {
                    "stringValue": "faea5b75-e390-4e2b-8733-9226a9026070"
                  }
                }
              ],
              "name": "cpu_util",
              "unit": "%"
            }
          ],
          "scope": {
            "name": "huawei_cloud_ces",
            "version": "v1"
          }
        }
      ]
    },
    {
      "resource": {
        "attributes": [
          {
            "key": "cloud.provider",
            "value": {
              "stringValue": "huawei_cloud"
            }
          },
          {
            "key": "project.id",
            "value": {
              "stringValue": "project_1"
            }
          },
          {
            "key": "region.id",
            "value": {
              "stringValue": "eu-west-101"
            }
          },
          {
            "key": "service.namespace",
            "value": {
              "stringValue": "SYS.VPC"
            }
          }
        ]
      },
      "scopeMetrics": [
        {
          "metrics": [
            {
              "gauge": {
                "dataPoints": [
                  {
                    "asDouble": 1,
                    "timeUnixNano": "1056625612000000000"
                  },
                  {
                    "asDouble": 3,
                    "timeUnixNano": "1256625717000000000"
                  }
                ]
              },
              "metadata": [
                {
                  "key": "instance_id",
                  "value": {
                    "stringValue": "06b4020f-461a-4a52-84da-53fa71c2f42b"
                  }
                }
              ],
              "name": "network_vm_connections",
              "unit": "count"
            }
          ],
          "scope": {
            "name": "huawei_cloud_ces",
            "version": "v1"
          }
        }
      ]
    }
  ]
}`

func TestConvertCESMetricsToOTLP(t *testing.T) {
	input := map[string][]*MetricData{
		"SYS.ECS": {
			{
				MetricName: "cpu_util",
				Namespace:  "SYS.ECS",
				Dimensions: []model.MetricsDimension{
					{
						Name:  "instance_id",
						Value: "faea5b75-e390-4e2b-8733-9226a9026070",
					},
				},
				Datapoints: []model.Datapoint{
					{
						Average:   float64Ptr(0.5),
						Timestamp: 1056625610000,
					},
					{
						Average:   float64Ptr(0.7),
						Timestamp: 1236625715000,
					},
				},
				Unit: "%",
			},
		},
		"SYS.VPC": {
			{
				MetricName: "network_vm_connections",
				Namespace:  "SYS.VPC",
				Dimensions: []model.MetricsDimension{
					{
						Name:  "instance_id",
						Value: "06b4020f-461a-4a52-84da-53fa71c2f42b",
					},
				},
				Datapoints: []model.Datapoint{
					{
						Average:   float64Ptr(1),
						Timestamp: 1056625612000,
					},
					{
						Average:   float64Ptr(3),
						Timestamp: 1256625717000,
					},
				},
				Unit: "count",
			},
		},
	}
	unm := pmetric.JSONUnmarshaler{}
	expectedMetrics, err := unm.UnmarshalMetrics([]byte(jsonBytes))
	require.NoError(t, err)
	got := ConvertCESMetricsToOTLP("project_1", "eu-west-101", "average", input)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, got, pmetrictest.IgnoreResourceMetricsOrder()))
}

func float64Ptr(f float64) *float64 {
	return &f
}
