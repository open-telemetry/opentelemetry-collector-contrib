// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter

import (
	"context"
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
			exp, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), &config)
			require.NoError(t, err)

			err = exp.Start(context.Background(), componenttest.NewNopHost())
			assert.NoError(t, err)

			logs := constructLogs(tt.attributeMap)
			err = exp.ConsumeLogs(context.Background(), logs)
			assert.NoError(t, err)
		})
	}
}

func constructLogs(attributes map[string]string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
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
			exp, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), &config)
			require.NoError(t, err)

			err = exp.Start(context.Background(), componenttest.NewNopHost())
			assert.NoError(t, err)

			logs := constructLogs(map[string]string{})
			err = exp.ConsumeLogs(context.Background(), logs)
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
			exp, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), &config)
			require.NoError(t, err)

			err = exp.Start(context.Background(), componenttest.NewNopHost())
			assert.NoError(t, err)

			logs := constructLogs(map[string]string{})
			err = exp.ConsumeLogs(context.Background(), logs)
			assert.NoError(t, err)
		})
	}
}
