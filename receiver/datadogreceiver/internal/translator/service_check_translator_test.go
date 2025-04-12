// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"encoding/json"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var testTimestamp = int64(1700000000)

func TestHandleStructureParsing(t *testing.T) {
	tests := []struct {
		name             string
		checkRunPayload  []byte
		expectedServices []ServiceCheck
	}{
		{
			name: "happy",
			checkRunPayload: []byte(`[
				{
					"check": "datadog.agent.check_status",
					"host_name": "hosta",
					"status": 0,
					"message": "",
					"tags": [
						"check:container"
					]
				},
				{
					"check": "app.working",
					"host_name": "hosta",
					"timestamp": 1700000000,
					"status": 0,
					"message": "",
					"tags": null
				},
				{
					"check": "env.test",
					"host_name": "hosta",
					"status": 0,
					"message": "",
					"tags": [
						"env:argle", "foo:bargle"
					]
				}
			]`),
			expectedServices: []ServiceCheck{
				{
					Check:    "datadog.agent.check_status",
					HostName: "hosta",
					Status:   0,
					Tags:     []string{"check:container"},
				},
				{
					Check:     "app.working",
					HostName:  "hosta",
					Status:    0,
					Timestamp: 1700000000,
				},
				{
					Check:    "env.test",
					HostName: "hosta",
					Status:   0,
					Tags:     []string{"env:argle", "foo:bargle"},
				},
			},
		},
		{
			name: "happy no tags",
			checkRunPayload: []byte(`[
				{
					"check": "app.working",
					"host_name": "hosta",
					"timestamp": 1700000000,
					"status": 0,
					"message": "",
					"tags": null
				}
			]`),
			expectedServices: []ServiceCheck{
				{
					Check:     "app.working",
					HostName:  "hosta",
					Status:    0,
					Timestamp: 1700000000,
				},
			},
		},
		{
			name: "happy no timestamp",
			checkRunPayload: []byte(`[
				{
					"check": "env.test",
					"host_name": "hosta",
					"status": 0,
					"message": "",
					"tags": [
						"env:argle", "foo:bargle"
					]
				}
			]`),
			expectedServices: []ServiceCheck{
				{
					Check:    "env.test",
					HostName: "hosta",
					Status:   0,
					Tags:     []string{"env:argle", "foo:bargle"},
				},
			},
		},
		{
			name:             "empty",
			checkRunPayload:  []byte(`[]`),
			expectedServices: []ServiceCheck{},
		},
		{
			name: "happy no hostname",
			checkRunPayload: []byte(`[
				{
					"check": "env.test",
					"status": 0,
					"message": "",
					"tags": [
						"env:argle", "foo:bargle"
					]
				}
			]`),
			expectedServices: []ServiceCheck{
				{
					Check:  "env.test",
					Status: 0,
					Tags:   []string{"env:argle", "foo:bargle"},
				},
			},
		},
		{
			name:             "empty",
			checkRunPayload:  []byte(`[]`),
			expectedServices: []ServiceCheck{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var services []ServiceCheck
			err := json.Unmarshal(tt.checkRunPayload, &services)
			require.NoError(t, err, "Failed to unmarshal service payload JSON")
			assert.Equal(t, tt.expectedServices, services, "Parsed series does not match expected series")
		})
	}
}

func TestTranslateCheckRun(t *testing.T) {
	tests := []struct {
		name     string
		services []ServiceCheck
		expect   func(t *testing.T, result pmetric.Metrics)
	}{
		{
			name: "OK status, with TS, no tags, no hostname",
			services: []ServiceCheck{
				{
					Check:     "app.working",
					Timestamp: 1700000000,
					Status:    datadogV1.SERVICECHECKSTATUS_OK,
					Tags:      []string{},
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				expectedAttrs := tagsToAttributes([]string{}, "", newStringPool())
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				require.Equal(t, 1, result.MetricCount())
				require.Equal(t, 1, result.DataPointCount())

				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireGauge(t, metric, "app.working", 1)

				dp := metric.Gauge().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 1700000000, 0)
			},
		},
		{
			name: "OK status, no TS",
			services: []ServiceCheck{
				{
					Check:    "app.working",
					HostName: "foo",
					Status:   datadogV1.SERVICECHECKSTATUS_OK,
					Tags:     []string{"env:tag1", "version:tag2"},
				},
			},
			expect: func(t *testing.T, result pmetric.Metrics) {
				expectedAttrs := tagsToAttributes([]string{"env:tag1", "version:tag2"}, "foo", newStringPool())
				require.Equal(t, 1, result.ResourceMetrics().Len())
				requireResourceAttributes(t, result.ResourceMetrics().At(0).Resource().Attributes(), expectedAttrs.resource)
				require.Equal(t, 1, result.MetricCount())
				require.Equal(t, 1, result.DataPointCount())

				requireScope(t, result, expectedAttrs.scope, component.NewDefaultBuildInfo().Version)

				metric := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				requireGauge(t, metric, "app.working", 1)

				dp := metric.Gauge().DataPoints().At(0)
				requireDp(t, dp, expectedAttrs.dp, 0, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := createMetricsTranslator()
			mt.buildInfo = component.BuildInfo{
				Command:     "otelcol",
				Description: "OpenTelemetry Collector",
				Version:     "latest",
			}
			result := mt.TranslateServices(tt.services)

			tt.expect(t, result)
		})
	}
}

func TestTranslateCheckRunStatuses(t *testing.T) {
	tests := []struct {
		name           string
		services       []ServiceCheck
		expectedStatus int64
	}{
		{
			name: "OK status, no TS",
			services: []ServiceCheck{
				{
					Check:    "app.working",
					HostName: "foo",
					Status:   datadogV1.SERVICECHECKSTATUS_OK,
					Tags:     []string{"env:tag1", "version:tag2"},
				},
			},
			expectedStatus: 0,
		},
		{
			name: "Warning status",
			services: []ServiceCheck{
				{
					Check:     "app.warning",
					HostName:  "foo",
					Status:    datadogV1.SERVICECHECKSTATUS_WARNING,
					Tags:      []string{"env:tag1", "version:tag2"},
					Timestamp: testTimestamp,
				},
			},
			expectedStatus: 1,
		},
		{
			name: "Critical status",
			services: []ServiceCheck{
				{
					Check:     "app.critical",
					HostName:  "foo",
					Status:    datadogV1.SERVICECHECKSTATUS_CRITICAL,
					Tags:      []string{"env:tag1", "version:tag2"},
					Timestamp: testTimestamp,
				},
			},
			expectedStatus: 2,
		},
		{
			name: "Unknown status",
			services: []ServiceCheck{
				{
					Check:     "app.unknown",
					HostName:  "foo",
					Status:    datadogV1.SERVICECHECKSTATUS_UNKNOWN,
					Tags:      []string{"env:tag1", "version:tag2"},
					Timestamp: testTimestamp,
				},
			},
			expectedStatus: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := createMetricsTranslator()
			mt.buildInfo = component.BuildInfo{
				Command:     "otelcol",
				Description: "OpenTelemetry Collector",
				Version:     "latest",
			}
			result := mt.TranslateServices(tt.services)

			require.Equal(t, 1, result.MetricCount())
			require.Equal(t, 1, result.DataPointCount())

			requireScopeMetrics(t, result, 1, 1)

			requireScope(t, result, pcommon.NewMap(), component.NewDefaultBuildInfo().Version)

			metrics := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				assert.Equal(t, tt.expectedStatus, metric.Gauge().DataPoints().At(0).IntValue())
			}
		})
	}
}
