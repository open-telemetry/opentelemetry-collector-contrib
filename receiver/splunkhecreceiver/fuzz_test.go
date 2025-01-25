// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"bytes"
	"net/http"

	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func FuzzHandleRawReq(f *testing.F) {
	f.Fuzz(func(t *testing.T, reqBody []byte) {
		req, err := http.NewRequest("POST", "http://example.com", bytes.NewReader(reqBody))
		if err != nil {
			t.Skip()
		}
		req.Header.Add("If-None-Match", `W/"wyzzy"`)
		sink := new(consumertest.LogsSink)
		defaultConfig := createDefaultConfig().(*Config)
		rcv, err := newLogsReceiver(receivertest.NewNopSettings(), *defaultConfig, sink)
		if err != nil {
			t.Fatal(err)
		}
		r := rcv.(*splunkReceiver)
		w := httptest.NewRecorder()
		r.handleRawReq(w, req)
	})
}
