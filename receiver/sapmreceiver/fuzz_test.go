// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmreceiver

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/signalfx/sapm-proto/sapmprotocol"
)

func FuzzParseTraceV2Request(f *testing.F) {
	f.Fuzz(func(t *testing.T, reqBody []byte, encoding uint8) {
		req, err := http.NewRequest("POST", "http://example.com", bytes.NewReader(reqBody))
		if err != nil {
			t.Skip()
		}
		req.Header.Add("Content-Type", "application/x-protobuf")
		switch int(encoding) % 3 {
		case 0:
			req.Header.Add("Content-Encoding", "gzip")
		case 1:
			req.Header.Add("Content-Encoding", "zstd")
		default:
		}

		sapmprotocol.ParseTraceV2Request(req)
	})
}
