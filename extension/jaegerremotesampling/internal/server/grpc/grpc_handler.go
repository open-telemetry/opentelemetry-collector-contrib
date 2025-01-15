// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package grpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/server/grpc"

import (
	"context"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/source"
)

// GRPCHandler is sampling strategy handler for gRPC.
type GRPCHandler struct {
	samplingProvider source.Source
}

// NewGRPCHandler creates a handler that controls sampling strategies for services.
func NewGRPCHandler(provider source.Source) GRPCHandler {
	return GRPCHandler{
		samplingProvider: provider,
	}
}

// GetSamplingStrategy returns sampling decision from store.
func (s GRPCHandler) GetSamplingStrategy(ctx context.Context, param *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {
	return s.samplingProvider.GetSamplingStrategy(ctx, param.GetServiceName())
}
