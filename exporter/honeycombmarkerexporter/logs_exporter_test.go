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
)

func TestExportMarkers(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		attributeMap map[string]string
		errorMessage string
		context      context.Context
	}{
		{
			name: "body",
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
			attributeMap: map[string]string{
				"message": "this is a test message",
				"body":    "test",
			},
		},
		{
			name: "one attr",
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
								`attributes["foo"] == "bar"`,
							},
						},
					},
				},
			},
			attributeMap: map[string]string{
				"foo":     "bar",
				"message": "a test messsage",
			},
		},
		{
			name: "marker not created",
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
								`attributes["test"] == "bar"`,
							},
						},
					},
				},
			},
			attributeMap: map[string]string{
				"test": "testVal",
				"not":  "a match",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				decodedBody := map[string]any{}
				err := json.NewDecoder(req.Body).Decode(&decodedBody)
				require.NoError(t, err)

				rw.WriteHeader(http.StatusAccepted)

				if tt.errorMessage != "" {
					assert.NotEmpty(t, tt.errorMessage)
					assert.ErrorContains(t, err, tt.errorMessage)
				} else {
					for attr := range tt.attributeMap {
						if attr == "type" || attr == "message" {
							assert.Equal(t, decodedBody[attr], tt.attributeMap[attr])
						}
					}
				}

			}))
			defer markerServer.Close()

			tt.config.APIURL = markerServer.URL

			f := NewFactory()
			exp, err := f.CreateLogsExporter(context.Background(), exportertest.NewNopCreateSettings(), &tt.config)
			require.NoError(t, err)

			err = exp.Start(context.Background(), componenttest.NewNopHost())

			logs := constructLogs(tt.attributeMap)
			err = exp.ConsumeLogs(context.Background(), logs)

		})
	}
}

func constructLogs(attributes map[string]string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	l := sl.LogRecords().AppendEmpty()
	for attr, attrVal := range attributes {
		if attr == "body" {
			l.Body().SetStr(attrVal)
		} else {
			l.Attributes().PutStr(attr, attrVal)
		}
	}
	return logs
}
