// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
)

type DatadogLogsServer struct {
	*httptest.Server
	// LogsData is the array of json requests sent to datadog backend
	LogsData []map[string]interface{}
}

// DatadogLogServerMock mocks a Datadog Logs Intake backend server
func DatadogLogServerMock(overwriteHandlerFuncs ...OverwriteHandleFunc) *DatadogLogsServer {
	mux := http.NewServeMux()

	server := &DatadogLogsServer{}
	handlers := map[string]http.HandlerFunc{
		// logs backend doesn't have validate endpoint
		// but adding one here for ease of testing
		"/api/v1/validate": validateAPIKeyEndpoint,
		"/":                server.logsEndpoint,
	}
	for _, f := range overwriteHandlerFuncs {
		p, hf := f()
		handlers[p] = hf
	}
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	server.Server = httptest.NewServer(mux)
	return server
}

func (s *DatadogLogsServer) logsEndpoint(w http.ResponseWriter, r *http.Request) {
	// we can reuse same response object for logs as well
	res := metricsResponse{Status: "ok"}
	resJSON, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Fatalln(err)
	}

	req, err := gUnzipData(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Fatalln(err)
	}

	var jsonLogs []map[string]interface{}
	err = json.Unmarshal(req, &jsonLogs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Fatalln(err)
	}
	s.LogsData = append(s.LogsData, jsonLogs...)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	_, err = w.Write(resJSON)
	if err != nil {
		log.Fatalln(err)
	}
}

func gUnzipData(rg io.Reader) ([]byte, error) {
	r, err := gzip.NewReader(rg)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}
