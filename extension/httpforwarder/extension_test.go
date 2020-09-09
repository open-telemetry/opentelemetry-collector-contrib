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

package httpforwarder

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"
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
		httpErrorFromBackend        bool
		requestErrorAtForwarder     bool
		clientRequestArgs           clientRequestArgs
	}{
		{
			name: "No additional headers",
			config: &Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: listenAt,
				},
			},
			expectedbackendStatusCode:   http.StatusAccepted,
			expectedBackendResponseBody: []byte("hello world"),
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
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: listenAt,
				},
				Upstream: confighttp.HTTPClientSettings{
					Headers: map[string]string{
						"key": "value",
					},
				},
			},
			expectedbackendStatusCode:   http.StatusAccepted,
			expectedBackendResponseBody: []byte("hello world with additional headers"),
			clientRequestArgs: clientRequestArgs{
				method: "PUT",
				url:    fmt.Sprintf("http://%s/api/dosomething", listenAt),
			},
		},
		{
			name: "Error code from backend",
			config: &Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: listenAt,
				},
				Upstream: confighttp.HTTPClientSettings{
					Headers: map[string]string{
						"key": "value",
					},
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
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: listenAt,
				},
				Upstream: confighttp.HTTPClientSettings{
					Headers: map[string]string{
						"key": "value",
					},
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

				// Assert additional headers added by forwarder.
				for k, v := range test.config.Upstream.Headers {
					got := r.Header.Get(k)
					assert.Equal(t, v, got)
				}

				w.WriteHeader(test.expectedbackendStatusCode)
				w.Write(test.expectedBackendResponseBody)
			}))
			defer backend.Close()

			// Fill in final destination URL.
			backendURL, _ := url.Parse(backend.URL)
			test.config.Upstream.Endpoint = backendURL.String()

			// Setup forwarder with wrong final address to mock failures.
			if test.requestErrorAtForwarder {
				test.config.Upstream.Endpoint = "http://" + testutil.GetAvailableLocalAddress(t)
			}

			hf, err := newHTTPForwarder(test.config, zap.NewNop())
			require.NoError(t, err)

			ctx := context.Background()
			require.NoError(t, hf.Start(ctx, componenttest.NewNopHost()))

			// Mock a client trying to talk to backend using the forwarder.
			httpClient := http.Client{}

			// Assert responses received by client.
			response, err := httpClient.Do(httpRequest(t, test.clientRequestArgs))
			require.NoError(t, err)
			require.NotNil(t, response)
			defer response.Body.Close()

			assert.Equal(t, test.expectedbackendStatusCode, response.StatusCode)
			if !test.requestErrorAtForwarder {
				assert.Equal(t, string(test.expectedBackendResponseBody), string(readBody(response.Body)))
			}

			require.NoError(t, hf.Shutdown(ctx))
		})
	}
}

func httpRequest(t *testing.T, args clientRequestArgs) *http.Request {
	r, err := http.NewRequest(args.method, args.url, ioutil.NopCloser(strings.NewReader(args.body)))
	require.NoError(t, err)

	for k, v := range args.headers {
		r.Header.Set(k, v)
	}

	return r
}

func readBody(body io.ReadCloser) []byte {
	out, _ := ioutil.ReadAll(body)
	return out
}
