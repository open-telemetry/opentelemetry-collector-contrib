// Copyright The OpenTelemetry Authors
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

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/DataDog/datadog-agent/pkg/otlp/model/source"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var (
	testAttributes = map[string]string{"datadog.host.name": "custom-hostname"}
	// TestMetrics metrics for tests.
	TestMetrics = newMetricsWithAttributeMap(testAttributes)
	// TestTraces traces for tests.
	TestTraces = newTracesWithAttributeMap(testAttributes)
)

type DatadogServer struct {
	*httptest.Server
	MetadataChan chan []byte
}

// DatadogServerMock mocks a Datadog backend server
func DatadogServerMock(overwriteHandlerFuncs ...OverwriteHandleFunc) *DatadogServer {
	metadataChan := make(chan []byte)
	mux := http.NewServeMux()

	handlers := map[string]http.HandlerFunc{
		"/api/v1/validate": validateAPIKeyEndpoint,
		"/api/v1/series":   metricsEndpoint,
		"/intake":          newMetadataEndpoint(metadataChan),
		"/":                func(w http.ResponseWriter, r *http.Request) {},
	}
	for _, f := range overwriteHandlerFuncs {
		p, hf := f()
		handlers[p] = hf
	}
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}

	srv := httptest.NewServer(mux)

	return &DatadogServer{
		srv,
		metadataChan,
	}
}

// OverwriteHandleFuncs allows to overwrite the default handler functions
type OverwriteHandleFunc func() (string, http.HandlerFunc)

// HTTPRequestRecorder records a HTTP request.
type HTTPRequestRecorder struct {
	Pattern  string
	Header   http.Header
	ByteBody []byte
}

func (rec *HTTPRequestRecorder) HandlerFunc() (string, http.HandlerFunc) {
	return rec.Pattern, func(w http.ResponseWriter, r *http.Request) {
		rec.Header = r.Header
		rec.ByteBody, _ = io.ReadAll(r.Body)
	}
}

// ValidateAPIKeyEndpointInvalid returns a handler function that returns an invalid API key response
func ValidateAPIKeyEndpointInvalid() (string, http.HandlerFunc) {
	return "/api/v1/validate", validateAPIKeyEndpointInvalid
}

type validateAPIKeyResponse struct {
	Valid bool `json:"valid"`
}

func validateAPIKeyEndpoint(w http.ResponseWriter, r *http.Request) {
	res := validateAPIKeyResponse{Valid: true}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

func validateAPIKeyEndpointInvalid(w http.ResponseWriter, r *http.Request) {
	res := validateAPIKeyResponse{Valid: false}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

type metricsResponse struct {
	Status string `json:"status"`
}

func metricsEndpoint(w http.ResponseWriter, r *http.Request) {
	res := metricsResponse{Status: "ok"}
	resJSON, _ := json.Marshal(res)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, err := w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

func newMetadataEndpoint(c chan []byte) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c <- body
	}
}

func fillAttributeMap(attrs pcommon.Map, mp map[string]string) {
	attrs.Clear()
	attrs.EnsureCapacity(len(mp))
	for k, v := range mp {
		attrs.Insert(k, pcommon.NewValueString(v))
	}
}

// NewAttributeMap creates a new attribute map (string only)
// from a Go map
func NewAttributeMap(mp map[string]string) pcommon.Map {
	attrs := pcommon.NewMap()
	fillAttributeMap(attrs, mp)
	return attrs
}

func newMetricsWithAttributeMap(mp map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	fillAttributeMap(md.ResourceMetrics().AppendEmpty().Resource().Attributes(), mp)
	return md
}

func newTracesWithAttributeMap(mp map[string]string) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()
	fillAttributeMap(rs.Resource().Attributes(), mp)
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return traces
}

type MockSourceProvider struct {
	Src source.Source
}

func (s *MockSourceProvider) Source(ctx context.Context) (source.Source, error) {
	return s.Src, nil
}
