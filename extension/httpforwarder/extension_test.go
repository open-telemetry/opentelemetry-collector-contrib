// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarder

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"

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
				Ingress: confighttp.HTTPServerSettings{
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
				Ingress: confighttp.HTTPServerSettings{
					Endpoint: listenAt,
				},
				Egress: confighttp.HTTPClientSettings{
					Headers: map[string]configopaque.String{
						"key": "value",
					},
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
				Ingress: confighttp.HTTPServerSettings{
					Endpoint: listenAt,
				},
				Egress: confighttp.HTTPClientSettings{
					Headers: map[string]configopaque.String{
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
				Ingress: confighttp.HTTPServerSettings{
					Endpoint: listenAt,
				},
				Egress: confighttp.HTTPClientSettings{
					Headers: map[string]configopaque.String{
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
		{
			name: "Invalid config - HTTP Client creation fails",
			config: &Config{
				Egress: confighttp.HTTPClientSettings{
					Endpoint: "localhost:9090",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "/non/existent",
						},
					},
				},
			},
			startUpError:        true,
			startUpErrorMessage: "failed to create HTTP Client: ",
		},
		{
			name: "Error on Startup",
			config: &Config{
				Ingress: confighttp.HTTPServerSettings{
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

				// Assert additional headers added by forwarder.
				for k, v := range test.config.Egress.Headers {
					got := r.Header.Get(k)
					assert.Equal(t, string(v), got)
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
			test.config.Egress.Endpoint = backendURL.String()

			// Setup forwarder with wrong final address to mock failures.
			if test.requestErrorAtForwarder {
				test.config.Egress.Endpoint = "http://" + testutil.GetAvailableLocalAddress(t)
			}

			hf, err := newHTTPForwarder(test.config, componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			ctx := context.Background()
			if test.startUpError {
				err = hf.Start(ctx, componenttest.NewNopHost())
				if test.startUpErrorMessage != "" {
					require.True(t, strings.Contains(err.Error(), test.startUpErrorMessage))
				}
				require.Error(t, err)

				return
			}
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
				assert.Equal(t, fmt.Sprintf("%s %s", response.Proto, listenAt), response.Header.Get("Via"))
			}

			for k := range response.Header {
				got := response.Header.Get(k)
				header := strings.ToLower(k)
				if want, ok := test.expectedHeaders[header]; ok {
					assert.Equal(t, want, configopaque.String(got))
					continue
				}

				if k == "Content-Length" || k == "Content-Type" || k == "X-Content-Type-Options" || k == "Date" || k == "Via" {
					// Content-Length, Content-Type, X-Content-Type-Options and Date are certain headers added by default.
					// Assertion for Via is done above.
					continue
				}
				t.Error("unexpected header found in response: ", k)
			}

			require.NoError(t, hf.Shutdown(ctx))
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
