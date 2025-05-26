// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudflarereceiver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func FuzzHandleReq(f *testing.F) {
	f.Fuzz(func(t *testing.T, reqBody []byte, gZip bool) {
		req, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(reqBody))
		if err != nil {
			t.Skip()
		}
		req.Header.Add(secretHeaderName, "abc123")
		req.Header.Add("Content-Type", "text/plain; charset=utf-8")
		if gZip {
			req.Header.Add("Content-Encoding", "gzip")
		}
		consumer := &consumertest.LogsSink{}

		r := newReceiver(t, &Config{
			Logs: LogsConfig{
				Endpoint:       "localhost:0",
				Secret:         "abc123",
				TimestampField: "MyTimestamp",
				Attributes: map[string]string{
					"ClientIP": "http_request.client_ip",
				},
				TLS: &configtls.ServerConfig{},
			},
		},
			consumer,
		)
		rec := httptest.NewRecorder()
		r.handleRequest(rec, req)
	})
}
