// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"strings"
	"testing"

	grpcgzip "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
)

func assertAcceptEncodingGzip(tb testing.TB, md metadata.MD) {
	tb.Helper()

	values := md.Get("grpc-accept-encoding")
	if len(values) == 0 {
		tb.Fatalf("expected grpc-accept-encoding metadata")
	}

	// The grpc-accept-encoding header contains a comma-separated list of all
	// compressors the client is willing to accept. We just need to verify that
	// gzip is in the list.
	acceptEncoding := values[0]
	encodings := strings.Split(acceptEncoding, ",")
	found := false
	for _, enc := range encodings {
		if strings.TrimSpace(enc) == grpcgzip.Name {
			found = true
			break
		}
	}
	if !found {
		tb.Fatalf("expected grpc-accept-encoding to contain %q, got %q", grpcgzip.Name, acceptEncoding)
	}
}
