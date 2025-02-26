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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

func FuzzHandleRawReq(f *testing.F) {
	f.Fuzz(func(t *testing.T, reqBody []byte) {
		req, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(reqBody))
		if err != nil {
			t.Skip()
		}
		req.Header.Add("If-None-Match", `W/"wyzzy"`)
		sink := new(consumertest.LogsSink)
		defaultConfig := createDefaultConfig().(*Config)
		rcv, err := newReceiver(receivertest.NewNopSettings(metadata.Type), *defaultConfig)
		rcv.logsConsumer = sink
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		rcv.handleRawReq(w, req)
	})
}
