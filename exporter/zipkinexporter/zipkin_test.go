// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinexporter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	zipkinmodel "github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/proto/zipkin_proto3"
	zipkinreporter "github.com/openzipkin/zipkin-go/reporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
)

// This function tests that Zipkin spans that are received then processed roundtrip
// back to almost the same JSON with differences:
// a) Go's net.IP.String intentional shortens 0s with "::" but also converts to hex values
//
//	so
//	      "7::0.128.128.127"
//	becomes
//	      "7::80:807f"
//
// The rest of the fields should match up exactly
func TestZipkinExporter_roundtripJSON(t *testing.T) {
	buf := new(bytes.Buffer)
	var sizes []int64
	cst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, _ := io.Copy(buf, r.Body)
		sizes = append(sizes, s)
		r.Body.Close()
	}))
	defer cst.Close()

	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: cst.URL,
		},
		Format: "json",
	}
	zexp, err := NewFactory().CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	assert.NoError(t, err)
	require.NotNil(t, zexp)

	require.NoError(t, zexp.Start(context.Background(), componenttest.NewNopHost()))

	// The test requires the spans from zipkinSpansJSONJavaLibrary to be sent in a single batch, use
	// a mock to ensure that this happens as intended.
	mzr := newMockZipkinReporter(cst.URL)

	// Run the Zipkin receiver to "receive spans upload from a client application"
	addr := testutil.GetAvailableLocalAddress(t)
	recvCfg := &zipkinreceiver.Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: addr,
		},
	}
	zi, err := zipkinreceiver.NewFactory().CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), recvCfg, zexp)
	assert.NoError(t, err)
	require.NotNil(t, zi)

	require.NoError(t, zi.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, zi.Shutdown(context.Background())) })

	// Let the receiver receive "uploaded Zipkin spans from a Java client application"
	_, err = http.Post("http://"+addr, "application/json", strings.NewReader(zipkinSpansJSONJavaLibrary))
	require.NoError(t, err)

	// Use the mock zipkin reporter to ensure all expected spans in a single batch. Since Flush waits for
	// server response there is no need for further synchronization.
	require.NoError(t, mzr.Flush())

	// We expect back the exact JSON that was received
	wants := []string{`
		[{
		  "traceId": "4d1e00c0db9010db86154a4ba6e91385","parentId": "86154a4ba6e91385","id": "4d1e00c0db9010db",
		  "kind": "CLIENT","name": "get",
		  "timestamp": 1472470996199000,"duration": 207000,
		  "localEndpoint": {"serviceName": "frontend","ipv6": "7::80:807f"},
		  "remoteEndpoint": {"serviceName": "backend","ipv4": "192.168.99.101","port": 9000},
		  "annotations": [
		    {"timestamp": 1472470996238000,"value": "foo"},
		    {"timestamp": 1472470996403000,"value": "bar"}
		  ],
		  "tags": {"http.path": "/api","clnt/finagle.version": "6.45.0"}
		},
		{
		  "traceId": "4d1e00c0db9010db86154a4ba6e91385","parentId": "86154a4ba6e91386","id": "4d1e00c0db9010dc",
		  "kind": "SERVER","name": "put",
		  "timestamp": 1472470996199000,"duration": 207000,
		  "localEndpoint": {"serviceName": "frontend","ipv6": "7::80:807f"},
		  "remoteEndpoint": {"serviceName": "frontend", "ipv4": "192.168.99.101","port": 9000},
		  "annotations": [
		    {"timestamp": 1472470996238000,"value": "foo"},
		    {"timestamp": 1472470996403000,"value": "bar"}
		  ],
		  "tags": {"http.path": "/api","clnt/finagle.version": "6.45.0"}
		},
		{
		  "traceId": "4d1e00c0db9010db86154a4ba6e91385",
		  "parentId": "86154a4ba6e91386",
		  "id": "4d1e00c0db9010dd",
		  "kind": "SERVER",
		  "name": "put",
		  "timestamp": 1472470996199000,
		  "duration": 207000
		}]
		`}
	for i, s := range wants {
		want := unmarshalZipkinSpanArrayToMap(t, s)
		gotBytes := buf.Next(int(sizes[i]))
		got := unmarshalZipkinSpanArrayToMap(t, string(gotBytes))
		for id, expected := range want {
			actual, ok := got[id]
			assert.True(t, ok)
			assert.Equal(t, expected.ID, actual.ID)
			assert.Equal(t, expected.Name, actual.Name)
			assert.Equal(t, expected.TraceID, actual.TraceID)
			assert.Equal(t, expected.Timestamp, actual.Timestamp)
			assert.Equal(t, expected.Duration, actual.Duration)
			assert.Equal(t, expected.Kind, actual.Kind)
		}
	}
}

type mockZipkinReporter struct {
	url        string
	client     *http.Client
	batch      []*zipkinmodel.SpanModel
	serializer zipkinreporter.SpanSerializer
}

var _ zipkinreporter.Reporter = (*mockZipkinReporter)(nil)

