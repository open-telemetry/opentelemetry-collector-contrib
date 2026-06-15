// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/integrationtest"

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// runtimeAPI is a minimal in-process Lambda Runtime API server.
// It is modeled on the test helper in github.com/aws/aws-lambda-go/lambda/invoke_loop_test.go.
type runtimeAPI struct {
	// Addr is the host:port value to assign to AWS_LAMBDA_RUNTIME_API.
	Addr   string
	server *httptest.Server
	// Events is the channel through which the test pushes invocation payloads.
	Events chan []byte
	// Done receives one result per invocation.
	Done chan invocationResult
}

type invocationResult struct {
	Response []byte
	// Err is non-nil when the handler reported an error via the /error endpoint.
	Err []byte
}

// startRuntimeAPI starts the server and registers t.Cleanup to close it.
func startRuntimeAPI(t *testing.T) *runtimeAPI {
	t.Helper()
	api := &runtimeAPI{
		Events: make(chan []byte, 1),
		Done:   make(chan invocationResult, 8),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/2018-06-01/runtime/invocation/next", func(w http.ResponseWriter, _ *http.Request) {
		payload := <-api.Events
		w.Header().Set("Lambda-Runtime-Aws-Request-Id", "test-request-id")
		w.Header().Set("Lambda-Runtime-Deadline-Ms",
			fmt.Sprintf("%d", time.Now().Add(30*time.Second).UnixMilli()))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/2018-06-01/runtime/invocation/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusAccepted)
		if strings.HasSuffix(r.URL.Path, "/response") {
			api.Done <- invocationResult{Response: body}
		} else if strings.HasSuffix(r.URL.Path, "/error") {
			api.Done <- invocationResult{Err: body}
		}
	})
	mux.HandleFunc("/2018-06-01/runtime/init/error", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusAccepted)
		t.Errorf("lambda init error: %s", body)
	})

	api.server = httptest.NewServer(mux)
	api.Addr = strings.TrimPrefix(api.server.URL, "http://")
	// Do NOT close the server in t.Cleanup: the lambda.Start goroutine blocks
	// on the next GET /next request. Closing the server causes an HTTP error
	// which triggers logFatalf → os.Exit(1) in the lambda runtime client.
	// The goroutine is harmlessly abandoned when the test binary exits.
	return api
}
