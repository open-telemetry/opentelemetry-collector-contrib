// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

func TestCreateNewLogReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)

	userDefinedServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	userDefinedServerConfig.WriteTimeout = 0
	userDefinedServerConfig.ReadHeaderTimeout = 0
	userDefinedServerConfig.IdleTimeout = 0
	userDefinedServerConfig.KeepAlivesEnabled = false
	userDefinedServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:8080",
	}

	headerRegexServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	headerRegexServerConfig.WriteTimeout = 0
	headerRegexServerConfig.ReadHeaderTimeout = 0
	headerRegexServerConfig.IdleTimeout = 0
	headerRegexServerConfig.KeepAlivesEnabled = false
	headerRegexServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:8080",
	}

	readTimeoutServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	readTimeoutServerConfig.WriteTimeout = 0
	readTimeoutServerConfig.ReadHeaderTimeout = 0
	readTimeoutServerConfig.IdleTimeout = 0
	readTimeoutServerConfig.KeepAlivesEnabled = false
	readTimeoutServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:8080",
	}

	writeTimeoutServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	writeTimeoutServerConfig.WriteTimeout = 0
	writeTimeoutServerConfig.ReadHeaderTimeout = 0
	writeTimeoutServerConfig.IdleTimeout = 0
	writeTimeoutServerConfig.KeepAlivesEnabled = false
	writeTimeoutServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:8080",
	}

	regexCompileServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	regexCompileServerConfig.WriteTimeout = 0
	regexCompileServerConfig.ReadHeaderTimeout = 0
	regexCompileServerConfig.IdleTimeout = 0
	regexCompileServerConfig.KeepAlivesEnabled = false
	regexCompileServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:8080",
	}

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
			err:      errMissingEndpointFromConfig,
		},
		{
			desc: "User defined config success",
			cfg: Config{
				ServerConfig: userDefinedServerConfig,
				ReadTimeout:  "5s",
				WriteTimeout: "5s",
				Path:         "/event",
				HealthPath:   "/health",
				RequiredHeader: RequiredHeader{
					Key:   "key-present",
					Value: "value-present",
				},
			},
			consumer: consumertest.NewNop(),
		},
		{
			desc: "User defined config success with header_attribute_regex supplied",
			cfg: Config{
				ServerConfig: headerRegexServerConfig,
				ReadTimeout:  "5s",
				WriteTimeout: "5s",
				Path:         "/event",
				HealthPath:   "/health",
				RequiredHeader: RequiredHeader{
					Key:   "key-present",
					Value: "value-present",
				},
				HeaderAttributeRegex: ".+",
			},
			consumer: consumertest.NewNop(),
		},
		{
			desc: "User defined read timeout exceeds max value",
			cfg: Config{
				ServerConfig: readTimeoutServerConfig,
				ReadTimeout:  "11s",
				WriteTimeout: "5s",
				Path:         "/event",
				HealthPath:   "/health",
				RequiredHeader: RequiredHeader{
					Key:   "key-present",
					Value: "value-present",
				},
			},
			consumer: consumertest.NewNop(),
			err:      errReadTimeoutExceedsMaxValue,
		},
		{
			desc: "User defined write timeout exceeds max value",
			cfg: Config{
				ServerConfig: writeTimeoutServerConfig,
				ReadTimeout:  "5s",
				WriteTimeout: "11s",
				Path:         "/event",
				HealthPath:   "/health",
				RequiredHeader: RequiredHeader{
					Key:   "key-present",
					Value: "value-present",
				},
			},
			consumer: consumertest.NewNop(),
			err:      errWriteTimeoutExceedsMaxValue,
		},
		{
			desc: "User defined regex fails to compile",
			cfg: Config{
				ServerConfig: regexCompileServerConfig,
				ReadTimeout:  "5s",
				WriteTimeout: "5s",
				Path:         "/event",
				HealthPath:   "/health",
				RequiredHeader: RequiredHeader{
					Key:   "key-present",
					Value: "value-present",
				},
				HeaderAttributeRegex: "\\q", // some bogus regex value that will not compile
			},
			consumer: consumertest.NewNop(),
			err:      errHeaderAttributeRegexCompile,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			rec, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), test.cfg, test.consumer)
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
	cfg.NetAddr.Endpoint = "localhost:0"

	tests := []struct {
		desc string
		cfg  Config
		req  *http.Request
	}{
		{
			desc: "Good request",
			cfg:  *cfg,
			req:  httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader("test")),
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
				err = gzipWriter.Close()
				require.NoError(t, err, "Gzip writer failed to close")

				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", &msg)
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
		},
		{
			desc: "Multiple logs",
			cfg:  *cfg,
			req:  httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader("log1\nlog2")),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			consumer := consumertest.NewNop()
			receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), test.cfg, consumer)
			require.NoError(t, err, "Failed to create receiver")

			r := receiver.(*eventReceiver)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()), "Failed to start receiver")
			defer func() {
				require.NoError(t, r.Shutdown(t.Context()), "Failed to shutdown receiver")
			}()

			w := httptest.NewRecorder()
			r.handleReq(w, test.req, httprouter.ParamsFromContext(t.Context()))

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
	cfg.NetAddr.Endpoint = "localhost:0"
	headerCfg := createDefaultConfig().(*Config)
	headerCfg.NetAddr.Endpoint = "localhost:0"
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
			req:    httptest.NewRequest(http.MethodGet, "http://localhost/events", http.NoBody),
			status: http.StatusBadRequest,
		},
		{
			desc:   "Empty body",
			cfg:    *cfg,
			req:    httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader("")),
			status: http.StatusBadRequest,
		},
		{
			desc: "Invalid encoding",
			cfg:  *cfg,
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader("test"))
				req.Header.Set("Content-Encoding", "glizzy")
				return req
			}(),
			status: http.StatusUnsupportedMediaType,
		},
		{
			desc: "Valid content encoding header invalid data",
			cfg:  *cfg,
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader("notzipped"))
				req.Header.Set("Content-Encoding", "gzip")
				return req
			}(),
			status: http.StatusBadRequest,
		},
		{
			desc: "Invalid required header value",
			cfg:  *headerCfg,
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader("test"))
				req.Header.Set("key-present", "incorrect-value")
				return req
			}(),
			status: http.StatusUnauthorized,
		},
		{
			desc: "Request body exceeds max size",
			cfg: func() Config {
				c := createDefaultConfig().(*Config)
				c.NetAddr.Endpoint = "localhost:0"
				c.MaxRequestBodySize = 70 * 1024 // Set to 70KB limit
				c.SplitLogsAtNewLine = true
				return *c
			}(),
			req: func() *http.Request {
				// Create a payload larger than 70KB to ensure it exceeds the limit
				largeBody := strings.Repeat("X", 80*1024)
				return httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(largeBody))
			}(),
			status: http.StatusBadRequest,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			consumer := consumertest.NewNop()
			receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), test.cfg, consumer)
			require.NoError(t, err, "Failed to create receiver")

			r := receiver.(*eventReceiver)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()), "Failed to start receiver")
			defer func() {
				require.NoError(t, r.Shutdown(t.Context()), "Failed to shutdown receiver")
			}()

			w := httptest.NewRecorder()
			r.handleReq(w, test.req, httprouter.ParamsFromContext(t.Context()))

			response := w.Result()
			require.Equal(t, test.status, response.StatusCode)
		})
	}
}

