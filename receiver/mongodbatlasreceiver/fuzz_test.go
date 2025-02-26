// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

func FuzzHandleReq(f *testing.F) {
	f.Fuzz(func(t *testing.T, reqBody []byte, payloadSigHeader string) {
		req, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewReader(reqBody))
		if err != nil {
			t.Skip()
		}
		req.Header.Add(signatureHeaderName, payloadSigHeader)
		consumer := &consumertest.LogsSink{}

		set := receivertest.NewNopSettings(metadata.Type)
		set.Logger = zaptest.NewLogger(t)
		ar, err := newAlertsReceiver(set, &Config{Alerts: AlertConfig{Secret: "some_secret"}}, consumer)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		ar.handleRequest(rec, req)
	})
}
