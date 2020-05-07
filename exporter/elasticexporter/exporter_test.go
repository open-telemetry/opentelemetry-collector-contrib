// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticexporter

import (
	"context"
	"encoding/pem"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/transport/transporttest"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestTraceExporter(t *testing.T) {
	var factory Factory
	recorder, cfg := newRecorder(t)
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	te, err := factory.CreateTraceExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	traces := pdata.NewTraces()
	resourceSpans := traces.ResourceSpans()
	resourceSpans.Resize(1)
	resourceSpans.At(0).InitEmpty()
	resourceSpans.At(0).InstrumentationLibrarySpans().Resize(1)
	resourceSpans.At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(1)
	span := resourceSpans.At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
	span.SetName("foobar")

	err = te.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	assert.Equal(t, "foobar", payloads.Transactions[0].Name)
}

// newRecorder returns a go.elastic.co/apm/transport/transporrtest.RecorderTransport,
// and an exporter config that sends to an HTTP server that will record events in the
// Elastic APM format.
func newRecorder(t *testing.T) (*transporttest.RecorderTransport, *Config) {
	var recorder transporttest.RecorderTransport
	srv := httptest.NewTLSServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/intake/v2/events" {
				http.Error(w, "unknown path", http.StatusNotFound)
				return
			}
			if err := recorder.SendStream(r.Context(), r.Body); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}),
	)
	t.Cleanup(srv.Close)

	// Write the server's self-signed certificate to a file to test the exporter's TLS config.
	certfile, err := ioutil.TempFile("", "otel-elastic-cacert")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(certfile.Name()) })
	err = pem.Encode(certfile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: srv.TLS.Certificates[0].Certificate[0],
	})
	require.NoError(t, err)

	var factory Factory
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.TLSClientSetting.CAFile = certfile.Name()
	eCfg.APMServerURL = srv.URL
	return &recorder, eCfg
}