// computeHMACSHA256 is a test helper that computes an HMAC-SHA256 hex digest.
func computeHMACSHA256(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestHMACSignatureSuccess(t *testing.T) {
	secret := "webhook-secret"
	body := `{"event": "push", "ref": "refs/heads/main"}`
	signature := "sha256=" + computeHMACSHA256(secret, body)

	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "localhost:0"
	cfg.HMACSignature = HMACSignature{
		Secret: "webhook-secret",
		Header: "X-Hub-Signature-256",
		Prefix: "sha256=",
	}

	consumer := consumertest.NewNop()
	receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), *cfg, consumer)
	require.NoError(t, err)

	r := receiver.(*eventReceiver)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, r.Shutdown(t.Context()))
	}()

	req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signature)

	w := httptest.NewRecorder()
	r.handleReq(w, req, httprouter.ParamsFromContext(t.Context()))
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestHMACSignatureWithFingerprintPrefix(t *testing.T) {
	secret := "fp-secret"
	body := `{"requestId": "abc123"}`
	signature := "v1=" + computeHMACSHA256(secret, body)

	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "localhost:0"
	cfg.HMACSignature = HMACSignature{
		Secret: "fp-secret",
		Header: "fpjs-event-signature",
		Prefix: "v1=",
	}

	consumer := consumertest.NewNop()
	receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), *cfg, consumer)
	require.NoError(t, err)

	r := receiver.(*eventReceiver)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, r.Shutdown(t.Context()))
	}()

	req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(body))
	req.Header.Set("fpjs-event-signature", signature)

	w := httptest.NewRecorder()
	r.handleReq(w, req, httprouter.ParamsFromContext(t.Context()))
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestHMACSignatureFailures(t *testing.T) {
	secret := "webhook-secret"
	body := `{"event": "push"}`
	validSig := "sha256=" + computeHMACSHA256(secret, body)

	hmacCfg := func() Config {
		cfg := createDefaultConfig().(*Config)
		cfg.NetAddr.Endpoint = "localhost:0"
		cfg.HMACSignature = HMACSignature{
			Secret: "webhook-secret",
			Header: "X-Hub-Signature-256",
			Prefix: "sha256=",
		}
		return *cfg
	}

	tests := []struct {
		desc   string
		cfg    Config
		req    *http.Request
		status int
		errMsg string
	}{
		{
			desc:   "Missing signature header",
			cfg:    hmacCfg(),
			req:    httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(body)),
			status: http.StatusUnauthorized,
			errMsg: "missing HMAC signature header",
		},
		{
			desc: "Invalid signature prefix",
			cfg:  hmacCfg(),
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(body))
				req.Header.Set("X-Hub-Signature-256", "md5=abc123")
				return req
			}(),
			status: http.StatusUnauthorized,
			errMsg: "invalid prefix",
		},
		{
			desc: "Invalid hex encoding",
			cfg:  hmacCfg(),
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(body))
				req.Header.Set("X-Hub-Signature-256", "sha256=nothex!!!")
				return req
			}(),
			status: http.StatusUnauthorized,
			errMsg: "hex encoding is invalid",
		},
		{
			desc: "Wrong secret produces signature mismatch",
			cfg:  hmacCfg(),
			req: func() *http.Request {
				wrongSig := "sha256=" + computeHMACSHA256("wrong-secret", body)
				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(body))
				req.Header.Set("X-Hub-Signature-256", wrongSig)
				return req
			}(),
			status: http.StatusUnauthorized,
			errMsg: "does not match",
		},
		{
			desc: "Tampered body produces signature mismatch",
			cfg:  hmacCfg(),
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "http://localhost/events", strings.NewReader(`{"event": "tampered"}`))
				req.Header.Set("X-Hub-Signature-256", validSig)
				return req
			}(),
			status: http.StatusUnauthorized,
			errMsg: "does not match",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			consumer := consumertest.NewNop()
			receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), test.cfg, consumer)
			require.NoError(t, err)

			r := receiver.(*eventReceiver)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			defer func() {
				require.NoError(t, r.Shutdown(t.Context()))
			}()

			w := httptest.NewRecorder()
			r.handleReq(w, test.req, httprouter.ParamsFromContext(t.Context()))
			require.Equal(t, test.status, w.Result().StatusCode)
		})
	}
}

func TestHealthCheck(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.NetAddr.Endpoint = "localhost:0"
	consumer := consumertest.NewNop()
	receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), *defaultConfig, consumer)
	require.NoError(t, err, "failed to create receiver")

	r := receiver.(*eventReceiver)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()), "failed to start receiver")
	defer func() {
		require.NoError(t, r.Shutdown(t.Context()), "failed to shutdown receiver")
	}()

	w := httptest.NewRecorder()
	r.handleHealthCheck(w, httptest.NewRequest(http.MethodGet, "http://localhost/health", http.NoBody), httprouter.ParamsFromContext(t.Context()))

	response := w.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
}
