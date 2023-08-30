// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"

	"github.com/jaegertracing/jaeger/thrift-gen/sampling"
)

type mockCfgMgr struct {
	getSamplingStrategyFunc func(ctx context.Context, serviceName string) (*sampling.SamplingStrategyResponse, error)
}

func (m *mockCfgMgr) GetSamplingStrategy(ctx context.Context, serviceName string) (*sampling.SamplingStrategyResponse, error) {
	if m.getSamplingStrategyFunc != nil {
		return m.getSamplingStrategyFunc(ctx, serviceName)
	}
	return sampling.NewSamplingStrategyResponse(), nil
}
