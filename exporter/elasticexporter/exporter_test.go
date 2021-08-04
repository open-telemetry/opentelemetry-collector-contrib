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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
)

func TestTracesExporter(t *testing.T) {
	cleanup, err := obsreporttest.SetupRecordedMetricsTest()
	require.NoError(t, err)
	defer cleanup()

	factory := NewFactory()
	recorder, cfg := newRecorder(t)
	params := componenttest.NewNopExporterCreateSettings()
	te, err := factory.CreateTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	traces := pdata.NewTraces()
	resourceSpans := traces.ResourceSpans()
	span := resourceSpans.AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("foobar")

	err = te.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)
	obsreporttest.CheckExporterTraces(t, cfg.ID(), 1, 0)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Transactions, 1)
	assert.Equal(t, "foobar", payloads.Transactions[0].Name)

	assert.NoError(t, te.Shutdown(context.Background()))
}

func TestMetricsExporter(t *testing.T) {
	cleanup, err := obsreporttest.SetupRecordedMetricsTest()
	require.NoError(t, err)
	defer cleanup()

	factory := NewFactory()
	recorder, cfg := newRecorder(t)
	params := componenttest.NewNopExporterCreateSettings()
	me, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")

	err = me.ConsumeMetrics(context.Background(), sampleMetrics())
	assert.NoError(t, err)

	payloads := recorder.Payloads()
	require.Len(t, payloads.Metrics, 2)
	assert.Contains(t, payloads.Metrics[0].Samples, "foobar")
	obsreporttest.CheckExporterMetrics(t, cfg.ID(), 2, 0)

	assert.NoError(t, me.Shutdown(context.Background()))
}

func TestMetricsExporterSendError(t *testing.T) {
	cleanup, err := obsreporttest.SetupRecordedMetricsTest()
	require.NoError(t, err)
	defer cleanup()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.APMServerURL = "http://testing.invalid"

	params := componenttest.NewNopExporterCreateSettings()
	me, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")

	err = me.ConsumeMetrics(context.Background(), sampleMetrics())
	assert.Error(t, err)
	obsreporttest.CheckExporterMetrics(t, cfg.ID(), 0, 2)

	assert.NoError(t, me.Shutdown(context.Background()))
}

func sampleMetrics() pdata.Metrics {
	metrics := pdata.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	resourceMetrics.EnsureCapacity(2)
	for i := 0; i < 2; i++ {
		metric := resourceMetrics.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("foobar")
		metric.SetDataType(pdata.MetricDataTypeGauge)
		metric.Gauge().DataPoints().AppendEmpty().SetDoubleVal(123)
	}
	return metrics
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

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.TLSClientSetting.CAFile = certfile.Name()
	eCfg.APMServerURL = srv.URL
	return &recorder, eCfg
}