func (r *mockZipkinReporter) Send(span zipkinmodel.SpanModel) {
	r.batch = append(r.batch, &span)
}
func (r *mockZipkinReporter) Close() error {
	return nil
}

func newMockZipkinReporter(url string) *mockZipkinReporter {
	return &mockZipkinReporter{
		url:        url,
		client:     &http.Client{},
		serializer: zipkinreporter.JSONSerializer{},
	}
}

func (r *mockZipkinReporter) Flush() error {
	sendBatch := r.batch
	r.batch = nil

	if len(sendBatch) == 0 {
		return nil
	}

	body, err := r.serializer.Serialize(sendBatch)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", r.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", r.serializer.ContentType())

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("http request failed with status code %d", resp.StatusCode)
	}

	return nil
}

const zipkinSpansJSONJavaLibrary = `
[{
  "traceId": "4d1e00c0db9010db86154a4ba6e91385",
  "parentId": "86154a4ba6e91385",
  "id": "4d1e00c0db9010db",
  "kind": "CLIENT",
  "name": "get",
  "timestamp": 1472470996199000,
  "duration": 207000,
  "localEndpoint": {
    "serviceName": "frontend",
    "ipv6": "7::0.128.128.127"
  },
  "remoteEndpoint": {
    "serviceName": "backend",
    "ipv4": "192.168.99.101",
    "port": 9000
  },
  "annotations": [
    {
      "timestamp": 1472470996238000,
      "value": "foo"
    },
    {
      "timestamp": 1472470996403000,
      "value": "bar"
    }
  ],
  "tags": {
    "http.path": "/api",
    "clnt/finagle.version": "6.45.0"
  }
},
{
  "traceId": "4d1e00c0db9010db86154a4ba6e91385",
  "parentId": "86154a4ba6e91386",
  "id": "4d1e00c0db9010dc",
  "kind": "SERVER",
  "name": "put",
  "timestamp": 1472470996199000,
  "duration": 207000,
  "localEndpoint": {
    "serviceName": "frontend",
    "ipv6": "7::0.128.128.127"
  },
  "remoteEndpoint": {
    "serviceName": "frontend",
    "ipv4": "192.168.99.101",
    "port": 9000
  },
  "annotations": [
    {
      "timestamp": 1472470996238000,
      "value": "foo"
    },
    {
      "timestamp": 1472470996403000,
      "value": "bar"
    }
  ],
  "tags": {
    "http.path": "/api",
    "clnt/finagle.version": "6.45.0"
  }
},
{
  "traceId": "4d1e00c0db9010db86154a4ba6e91385",
  "parentId": "86154a4ba6e91386",
  "id": "4d1e00c0db9010dd",
  "kind": "SERVER",
  "name": "put",
  "timestamp": 1472470996199000,
  "duration": 207000
}]
`

func TestZipkinExporter_invalidFormat(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "1.2.3.4",
		},
		Format: "foobar",
	}
	f := NewFactory()
	set := exportertest.NewNopCreateSettings()
	_, err := f.CreateTracesExporter(context.Background(), set, config)
	require.Error(t, err)
}

// The rest of the fields should match up exactly
func TestZipkinExporter_roundtripProto(t *testing.T) {
	buf := new(bytes.Buffer)
	var contentType string
	cst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(buf, r.Body)
		assert.NoError(t, err)
		contentType = r.Header.Get("Content-Type")
		r.Body.Close()
	}))
	defer cst.Close()

	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: cst.URL,
		},
		Format: "proto",
	}
	zexp, err := NewFactory().CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	require.NoError(t, zexp.Start(context.Background(), componenttest.NewNopHost()))

	// The test requires the spans from zipkinSpansJSONJavaLibrary to be sent in a single batch, use
	// a mock to ensure that this happens as intended.
	mzr := newMockZipkinReporter(cst.URL)

	mzr.serializer = zipkin_proto3.SpanSerializer{}

	// Run the Zipkin receiver to "receive spans upload from a client application"
	addr := testutil.GetAvailableLocalAddress(t)
	recvCfg := &zipkinreceiver.Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: addr,
		},
	}
	zi, err := zipkinreceiver.NewFactory().CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), recvCfg, zexp)
	require.NoError(t, err)

	err = zi.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, zi.Shutdown(context.Background())) })

	// Let the receiver receive "uploaded Zipkin spans from a Java client application"
	_, _ = http.Post("http://"+addr, "", strings.NewReader(zipkinSpansJSONJavaLibrary))

	// Use the mock zipkin reporter to ensure all expected spans in a single batch. Since Flush waits for
	// server response there is no need for further synchronization.
	err = mzr.Flush()
	require.NoError(t, err)

	require.Equal(t, zipkin_proto3.SpanSerializer{}.ContentType(), contentType)
	// Finally we need to inspect the output
	gotBytes, err := io.ReadAll(buf)
	require.NoError(t, err)

	_, err = zipkin_proto3.ParseSpans(gotBytes, false)
	require.NoError(t, err)
}
