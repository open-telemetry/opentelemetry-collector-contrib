// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var encodedMetrics = `{"records":[
{
  "count":23,
  "total":12292.1382,
  "minimum":27.4786,
  "maximum":6695.419,
  "average":534.440791304348,
  "resourceId":"/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/SERVICE",
  "time":"2025-07-14T12:45:00.0000000Z",
  "metricName":"dependencies/duration",
  "timeGrain":"PT1M"
},
{
  "time":"2025-07-14T12:35:36.3259399Z",
  "resourceId":"/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/SERVICE",
  "ResourceGUID":"00000000-0000-0000-0000-000000000000",
  "Type":"AppMetrics",
  "AppRoleInstance":"00000000-0000-0000-0000-000000000000",
  "AppRoleName":"service",
  "AppVersion":"1.0.0.0",
  "ClientBrowser":"Other",
  "ClientCity":"City",
  "ClientCountryOrRegion":"Country",
  "ClientIP":"0.0.0.0",
  "ClientModel":"Other",
  "ClientOS":"Linux",
  "ClientStateOrProvince":"Province",
  "ClientType":"PC",
  "IKey":"00000000-0000-0000-0000-000000000000",
  "_BilledSize":444,
  "SDKVersion":"dotnetiso:1.1.0.0_dotnet8.0.16:otel1.12.0:ext1.4.0",
  "Properties": {
    "an_attribute": "a_value",
    "another_attribute": "another_value"
  },
  "Name":"metric.name",
  "Sum":8,
  "Min":8,
  "Max":8,
  "ItemCount":1
}
]}`

func TestAzureResourceMetricsUnmarshaler_UnmarshalMixedMetrics(t *testing.T) {
	event := azureEvent{AzEventData: &azeventhubs.ReceivedEventData{EventData: azeventhubs.EventData{Body: []byte(encodedMetrics)}}}
	logger := zap.NewNop()
	unmarshaler := newAzureResourceMetricsUnmarshaler(
		component.BuildInfo{
			Command:     "Test",
			Description: "Test",
			Version:     "Test",
		},
		logger,
		&Config{},
	)
	metrics, err := unmarshaler.UnmarshalMetrics(&event)

	assert.NoError(t, err)
	assert.Equal(t, 9, metrics.MetricCount())
}

func TestAzureResourceMetricsUnmarshaler_UnmarshalAppMetricsWithAttributes(t *testing.T) {
	event := azureEvent{AzEventData: &azeventhubs.ReceivedEventData{EventData: azeventhubs.EventData{Body: []byte(encodedMetrics)}}}
	logger := zap.NewNop()
	unmarshaler := newAzureResourceMetricsUnmarshaler(
		component.BuildInfo{
			Command:     "Test",
			Description: "Test",
			Version:     "Test",
		},
		logger,
		&Config{},
	)
	metrics, err := unmarshaler.UnmarshalMetrics(&event)

	assert.NoError(t, err)

	expectedAttributes := map[string]string{
		"service.instance.id":   "00000000-0000-0000-0000-000000000000",
		"service.name":          "service",
		"service.version":       "1.0.0.0",
		"telemetry.sdk.version": "dotnetiso:1.1.0.0_dotnet8.0.16:otel1.12.0:ext1.4.0",
		"cloud.provider":        "azure",
		"cloud.region":          "Country",
		"azure.resource.id":     "/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/SERVICE",
		"os.name":               "Linux",
		"an_attribute":          "a_value",
		"another_attribute":     "another_value",
	}
	metric := metrics.ResourceMetrics().At(1).Resource()

	assert.Equal(t, len(expectedAttributes), metric.Attributes().Len())

	for k, expected := range expectedAttributes {
		actual, ok := metric.Attributes().Get(k)

		if !ok {
			t.Errorf("Attribute %s not found", k)
			continue
		}

		assert.Equal(t, expected, actual.AsString())
	}
}

func TestAzureResourceMetricsUnmarshaler_UnmarshalAggregatedAppMetrics(t *testing.T) {
	event := azureEvent{AzEventData: &azeventhubs.ReceivedEventData{EventData: azeventhubs.EventData{Body: []byte(encodedMetrics)}}}
	logger := zap.NewNop()
	unmarshaler := newAzureResourceMetricsUnmarshaler(
		component.BuildInfo{
			Command:     "Test",
			Description: "Test",
			Version:     "Test",
		},
		logger,
		&Config{
			MetricAggregation: "average",
		},
	)
	metrics, err := unmarshaler.UnmarshalMetrics(&event)

	assert.NoError(t, err)
	assert.Equal(t, 2, metrics.MetricCount())

	resMetric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "dependencies/duration", resMetric.Name())
	assert.Equal(t, 534.4407913043478, resMetric.Gauge().DataPoints().At(0).DoubleValue())

	appMetric := metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "metric.name", appMetric.Name())
	assert.Equal(t, 8.0, appMetric.Gauge().DataPoints().At(0).DoubleValue())
}

func TestAzureResourceMetricsUnmarshaler_SkipsMissingAggregates(t *testing.T) {
	// Standard Load Balancer diagnostic settings export total and count but no
	// minimum/maximum aggregates; the unmarshaler must not fabricate 0-valued
	// series for them, while an aggregate explicitly present as 0 is kept.
	eventPayload := `{"records":[
	{
	  "count":23,
	  "total":12292.1382,
	  "resourceId":"/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.NETWORK/LOADBALANCERS/LB",
	  "time":"2025-07-14T12:45:00.0000000Z",
	  "metricName":"bytecount",
	  "timeGrain":"PT1M"
	},
	{
	  "count":2,
	  "total":10,
	  "minimum":0,
	  "maximum":8,
	  "average":5,
	  "resourceId":"/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.NETWORK/LOADBALANCERS/LB",
	  "time":"2025-07-14T12:45:00.0000000Z",
	  "metricName":"withzeros",
	  "timeGrain":"PT1M"
	}
	]}`
	event := azureEvent{AzEventData: &azeventhubs.ReceivedEventData{EventData: azeventhubs.EventData{Body: []byte(eventPayload)}}}
	unmarshaler := newAzureResourceMetricsUnmarshaler(
		component.BuildInfo{
			Command:     "Test",
			Description: "Test",
			Version:     "Test",
		},
		zap.NewNop(),
		&Config{},
	)
	metrics, err := unmarshaler.UnmarshalMetrics(&event)

	assert.NoError(t, err)
	require.Equal(t, 7, metrics.MetricCount())

	// Each Event Hub record becomes its own ResourceMetrics entry.
	names := map[string]bool{}
	values := map[string]float64{}
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		ms := metrics.ResourceMetrics().At(i).ScopeMetrics().At(0).Metrics()
		for j := 0; j < ms.Len(); j++ {
			names[ms.At(j).Name()] = true
			values[ms.At(j).Name()] = ms.At(j).Gauge().DataPoints().At(0).DoubleValue()
		}
	}

	assert.Equal(t, map[string]bool{
		"bytecount_total":   true,
		"bytecount_count":   true,
		"withzeros_total":   true,
		"withzeros_count":   true,
		"withzeros_minimum": true,
		"withzeros_maximum": true,
		"withzeros_average": true,
	}, names)
	assert.Equal(t, 0.0, values["withzeros_minimum"])
}

func TestAzureResourceMetricsUnmarshaler_AverageAggregationSkipsRecordWithoutTotalOrCount(t *testing.T) {
	eventPayload := `{"records":[
	{
	  "average":5,
	  "resourceId":"/SUBSCRIPTIONS/00000000-0000-0000-0000-000000000000/RESOURCEGROUPS/RG/PROVIDERS/MICROSOFT.NETWORK/LOADBALANCERS/LB",
	  "time":"2025-07-14T12:45:00.0000000Z",
	  "metricName":"bytecount",
	  "timeGrain":"PT1M"
	}
	]}`
	event := azureEvent{AzEventData: &azeventhubs.ReceivedEventData{EventData: azeventhubs.EventData{Body: []byte(eventPayload)}}}
	unmarshaler := newAzureResourceMetricsUnmarshaler(
		component.BuildInfo{
			Command:     "Test",
			Description: "Test",
			Version:     "Test",
		},
		zap.NewNop(),
		&Config{
			MetricAggregation: "average",
		},
	)
	metrics, err := unmarshaler.UnmarshalMetrics(&event)

	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.MetricCount())
}
