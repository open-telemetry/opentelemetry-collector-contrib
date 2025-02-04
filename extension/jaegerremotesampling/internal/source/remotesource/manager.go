// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package remotesource // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/source/remotesource"

import (
	"context"
	"fmt"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"google.golang.org/grpc"
)

// ConfigManagerProxy returns sampling decisions from collector over gRPC.
type ConfigManagerProxy struct {
	client api_v2.SamplingManagerClient
}

// NewConfigManager creates gRPC sampling manager.
func NewConfigManager(conn *grpc.ClientConn) *ConfigManagerProxy {
	return &ConfigManagerProxy{
		client: api_v2.NewSamplingManagerClient(conn),
	}
}

// GetSamplingStrategy returns sampling strategies from collector.
func (s *ConfigManagerProxy) GetSamplingStrategy(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error) {
	resp, err := s.client.GetSamplingStrategy(ctx, &api_v2.SamplingStrategyParameters{ServiceName: serviceName})
	if err != nil {
		return nil, fmt.Errorf("failed to get sampling strategy: %w", err)
	}
	return resp, nil
}
