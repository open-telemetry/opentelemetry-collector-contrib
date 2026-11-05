// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
)

// TestPrometheusExporter_ContentNegotiation checks escaping after translation,
// including when UTF-8 is requested only on formats the exporter does not serve.
func TestPrometheusExporter_ContentNegotiation(t *testing.T) {
	const prometheusAccept = "application/openmetrics-text;version=1.0.0;escaping=allow-utf-8;q=0.6,application/openmetrics-text;version=0.0.1;q=0.5,text/plain;version=1.0.0;escaping=allow-utf-8;q=0.4,text/plain;version=0.0.4;q=0.3,*/*;q=0.2"
	tests := []struct {
		name              string
		accept            string
		enableOpenMetrics bool
		wantContentType   string
		wantUTF8          bool
	}{
		{
			name:            "unsupported UTF-8 formats do not affect the selected text format",
			accept:          prometheusAccept,
			wantContentType: "text/plain; version=0.0.4; charset=utf-8; escaping=underscores",
		},
		{
			name:              "OpenMetrics honors its explicit UTF-8 request",
			accept:            prometheusAccept,
			enableOpenMetrics: true,
			wantContentType:   "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=allow-utf-8",
			wantUTF8:          true,
		},
		{
			name:            "text honors its explicit UTF-8 request",
			accept:          "text/plain;version=0.0.4;escaping=allow-utf-8",
			wantContentType: "text/plain; version=0.0.4; charset=utf-8; escaping=allow-utf-8",
			wantUTF8:        true,
		},
		{
			name:            "selected escaping overrides an unsupported UTF-8 offer",
			accept:          "application/openmetrics-text;version=1.0.0;escaping=allow-utf-8;q=0.9,text/plain;version=0.0.4;escaping=underscores;q=0.8",
			wantContentType: "text/plain; version=0.0.4; charset=utf-8; escaping=underscores",
		},
		{
			name:            "missing Accept header uses underscore escaping",
			wantContentType: "text/plain; version=0.0.4; charset=utf-8; escaping=underscores",
		},
	}
	for _, strategy := range []translationStrategy{noUTF8EscapingWithSuffixes, noTranslation} {
		t.Run(string(strategy), func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					cfg := createDefaultConfig().(*Config)
					cfg.ServerConfig.NetAddr.Endpoint = "localhost:8889"
					cfg.TranslationStrategy = strategy
					cfg.EnableOpenMetrics = tt.enableOpenMetrics
					exp, err := newPrometheusExporter(cfg, exportertest.NewNopSettings(metadata.Type))
					require.NoError(t, err)

					md := pmetric.NewMetrics()
					metric := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
					metric.SetName("foo.bar")
					metric.SetDescription("Example counter")
					metric.SetUnit("By")
					sum := metric.SetEmptySum()
					sum.SetIsMonotonic(true)
					sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
					dp := sum.DataPoints().AppendEmpty()
					dp.SetIntValue(7)
					dp.Attributes().PutStr("my.label", "value")
					require.NoError(t, exp.ConsumeMetrics(t.Context(), md))

					req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
					if tt.accept != "" {
						req.Header.Set("Accept", tt.accept)
					}
					res := httptest.NewRecorder()
					exp.handler.ServeHTTP(res, req)
					require.Equal(t, http.StatusOK, res.Code)
					assert.Equal(t, tt.wantContentType, res.Header().Get("Content-Type"))

					wantName, wantLabel := "foo_bar", "my_label"
					if tt.wantUTF8 {
						wantName, wantLabel = "foo.bar", `"my.label"`
					}
					if strategy == noUTF8EscapingWithSuffixes {
						wantName += "_bytes_total"
					}
					wantSamplePrefix := wantName + "{"
					if tt.wantUTF8 {
						wantSamplePrefix = `{"` + wantName + `",`
					}
					wantValue := "7"
					if tt.enableOpenMetrics {
						wantValue = "7.0"
					}
					wantSample := wantSamplePrefix + wantLabel + `="value",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} ` + wantValue + "\n"
					assert.Contains(t, res.Body.String(), wantSample)
				})
			}
		})
	}
}
