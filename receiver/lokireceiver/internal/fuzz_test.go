// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"net/http"
	"testing"
)

func FuzzParseRequest(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte, headerType uint8) {
		req, err := http.NewRequest("POST", "http://example.com", bytes.NewReader(data))
		if err != nil {
			t.Skip()
		}
		switch int(headerType) % 3 {
		case 0:
			req.Header.Add("Content-Encoding", "snappy")
		case 1:
			req.Header.Add("Content-Encoding", "gzip")
		case 2:
			req.Header.Add("Content-Encoding", "deflat")
		}
		_, _ = ParseRequest(req)
	})
}
