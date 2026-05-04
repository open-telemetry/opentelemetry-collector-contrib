// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter/internal/metadata"
)

func TestExportMarkers(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		attributeMap map[string]string
		expectedURL  string
	}{
		{
			name: "all fields",
			config: Config{
				APIKey: "test-apikey",
				Markers: []Marker{
					{
						Type:        "test-type",
						MessageKey:  "message",
						URLKey:      "url",
						DatasetSlug: "test-dataset",
						Rules: Rules{
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
			attributeMap: map[string]string{
				"message": "this is a test message",
				"url":     "https://api.testhost.io",
				"type":    "test-type",
			},
			expectedURL: "/1/markers/test-dataset",
		},
		{
			name: "no message key",
			config: Config{
				APIKey: "test-apikey",
				Markers: []Marker{
					{
						Type:        "test-type",
						URLKey:      "url",
						DatasetSlug: "test-dataset",
						Rules: Rules{
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
			attributeMap: map[string]string{
				"url":  "https://api.testhost.io",
				"type": "test-type",
			},
			expectedURL: "/1/markers/test-dataset",
		},
		{
			name: "no url",
			config: Config{
				APIKey: "test-apikey",
				Markers: []Marker{
					{
						Type:        "test-type",
						MessageKey:  "message",
						DatasetSlug: "test-dataset",
						Rules: Rules{
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
			attributeMap: map[string]string{
				"message": "this is a test message",
				"type":    "test-type",
			},
			expectedURL: "/1/markers/test-dataset",
		},
		{
			name: "no dataset_slug",
			config: Config{
				APIKey: "test-apikey",
				Markers: []Marker{
					{
						Type: "test-type",
						Rules: Rules{
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
			attributeMap: map[string]string{
				"type": "test-type",
			},
			expectedURL: "/1/markers/__all__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				decodedBody := map[string]any{}
				err := json.NewDecoder(req.Body).Decode(&decodedBody)

				assert.NoError(t, err)

				assert.Len(t, decodedBody, len(tt.attributeMap))

				for attr := range tt.attributeMap {
					assert.Equal(t, tt.attributeMap[attr], decodedBody[attr])
				}
				assert.Contains(t, req.URL.Path, tt.expectedURL)

				apiKey := req.Header.Get(honeycombTeam)
				assert.Equal(t, string(tt.config.APIKey), apiKey)

				userAgent := req.Header.Get(userAgentHeaderKey)
				assert.NotEmpty(t, userAgent)
				assert.Contains(t, userAgent, "OpenTelemetry Collector")

				rw.WriteHeader(http.StatusAccepted)
			}))
			defer markerServer.Close()

			config := tt.config
			config.APIURL = markerServer.URL

			f := NewFactory()
			exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), &config)
			require.NoError(t, err)

			err = exp.Start(t.Context(), componenttest.NewNopHost())
			assert.NoError(t, err)

			logs := constructLogs(tt.attributeMap)
			err = exp.ConsumeLogs(t.Context(), logs)
			assert.NoError(t, err)
		})
	}
}

func constructLogs(attributes map[string]string) plog.Logs {
	return constructLogsWithResource(attributes, nil)
}

func constructLogsWithResource(attributes, resourceAttributes map[string]string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	for k, v := range resourceAttributes {
		rl.Resource().Attributes().PutStr(k, v)
	}
	sl := rl.ScopeLogs().AppendEmpty()
	l := sl.LogRecords().AppendEmpty()

	l.Body().SetStr("test")
	for attr, attrVal := range attributes {
		l.Attributes().PutStr(attr, attrVal)
	}
	return logs
}

func TestExportMarkers_Error(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		responseCode int
		errorMessage string
	}{
		{
			name: "unauthorized greater than 400",
			config: Config{
				APIKey: "test-apikey",
				Markers: []Marker{
					{
						Type:        "test-type",
						MessageKey:  "message",
						URLKey:      "https://api.testhost.io",
						DatasetSlug: "test-dataset",
						Rules: Rules{
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
			responseCode: http.StatusUnauthorized,
			errorMessage: "marker creation failed with 401",
		},
		{
			name: "continue less than 200",
			config: Config{
				APIKey: "test-apikey",
				Markers: []Marker{
					{
						Type:        "test-type",
						MessageKey:  "message",
						URLKey:      "https://api.testhost.io",
						DatasetSlug: "test-dataset",
						Rules: Rules{
							LogConditions: []string{
								`body == "test"`,
							},
						},
					},
				},
			},
			responseCode: http.StatusSwitchingProtocols,
			errorMessage: "marker creation failed with 101",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
				rw.WriteHeader(tt.responseCode)
			}))
			defer markerServer.Close()

			config := tt.config
			config.APIURL = markerServer.URL

			f := NewFactory()
			exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), &config)
			require.NoError(t, err)

			err = exp.Start(t.Context(), componenttest.NewNopHost())
			assert.NoError(t, err)

			logs := constructLogs(map[string]string{})
			err = exp.ConsumeLogs(t.Context(), logs)
			assert.ErrorContains(t, err, tt.errorMessage)
		})
	}
}

func TestExportMarkers_NoAPICall(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "all fields",
			config: Config{
				APIKey: "test-apikey",
				Markers: []Marker{
					{
						Type:        "test-type",
						MessageKey:  "message",
						URLKey:      "url",
						DatasetSlug: "test-dataset",
						Rules: Rules{
							LogConditions: []string{
								`body == "foo"`,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
				assert.Fail(t, "should not call the markers api")
				rw.WriteHeader(http.StatusBadRequest) // 400
			}))
			defer markerServer.Close()

			config := tt.config
			config.APIURL = markerServer.URL

			f := NewFactory()
			exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), &config)
			require.NoError(t, err)

			err = exp.Start(t.Context(), componenttest.NewNopHost())
			assert.NoError(t, err)

			logs := constructLogs(map[string]string{})
			err = exp.ConsumeLogs(t.Context(), logs)
			assert.NoError(t, err)
		})
	}
}

func TestResolveDatasetSlug(t *testing.T) {
	tests := []struct {
		name        string
		marker      Marker
		serviceName string
		want        string
	}{
		{name: "default falls back to __all__", marker: Marker{}, want: "__all__"},
		{name: "configured slug used when flag off", marker: Marker{DatasetSlug: "billing"}, want: "billing"},
		{name: "service name used when flag on", marker: Marker{UseServiceNameAsDatasetSlug: true}, serviceName: "billing", want: "billing"},
		{name: "service name preferred over configured slug", marker: Marker{UseServiceNameAsDatasetSlug: true, DatasetSlug: "fallback"}, serviceName: "billing", want: "billing"},
		{name: "missing service name falls back to configured slug", marker: Marker{UseServiceNameAsDatasetSlug: true, DatasetSlug: "fallback"}, want: "fallback"},
		{name: "missing service name and missing slug falls back to default", marker: Marker{UseServiceNameAsDatasetSlug: true}, want: "__all__"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDatasetSlug(tt.marker, tt.serviceName)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExportMarkers_UseServiceNameAsDatasetSlug verifies the end-to-end path:
// when use_service_name_as_dataset_slug is true and the resource has a service.name,
// the marker request is sent to /1/markers/<service.name>.
func TestExportMarkers_UseServiceNameAsDatasetSlug(t *testing.T) {
	cfg := Config{
		APIKey: "test-apikey",
		Markers: []Marker{
			{
				Type:                        "test-type",
				DatasetSlug:                 "should-be-overridden",
				UseServiceNameAsDatasetSlug: true,
				Rules: Rules{
					LogConditions: []string{`body == "test"`},
				},
			},
		},
	}

	var gotPath string
	markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer markerServer.Close()

	cfg.APIURL = markerServer.URL

	f := NewFactory()
	exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), &cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

	logs := constructLogsWithResource(nil, map[string]string{"service.name": "billing"})
	require.NoError(t, exp.ConsumeLogs(t.Context(), logs))

	assert.Equal(t, "/1/markers/billing", gotPath)
}

// TestExportMarkers_UseServiceNameMissingFallsBackToSlug verifies the fallback when
// the flag is on but the resource is missing service.name: the marker still goes to
// the configured DatasetSlug rather than silently dropping or sending to /__all__.
func TestExportMarkers_UseServiceNameMissingFallsBackToSlug(t *testing.T) {
	cfg := Config{
		APIKey: "test-apikey",
		Markers: []Marker{
			{
				Type:                        "test-type",
				DatasetSlug:                 "fallback-dataset",
				UseServiceNameAsDatasetSlug: true,
				Rules: Rules{
					LogConditions: []string{`body == "test"`},
				},
			},
		},
	}

	var gotPath string
	markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer markerServer.Close()

	cfg.APIURL = markerServer.URL

	f := NewFactory()
	exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), &cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))

	// No resource attributes — service.name is missing.
	logs := constructLogs(nil)
	require.NoError(t, exp.ConsumeLogs(t.Context(), logs))

	assert.Equal(t, "/1/markers/fallback-dataset", gotPath)
}
