// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"context"

	grpcstore "github.com/jaegertracing/jaeger/cmd/agent/app/configmanager/grpc"
	"github.com/jaegertracing/jaeger/cmd/collector/app/sampling/strategystore"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type grpcRemoteStrategyStore struct {
	headerAdditions map[string]configopaque.String
	delegate        *grpcstore.ConfigManagerProxy
}

// NewRemoteStrategyStore returns a StrategyStore that delegates to the configured Jaeger gRPC endpoint, making
// extension-configured enhancements (header additions only for now) to the gRPC context of every outbound gRPC call.
// Note: it would be nice to expand the configuration surface to include an optional TTL-based caching behavior
// for service-specific outbound GetSamplingStrategy calls.
func NewRemoteStrategyStore(
	conn *grpc.ClientConn,
	grpcClientSettings *configgrpc.GRPCClientSettings,
) strategystore.StrategyStore {
	return &grpcRemoteStrategyStore{
		headerAdditions: grpcClientSettings.Headers,
		delegate:        grpcstore.NewConfigManager(conn),
	}
}

func (g *grpcRemoteStrategyStore) GetSamplingStrategy(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error) {
	return g.delegate.GetSamplingStrategy(g.enhanceContext(ctx), serviceName)
}

// This function is used to add the extension configuration defined HTTP headers to a given outbound gRPC call's context.
func (g *grpcRemoteStrategyStore) enhanceContext(ctx context.Context) context.Context {
	md := metadata.New(nil)
	for k, v := range g.headerAdditions {
		md.Set(k, string(v))
	}
	return metadata.NewOutgoingContext(ctx, md)
}
