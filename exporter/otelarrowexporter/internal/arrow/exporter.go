// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"

import (
	"context"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	"google.golang.org/grpc"
)

// Exporter exports OpenTelemetry Protocol with Apache Arrow protocol
// data for a specific signal.  One of these structs is created per
// baseExporter, in the top-level module, when Arrow is enabled.
type Exporter struct {
	// TODO: Implementation
}

// AnyStreamClient is the interface supported by all Arrow streams,
// i.e., any of the Arrow-supported signals having a single method w/
// the appropriate per-signal name.
type AnyStreamClient interface {
	Send(*arrowpb.BatchArrowRecords) error
	Recv() (*arrowpb.BatchStatus, error)
	grpc.ClientStream
}

// StreamClientFunc is a constructor for AnyStreamClients.  These return
// the method name to assist with instrumentation, since the gRPC stats
// handler isn't able to see the correct uncompressed size.
type StreamClientFunc func(context.Context, ...grpc.CallOption) (AnyStreamClient, string, error)

// MakeAnyStreamClient accepts any Arrow-like stream, which is one of
// the Arrow-supported signals having a single method w/ the
// appropriate name, and turns it into an AnyStreamClient.  The method
// name is carried through because once constructed, gRPC clients will
// not reveal their service and method names.
func MakeAnyStreamClient[T AnyStreamClient](method string, clientFunc func(ctx context.Context, opts ...grpc.CallOption) (T, error)) StreamClientFunc {
	return func(ctx context.Context, opts ...grpc.CallOption) (AnyStreamClient, string, error) {
		client, err := clientFunc(ctx, opts...)
		return client, method, err
	}
}
