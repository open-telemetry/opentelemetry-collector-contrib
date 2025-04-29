// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package source // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/source"

import (
	"context"
	"io"

	"github.com/jaegertracing/jaeger-idl/proto-gen/api_v2"
)

// Source keeps track of service specific sampling strategies.
type Source interface {
	// Close() from io.Closer  stops the processor from calculating probabilities.
	io.Closer

	// GetSamplingStrategy retrieves the sampling strategy for the specified service.
	GetSamplingStrategy(ctx context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error)
}
