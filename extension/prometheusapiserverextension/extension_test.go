// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusapiserverextension

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

type clientRequestArgs struct {
	method  string
	url     string
	headers map[string]string
	body    string
}

func TestExtension(t *testing.T) {
	listenAt := testutil.GetAvailableLocalAddress(t)
	tests := []struct {
		name                        string
		config                      *Config
		expectedbackendStatusCode   int
		expectedBackendResponseBody []byte
		expectedHeaders             map[string]configopaque.String
		httpErrorFromBackend        bool
		requestErrorAtForwarder     bool
		clientRequestArgs           clientRequestArgs
		startUpError                bool
		startUpErrorMessage         string
	}{
		{
			name: "No additional headers",
			config: &Config{
				Server: confighttp.ServerConfig{
					Endpoint: listenAt,
				},
			},
			expectedbackendStatusCode:   http.StatusAccepted,
			expectedBackendResponseBody: []byte("hello world"),
			expectedHeaders: map[string]configopaque.String{
				"header": "value",
			},
			clientRequestArgs: clientRequestArgs{
				method: "GET",
				url:    fmt.Sprintf("http://%s/api/dosomething", listenAt),
				headers: map[string]string{
					"client_header": "val1",
				},
				body: "client_body",
			},
		},
		{
			name: "With additional headers",
			config: &Config{
				Server: confighttp.ServerConfig{
					Endpoint: listenAt,
				},
			},
			expectedbackendStatusCode:   http.StatusAccepted,
			expectedBackendResponseBody: []byte("hello world with additional headers"),
			expectedHeaders: map[string]configopaque.String{
				"header": "value",
			},
			clientRequestArgs: clientRequestArgs{
				method: "PUT",
				url:    fmt.Sprintf("http://%s/api/dosomething", listenAt),
			},
		},
		{
			name: "Error code from backend",
			config: &Config{
				Server: confighttp.ServerConfig{
					Endpoint: listenAt,
				},
			},
			expectedbackendStatusCode:   http.StatusInternalServerError,
			expectedBackendResponseBody: []byte("\n"),
			httpErrorFromBackend:        true,
			clientRequestArgs: clientRequestArgs{
				method: "PATCH",
				url:    fmt.Sprintf("http://%s/api/dosomething", listenAt),
			},
		},
		{
			name: "Error making request at forwarder",
			config: &Config{
				Server: confighttp.ServerConfig{
					Endpoint: listenAt,
				},
			},
			expectedbackendStatusCode:   http.StatusBadGateway,
			expectedBackendResponseBody: []byte("\n"),
			requestErrorAtForwarder:     true,
			clientRequestArgs: clientRequestArgs{
				method: "GET",
				url:    fmt.Sprintf("http://%s/api/dosomething", listenAt),
			},
		},
		{
			name: "Error on Startup",
			config: &Config{
				Server: confighttp.ServerConfig{
					Endpoint: "invalid", // to mock error setting up listener.
				},
			},
			startUpError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.httpErrorFromBackend {
					http.Error(w, "", http.StatusInternalServerError)
					return
				}

				assert.Equal(t, getParsedURL(t, test.clientRequestArgs.url).RequestURI(), r.RequestURI)
				assert.Equal(t, test.clientRequestArgs.method, r.Method)
				assert.Equal(t, test.clientRequestArgs.body, string(readBody(r.Body)))

				// Assert headers originating from client.
				for k, v := range test.clientRequestArgs.headers {
					got := r.Header.Get(k)
					assert.Equal(t, v, got)
				}

				// Assert Via header added by the forwarder on all requests.
				assert.Equal(t, fmt.Sprintf("%s %s", r.Proto, listenAt), r.Header.Get("Via"))

				for k, v := range test.expectedHeaders {
					w.Header().Set(k, string(v))
				}
				w.WriteHeader(test.expectedbackendStatusCode)
				_, err := w.Write(test.expectedBackendResponseBody)
				assert.NoError(t, err)
			}))
			defer backend.Close()

			// Fill in final destination URL.
			backendURL, _ := url.Parse(backend.URL)
			test.config.Server.Endpoint = backendURL.String()

			// Setup forwarder with wrong final address to mock failures.
			if test.requestErrorAtForwarder {
				test.config.Server.Endpoint = "http://" + testutil.GetAvailableLocalAddress(t)
			}
		})
	}
}

func httpRequest(t *testing.T, args clientRequestArgs) *http.Request {
	r, err := http.NewRequest(args.method, args.url, io.NopCloser(strings.NewReader(args.body)))
	require.NoError(t, err)

	for k, v := range args.headers {
		r.Header.Set(k, v)
	}

	return r
}

func readBody(body io.ReadCloser) []byte {
	out, _ := io.ReadAll(body)
	return out
}

func getParsedURL(t *testing.T, rawURL string) *url.URL {
	var url, err = url.Parse(rawURL)
	require.NoError(t, err)
	return url
}
