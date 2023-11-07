// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

type mockCfgMgr struct {
	getSamplingStrategyFunc func(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error)
}

func (m *mockCfgMgr) GetSamplingStrategy(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error) {
	if m.getSamplingStrategyFunc != nil {
		return m.getSamplingStrategyFunc(ctx, serviceName)
	}
	return &api_v2.SamplingStrategyResponse{}, nil
}
