package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
)

type DatadogLogsAgentServer struct {
	*httptest.Server
	// LogsData is the array of json requests sent to datadog backend
	LogsData          JSONLogs
	connectivityCheck sync.Once
}

// DatadogLogsAgentServerMock mocks a Datadog Logs Intake backend server for the logs agent
func DatadogLogsAgentServerMock(overwriteHandlerFuncs ...OverwriteHandleFunc) *DatadogLogsAgentServer {
	mux := http.NewServeMux()

	server := &DatadogLogsAgentServer{}
	handlers := map[string]http.HandlerFunc{
		"/api/v2/logs": server.logsAgentEndpoint,
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

func (s *DatadogLogsAgentServer) logsAgentEndpoint(w http.ResponseWriter, r *http.Request) {
	connectivityCheck := false
	s.connectivityCheck.Do(func() {
		// The logs agent performs a connectivity check upon initialization.
		// This function mocks a successful response for the first request received.
		w.WriteHeader(http.StatusAccepted)
		connectivityCheck = true
		return
	})
	if !connectivityCheck {
		jsonLogs := processLogsAgentRequest(w, r)
		s.LogsData = append(s.LogsData, jsonLogs...)
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
		json.Unmarshal([]byte(messageJSON), &message)
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
