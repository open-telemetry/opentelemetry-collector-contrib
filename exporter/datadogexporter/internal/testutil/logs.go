// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
)

// JSONLogs is the type for the array of processed JSON log data from each request
type JSONLogs []map[string]any

// HasDDTag returns true if every log has the given ddtags
func (jsonLogs *JSONLogs) HasDDTag(ddtags string) bool {
	for _, logData := range *jsonLogs {
		if ddtags != logData["ddtags"] {
			return false
		}
	}
	return true
}

type DatadogLogsServer struct {
	*httptest.Server
	// LogsData is the array of json requests sent to datadog backend
	LogsData JSONLogs
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
	jsonLogs := processLogsRequest(w, r)
	s.LogsData = append(s.LogsData, jsonLogs...)
}

func processLogsRequest(w http.ResponseWriter, r *http.Request) JSONLogs {
	// we can reuse same response object for logs as well
	req, err := gUnzipData(r.Body)
	handleError(w, err, http.StatusBadRequest)
	var jsonLogs JSONLogs
	err = json.Unmarshal(req, &jsonLogs)
	handleError(w, err, http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, err = w.Write([]byte(`{"status":"ok"}`))
	handleError(w, err, 0)
	return jsonLogs
}

func gUnzipData(rg io.Reader) ([]byte, error) {
	r, err := gzip.NewReader(rg)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// handleError logs the given error and writes the given status code if one is provided
// A statusCode of 0 represents no status code to write
func handleError(w http.ResponseWriter, err error, statusCode int) {
	if err != nil {
		if statusCode != 0 {
			w.WriteHeader(statusCode)
		}
		log.Fatalln(err)
	}
}

// MockLogsEndpoint returns the processed JSON log data for each endpoint call
func MockLogsEndpoint(w http.ResponseWriter, r *http.Request) JSONLogs {
	return processLogsRequest(w, r)
}
