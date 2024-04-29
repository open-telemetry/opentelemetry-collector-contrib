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
	"sync"
)

// JSONLogs is the type for the array of processed JSON log data from each request
type JSONLogs []JSONLog

// JSONLog is the type for the processed JSON log data from a single log
type JSONLog map[string]any

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
	LogsData             JSONLogs
	connectivityCheck    sync.Once
	logsAgentDoneChannel chan bool
}

// DatadogLogServerMock mocks a Datadog Logs Intake backend server
func DatadogLogServerMock(logsAgentDoneChannel chan bool, overwriteHandlerFuncs ...OverwriteHandleFunc) *DatadogLogsServer {
	mux := http.NewServeMux()

	server := &DatadogLogsServer{
		logsAgentDoneChannel: logsAgentDoneChannel,
	}
	handlers := map[string]http.HandlerFunc{
		// logs backend doesn't have validate endpoint
		// but adding one here for ease of testing
		"/api/v1/validate": validateAPIKeyEndpoint,
		"/":                server.logsEndpoint,
	}
	if logsAgentDoneChannel != nil {
		// "/api/v2/logs" overrides "/", so we only set this endpoint for logs agent tests
		handlers["/api/v2/logs"] = server.logsAgentEndpoint
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

func (s *DatadogLogsServer) logsAgentEndpoint(w http.ResponseWriter, r *http.Request) {
	connectivityCheck := false
	s.connectivityCheck.Do(func() {
		// The logs agent performs a connectivity check upon initialization.
		// This function mocks a successful response for the first request received.
		w.WriteHeader(http.StatusAccepted)
		connectivityCheck = true
	})
	if !connectivityCheck {
		jsonLogs := processLogsAgentRequest(w, r)
		s.LogsData = append(s.LogsData, jsonLogs...)
		s.logsAgentDoneChannel <- true
	}
}

func processLogsAgentRequest(w http.ResponseWriter, r *http.Request) JSONLogs {
	// we can reuse same response object for logs as well
	req, err := gUnzipData(r.Body)
	handleError(w, err, http.StatusBadRequest)
	var jsonLogs JSONLogs
	err = json.Unmarshal(req, &jsonLogs)
	handleError(w, err, http.StatusBadRequest)

	// unmarshal nested message JSON
	for i := range jsonLogs {
		messageJSON := jsonLogs[i]["message"].(string)
		var message JSONLog
		err = json.Unmarshal([]byte(messageJSON), &message)
		handleError(w, err, http.StatusBadRequest)
		jsonLogs[i]["message"] = message
		// delete dynamic keys that can't be tested
		delete(jsonLogs[i], "hostname")  // hostname of host running tests
		delete(jsonLogs[i], "timestamp") // ingestion timestamp
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, err = w.Write([]byte(`{"status":"ok"}`))
	handleError(w, err, 0)
	return jsonLogs
}
