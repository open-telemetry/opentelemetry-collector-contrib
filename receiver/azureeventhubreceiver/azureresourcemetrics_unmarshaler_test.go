// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver

import (
	"testing"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/stretchr/testify/assert"
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
	event := azureEvent{EventHubEvent: &eventhub.Event{Data: []byte(encodedMetrics)}}
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
	event := azureEvent{EventHubEvent: &eventhub.Event{Data: []byte(encodedMetrics)}}
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
	event := azureEvent{EventHubEvent: &eventhub.Event{Data: []byte(encodedMetrics)}}
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
