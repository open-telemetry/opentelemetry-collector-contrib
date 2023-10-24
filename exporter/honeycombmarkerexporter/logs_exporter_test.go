package honeycombmarkerexporter

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestExportMarkers(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		attributeMap map[string]string
		errorMessage string
	}{
		{
			name: "",
			config: Config{
				APIKey:      "test-apikey",
				DatasetSlug: "test-dataset",
				Markers: []Marker{
					{
						Type:       "test-type",
						MessageKey: "message",
						URLKey:     "https://api.testhost.io",
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
		//{
		//	name: "normal config",
		//	config: Config{
		//		APIKey:      "test-apikey",
		//		DatasetSlug: "test-dataset",
		//		Markers: []Marker{
		//			{
		//				Type:       "test-type",
		//				MessageKey: "message",
		//				URLKey:     "https://api.testhost.io",
		//				Rules: Rules{
		//					LogConditions: []string{
		//						`attributes["foo"] == "bar"`,
		//					},
		//				},
		//			},
		//		},
		//	},
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var decodedBody map[string]any
			markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				err := json.NewDecoder(req.Body).Decode(&decodedBody)
				require.NoError(t, err)

				rw.WriteHeader(http.StatusAccepted)
			}))
			defer markerServer.Close()

			tt.config.APIURL = markerServer.URL

			f := NewFactory()
			exp, err := f.CreateLogsExporter(context.Background(), exportertest.NewNopCreateSettings(), &tt.config)
			require.NoError(t, err)

			err = exp.Start(context.Background(), componenttest.NewNopHost())
			if tt.errorMessage != "" {
				assert.ErrorContains(t, err, tt.errorMessage)
			} else {
				require.NoError(t, err)
				logs := constructLogs(tt.attributeMap)
				err = exp.ConsumeLogs(context.Background(), logs)
				require.NoError(t, err)
				for attr := range tt.attributeMap {
					assert.Equal(t, decodedBody[attr], tt.attributeMap[attr])
				}
			}
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
