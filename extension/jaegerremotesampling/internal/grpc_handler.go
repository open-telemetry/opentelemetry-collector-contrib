// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

// GRPCHandler is sampling strategy handler for gRPC.
type GRPCHandler struct {
	samplingProvider Provider
}

// NewGRPCHandler creates a handler that controls sampling strategies for services.
func NewGRPCHandler(provider Provider) GRPCHandler {
	return GRPCHandler{
		samplingProvider: provider,
	}
}

// GetSamplingStrategy returns sampling decision from store.
func (s GRPCHandler) GetSamplingStrategy(ctx context.Context, param *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {
	return s.samplingProvider.GetSamplingStrategy(ctx, param.GetServiceName())
}
