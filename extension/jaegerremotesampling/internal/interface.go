// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"io"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

// Provider keeps track of service specific sampling strategies.
type Provider interface {
	// Close() from io.Closer  stops the processor from calculating probabilities.
	io.Closer

	// GetSamplingStrategy retrieves the sampling strategy for the specified service.
	GetSamplingStrategy(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error)
}
