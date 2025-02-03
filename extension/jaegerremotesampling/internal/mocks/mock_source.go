// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mocks // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/mocks"

import (
	"context"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

type MockCfgMgr struct {
	GetSamplingStrategyFunc func(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error)
}

func (m *MockCfgMgr) Close() error {
	return nil
}

func (m *MockCfgMgr) GetSamplingStrategy(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error) {
	if m.GetSamplingStrategyFunc != nil {
		return m.GetSamplingStrategyFunc(ctx, serviceName)
	}
	return &api_v2.SamplingStrategyResponse{}, nil
}
