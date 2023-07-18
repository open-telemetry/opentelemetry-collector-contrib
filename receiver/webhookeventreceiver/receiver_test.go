// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateNewLogReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)

	tests := []struct {
		desc     string
		cfg      Config
		consumer consumer.Logs
		err      error
	}{
		{
			desc:     "Default config fails (no endpoint)",
			cfg:      *defaultConfig,
			consumer: consumertest.NewNop(),
			err:      errMissingEndpoint,
		},
		{
			desc: "User defined config success",
			cfg: Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: "localhost:8080",
				},
				ReadTimeout:  "543",
				WriteTimeout: "210",
				Path:         "/event",
				HealthPath:   "/health",
				RequiredHeader:     RequiredHeader{
					Key:   "key-present",
					Value: "value-present",
				},
			},
			consumer: consumertest.NewNop(),
		},
		{
			desc: "Missing consumer fails",
			cfg:  *defaultConfig,
			err:  errNilLogsConsumer,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			rec, err := newLogsReceiver(receivertest.NewNopCreateSettings(), test.cfg, test.consumer)
			if test.err == nil {
				require.NotNil(t, rec)
			} else {
				require.ErrorIs(t, err, test.err)
				require.Nil(t, rec)
			}
		})
	}
}

// these requests should all succeed
func TestHandleReq(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0"

	tests := []struct {
		desc string
		cfg  Config
		req  *http.Request
	}{
		{
			desc: "Good request",
			cfg:  *cfg,
			req:  httptest.NewRequest("POST", "http://localhost/events", strings.NewReader("test")),
		},
		{
			desc: "Good request with gzip",
			cfg:  *cfg,
			req: func() *http.Request {
				// create gzip encoded message
				msgStruct := struct {
					Field1 string
					Field2 int
					Field3 string
				}{
					Field1: "hello",
					Field2: 42,
					Field3: "world",
				}
				msgJSON, err := json.Marshal(msgStruct)
				require.NoError(t, err, "failed to marshall message into valid json")

				var msg bytes.Buffer
				gzipWriter := gzip.NewWriter(&msg)
				_, err = gzipWriter.Write(msgJSON)
				require.NoError(t, err, "Gzip writer failed")

				req := httptest.NewRequest("POST", "http://localhost/events", &msg)
				return req
			}(),
		},
		{
			desc: "Multiple logs",
			cfg:  *cfg,
			req:  httptest.NewRequest("POST", "http://localhost/events", strings.NewReader("log1\nlog2")),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			consumer := consumertest.NewNop()
			receiver, err := newLogsReceiver(receivertest.NewNopCreateSettings(), test.cfg, consumer)
			require.NoError(t, err, "Failed to create receiver")

			r := receiver.(*eventReceiver)
			require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()), "Failed to start receiver")
			defer func() {
				require.NoError(t, r.Shutdown(context.Background()), "Failed to shutdown receiver")
			}()

			w := httptest.NewRecorder()
			r.handleReq(w, test.req, httprouter.ParamsFromContext(context.Background()))

			response := w.Result()
			_, err = io.ReadAll(response.Body)
			require.NoError(t, err, "Failed to read message body")

			require.Equal(t, http.StatusOK, response.StatusCode)
		})
	}
}

// failure in its many forms
func TestFailedReq(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0"
	headerCfg := createDefaultConfig().(*Config)
	headerCfg.Endpoint = "localhost:0" 
	headerCfg.RequiredHeader.Key = "key-present"
	headerCfg.RequiredHeader.Value = "value-present"

	tests := []struct {
		desc   string
		cfg    Config
		req    *http.Request
		status int
	}{
		{
			desc:   "Invalid method",
			cfg:    *cfg,
			req:    httptest.NewRequest("GET", "http://localhost/events", nil),
			status: http.StatusBadRequest,
		},
		{
			desc:   "Empty body",
			cfg:    *cfg,
			req:    httptest.NewRequest("POST", "http://localhost/events", strings.NewReader("")),
			status: http.StatusBadRequest,
		},
		{
			desc: "Invalid encoding",
			cfg:  *cfg,
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/events", strings.NewReader("test"))
				req.Header.Set("Content-Encoding", "glizzy")
				return req
			}(),
			status: http.StatusUnsupportedMediaType,
		},
		{
			desc: "Valid content encoding header invalid data",
			cfg:  *cfg,
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/events", strings.NewReader("notzipped"))
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			status: http.StatusBadRequest,
		},
		{
			desc: "Invalid required header value",
			cfg:  *headerCfg,
			req: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost/events", strings.NewReader("test"))
				req.Header.Set("key-present", "incorrect-value")
				return req
			}(),
			status: http.StatusUnauthorized,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			consumer := consumertest.NewNop()
			receiver, err := newLogsReceiver(receivertest.NewNopCreateSettings(), test.cfg, consumer)
			require.NoError(t, err, "Failed to create receiver")

			r := receiver.(*eventReceiver)
			require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()), "Failed to start receiver")
			defer func() {
				require.NoError(t, r.Shutdown(context.Background()), "Failed to shutdown receiver")
			}()

			w := httptest.NewRecorder()
			r.handleReq(w, test.req, httprouter.ParamsFromContext(context.Background()))

			response := w.Result()
			require.Equal(t, test.status, response.StatusCode)
		})
	}
}

func TestHealthCheck(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.Endpoint = "localhost:0"
	consumer := consumertest.NewNop()
	receiver, err := newLogsReceiver(receivertest.NewNopCreateSettings(), *defaultConfig, consumer)
	require.NoError(t, err, "failed to create receiver")

	r := receiver.(*eventReceiver)
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()), "failed to start receiver")
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()), "failed to shutdown revceiver")
	}()

	w := httptest.NewRecorder()
	r.handleHealthCheck(w, httptest.NewRequest("GET", "http://localhost/health", nil), httprouter.ParamsFromContext(context.Background()))

	response := w.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
}
