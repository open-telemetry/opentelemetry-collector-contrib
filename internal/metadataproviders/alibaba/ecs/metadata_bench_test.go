// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"
)

func BenchmarkMetadata(b *testing.B) {
	doer := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == tokenURL {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("bench-token")),
			}, nil
		}
		time.Sleep(2 * time.Millisecond) // simulate IMDS latency
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("bench-value")),
		}, nil
	})
	p := &metadataClient{
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: doer,
		},
	}

	ctx := b.Context()
	for b.Loop() {
		_, _ = p.Metadata(ctx)
	}
}
