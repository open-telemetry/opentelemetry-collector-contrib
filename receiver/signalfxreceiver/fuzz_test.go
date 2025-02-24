// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver/internal/metadata"
)

func FuzzHandleDatapointReq(f *testing.F) {
	f.Fuzz(func(t *testing.T, reqBody []byte) {
		req, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(reqBody))
		if err != nil {
			t.Skip()
		}
		req.Header.Add(httpContentTypeHeader, otlpProtobufContentType)
		rec, err := newReceiver(receivertest.NewNopSettings(metadata.Type), Config{})
		if err != nil {
			t.Fatal(err)
		}
		sink := new(consumertest.MetricsSink)
		rec.RegisterMetricsConsumer(sink)
		rec.handleDatapointReq(httptest.NewRecorder(), req)
	})
}
