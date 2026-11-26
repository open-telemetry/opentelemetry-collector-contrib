// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

// TestOTLPHTTPTLSBackend sends over https, which checks that the scheme of the endpoint is kept.
func TestOTLPHTTPTLSBackend(t *testing.T) {
	var hits int
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{u.Host}})
	h := httpDefaults()
	h.ClientConfig.Endpoint = "https://placeholder"
	h.ClientConfig.TLS.InsecureSkipVerify = true
	h.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
	h.RetryConfig.Enabled = false
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	require.NoError(t, cfg.Validate())

	ctx := t.Context()
	me, err := createMetricsExporter(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, me.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, me.Shutdown(t.Context())) })

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric")
	require.NoError(t, me.ConsumeMetrics(ctx, md))
	assert.Equal(t, 1, hits)
}
