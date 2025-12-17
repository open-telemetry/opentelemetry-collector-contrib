// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

type testServerResponse struct {
	Status  int
	Headers http.Header
	Body    []byte
}

type testServerRequest struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// testServer is a tiny HTTP test server that returns queued responses per method+path
// and records incoming requests for assertions.
type testServer struct {
	mu       sync.Mutex
	scripts  map[string][]testServerResponse
	requests map[string][]testServerRequest
	server   *httptest.Server
}

func newTestServer() *testServer {
	ts := &testServer{
		scripts:  make(map[string][]testServerResponse),
		requests: make(map[string][]testServerRequest),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path

		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		ts.mu.Lock()
		ts.requests[key] = append(ts.requests[key], testServerRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Header: r.Header.Clone(),
			Body:   body,
		})

		resp := testServerResponse{Status: http.StatusOK, Body: []byte(`{}`)}
		if queue := ts.scripts[key]; len(queue) > 0 {
			resp = queue[0]
			ts.scripts[key] = queue[1:]
		}
		ts.mu.Unlock()

		for k, vs := range resp.Headers {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		status := resp.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(resp.Body)
	})

	ts.server = httptest.NewServer(mux)
	return ts
}

func (ts *testServer) close() {
	if ts.server != nil {
		ts.server.Close()
	}
}

func (ts *testServer) queue(method, path string, resp testServerResponse) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	key := method + " " + path
	ts.scripts[key] = append(ts.scripts[key], resp)
}

func (ts *testServer) recorded(method, path string) []testServerRequest {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	key := method + " " + path
	out := append([]testServerRequest(nil), ts.requests[key]...)
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func dsnForServer(pubKey string, ts *testServer) string {
	return strings.Replace(ts.server.URL, "http://", "http://"+pubKey+"@", 1) + "/1"
}
