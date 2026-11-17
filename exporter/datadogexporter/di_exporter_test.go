// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !freebsd && !solaris

package datadogexporter

import (
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestDIDeliveryThroughExporterFactory(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if string(body) != `[{"service":"checkout","message":"capture"}]` {
			t.Errorf("unexpected native payload: %s", body)
		}
		if r.Header.Get("DD-EVP-ORIGIN") != "agent-debugger" {
			t.Error("missing native debugger origin")
		}
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o600))
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*datadogconfig.Config)
	cfg.API.Key = "0123456789abcdef0123456789abcdef"
	cfg.API.FailOnInvalidKey = false
	cfg.Hostname = "application-host"
	cfg.HostMetadata.Enabled = false
	cfg.DynamicInstrumentation.Enabled = true
	cfg.DynamicInstrumentation.Endpoint = server.URL + "/api/v2/debugger"
	cfg.ClientConfig.TLS.CAFile = ca
	cfg.InitialInterval = time.Millisecond
	cfg.MaxInterval = time.Millisecond
	cfg.MaxElapsedTime = time.Second
	cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
	require.NoError(t, cfg.Validate())
	exp, err := f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	input := plog.NewLogs()
	rl := input.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "checkout")
	record := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	record.SetEventName("dd.debugger.snapshot")
	record.Attributes().PutInt("datadog.di.schema_version", 1)
	record.Attributes().PutStr("datadog.di.payload", `{"service":"checkout","message":"capture"}`)
	require.NoError(t, exp.ConsumeLogs(t.Context(), input))
	require.Equal(t, int32(2), attempts.Load())
	require.NoError(t, exp.Shutdown(t.Context()))
}
