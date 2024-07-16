// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instanaexporter

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestPushConvertedTraces(t *testing.T) {
	traceServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer traceServer.Close()

	cfg := Config{
		AgentKey:     "key11",
		ClientConfig: confighttp.ClientConfig{Endpoint: traceServer.URL},
		Endpoint:     traceServer.URL,
	}

	instanaExporter := newInstanaExporter(&cfg, exportertest.NewNopSettings())
	ctx := context.Background()
	err := instanaExporter.start(ctx, componenttest.NewNopHost())
	assert.NoError(t, err)

	err = instanaExporter.pushConvertedTraces(ctx, newTestTraces())
	assert.NoError(t, err)
}

func newTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	rspans.Resource().Attributes().PutStr("instana.agent", "agent1")
	span := rspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4})
	span.SetSpanID([8]byte{0, 0, 0, 0, 1, 2, 3, 4})
	return traces
}

func TestSelfSignedBackend(t *testing.T) {
	var err error
	caFile := "testdata/ca.crt"
	handler := http.NewServeMux()
	handler.HandleFunc("/bundle", func(w http.ResponseWriter, _ *http.Request) {
		_, err = io.WriteString(w, "Hello from CA self signed server")

		if err != nil {
			t.Fatal(err)
		}
	})

	server := httptest.NewTLSServer(handler)
	defer server.Close()

	s := base64.StdEncoding.EncodeToString(server.Certificate().Raw)
	wholeCert := "-----BEGIN CERTIFICATE-----\n" + s + "\n-----END CERTIFICATE-----"

	err = os.WriteFile(caFile, []byte(wholeCert), os.FileMode(0600))
	defer func() {
		assert.NoError(t, os.Remove(caFile))
	}()

	if err != nil {
		t.Fatal(err)
	}

	// Starts the exporter to test the HTTP client request

	cfg := Config{
		AgentKey: "key11",
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			TLSSetting: configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: caFile,
				},
			},
		},
		Endpoint: server.URL,
	}

	ctx := context.Background()

	instanaExporter := newInstanaExporter(&cfg, exportertest.NewNopSettings())
	err = instanaExporter.start(ctx, componenttest.NewNopHost())

	if err != nil {
		t.Fatal(err)
	}

	assert.NoError(t, instanaExporter.export(ctx, server.URL, make(map[string]string), []byte{}))
}

func TestSelfSignedBackendCAFileNotFound(t *testing.T) {
	cfg := Config{
		AgentKey: "key11",
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "",
			TLSSetting: configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "ca_file_not_found.pem",
				},
			},
		},
		Endpoint: "",
	}

	ctx := context.Background()

	instanaExporter := newInstanaExporter(&cfg, exportertest.NewNopSettings())

	assert.Error(t, instanaExporter.start(ctx, componenttest.NewNopHost()), "expect not to find the ca file")
}
