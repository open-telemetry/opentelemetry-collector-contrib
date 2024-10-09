// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func FuzzHandleReq(f *testing.F) {
	f.Fuzz(func(t *testing.T, reqBody []byte, useGzip bool) {
		req, err := http.NewRequest("POST", "http://example.com", bytes.NewReader(reqBody))
		if err != nil {
			t.Skip()
		}
		if useGzip {
			req.Header.Add("Content-Encoding", "gzip")
		}

		consumer := consumertest.NewNop()
		receiver, err := newLogsReceiver(receivertest.NewNopSettings(), Config{ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:8080",
		}}, consumer)
		if err != nil {
			t.Fatal(err)
		}

		r := receiver.(*eventReceiver)

		w := httptest.NewRecorder()
		r.handleReq(w, req, httprouter.ParamsFromContext(context.Background()))
	})
}
