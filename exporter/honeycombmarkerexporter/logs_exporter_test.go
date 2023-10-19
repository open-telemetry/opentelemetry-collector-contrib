package honeycombmarkerexporter

import (
	"context"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"net/http"
	"net/http/httptest"
)

func TestExportMarkers(t *testing.T) {
	markerServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer markerServer.Close()

	cfg := Config{
		APIKey: "test-apikey",
		APIURL: "https://api.testhost.io",
		Markers: []Marker{
			{
				Type:         "test-type",
				MessageField: "test message",
				URLField:     "https://api.testhost.io",
				Rules: Rules{
					LogConditions: []string{
						`body == "test"`,
					},
				},
			},
		},
	}
	noOpSettings := exportertest.NewNopCreateSettings()
	markerExporter, err := newHoneycombLogsExporter(noOpSettings.TelemetrySettings, &cfg)
	assert.NoError(t, err)

	ctx := context.Background()

	logs := plog.NewLogs()
	err = markerExporter.exportMarkers(ctx, logs)
	assert.NoError(t, err)
}
