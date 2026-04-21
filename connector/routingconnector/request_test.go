// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/client"
	"google.golang.org/grpc/metadata"
)

func withGRPCMetadata(ctx context.Context, md map[string]string) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.New(md))
}

func withHTTPMetadata(ctx context.Context, md map[string][]string) context.Context {
	return client.NewContext(ctx, client.Info{Metadata: client.NewMetadata(md)})
}
