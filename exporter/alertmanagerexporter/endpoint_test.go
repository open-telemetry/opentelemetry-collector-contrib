// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter/internal/metadata"
)

func TestExporterEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		endpointSuffix string
		apiVersion     string
		wantPath       string
		wantQuery      string
	}{
		{name: "root", wantPath: "/api/v2/alerts"},
		{name: "trailing slash", endpointSuffix: "/", wantPath: "/api/v2/alerts"},
		{name: "path prefix", endpointSuffix: "/alertmanager", wantPath: "/alertmanager/api/v2/alerts"},
		{name: "path prefix trailing slash", endpointSuffix: "/alertmanager/", wantPath: "/alertmanager/api/v2/alerts"},
		{name: "escaped path prefix", endpointSuffix: "/team%2Ftenant/", wantPath: "/team%2Ftenant/api/v2/alerts"},
		{name: "query", endpointSuffix: "/alertmanager/?tenant=east%2Fwest", wantPath: "/alertmanager/api/v2/alerts", wantQuery: "tenant=east%2Fwest"},
		{name: "fragment", endpointSuffix: "/alertmanager/#overview", wantPath: "/alertmanager/api/v2/alerts"},
		{name: "query and fragment", endpointSuffix: "/alertmanager/?tenant=east%2Fwest#overview", wantPath: "/alertmanager/api/v2/alerts", wantQuery: "tenant=east%2Fwest"},
		{name: "v1", endpointSuffix: "/", apiVersion: "v1", wantPath: "/api/v1/alerts"},
	}
	for _, signal := range []string{"logs", "traces"} {
		for _, tt := range tests {
			t.Run(signal+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				alertsReceived := make(chan []model.Alert, 1)
				mux := http.NewServeMux()
				mux.HandleFunc("POST "+tt.wantPath, func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, tt.wantPath, r.URL.EscapedPath())
					assert.Equal(t, tt.wantQuery, r.URL.RawQuery)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
					var alerts []model.Alert
					if err := json.NewDecoder(r.Body).Decode(&alerts); !assert.NoError(t, err) {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					alertsReceived <- alerts
				})
				server := httptest.NewServer(mux)
				t.Cleanup(server.Close)

				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				cfg.ClientConfig.Endpoint = server.URL + tt.endpointSuffix
				cfg.GeneratorURL = "http://localhost/collector"
				cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
				cfg.BackoffConfig.Enabled = false
				if tt.apiVersion != "" {
					cfg.APIVersion = tt.apiVersion
				}
				set := exportertest.NewNopSettings(metadata.Type)
				var exp component.Component
				var consume func() error
				if signal == "logs" {
					logsExporter, err := factory.CreateLogs(t.Context(), set, cfg)
					require.NoError(t, err)
					exp = logsExporter
					logs := plog.NewLogs()
					logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test alert")
					consume = func() error { return logsExporter.ConsumeLogs(t.Context(), logs) }
				} else {
					tracesExporter, err := factory.CreateTraces(t.Context(), set, cfg)
					require.NoError(t, err)
					exp = tracesExporter
					traces, span := createTracesAndSpan()
					span.Events().AppendEmpty().SetName("test alert")
					consume = func() error { return tracesExporter.ConsumeTraces(t.Context(), traces) }
				}
				require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
				t.Cleanup(func() { assert.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context()))) })
				require.NoError(t, consume())
				select {
				case alerts := <-alertsReceived:
					require.Len(t, alerts, 1)
					assert.Equal(t, model.LabelValue("info"), alerts[0].Labels["severity"])
				default:
					t.Fatal("no alert was received")
				}
			})
		}
	}
}

func TestCreateExporterInvalidEndpoint(t *testing.T) {
	t.Parallel()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "http://%zz"
	set := exportertest.NewNopSettings(metadata.Type)
	logs, err := factory.CreateLogs(t.Context(), set, cfg)
	require.Error(t, err)
	assert.Nil(t, logs)
	traces, err := factory.CreateTraces(t.Context(), set, cfg)
	require.Error(t, err)
	assert.Nil(t, traces)
}
