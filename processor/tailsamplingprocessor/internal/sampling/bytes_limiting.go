// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type bytesLimiting struct {
	// Rate limiter using golang.org/x/time/rate for efficient token bucket implementation
	limiter *rate.Limiter
}

var _ samplingpolicy.Evaluator = (*bytesLimiting)(nil)

// NewBytesLimiting creates a policy evaluator that samples traces based on byte limit per second using a token bucket algorithm.
// The bucket capacity defaults to 2x the bytes per second to allow for reasonable burst traffic.
func NewBytesLimiting(settings component.TelemetrySettings, bytesPerSecond int64) samplingpolicy.Evaluator {
	return NewBytesLimitingWithBurstCapacity(settings, bytesPerSecond, bytesPerSecond*2)
}

// NewBytesLimitingWithBurstCapacity creates a policy evaluator with custom burst capacity.
// Uses golang.org/x/time/rate.Limiter for efficient, thread-safe token bucket implementation.
func NewBytesLimitingWithBurstCapacity(_ component.TelemetrySettings, bytesPerSecond, burstCapacity int64) samplingpolicy.Evaluator {
	// Create rate limiter with specified rate and burst capacity
	// rate.Limit is tokens per second (bytes per second in our case)
	// burst capacity is the maximum number of tokens (bytes) that can be consumed in a single request
	limiter := rate.NewLimiter(rate.Limit(bytesPerSecond), int(burstCapacity))

	return &bytesLimiting{
		limiter: limiter,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision based on token bucket algorithm.
// Uses golang.org/x/time/rate.Limiter.AllowN() for efficient, thread-safe token consumption.
func (b *bytesLimiting) Evaluate(_ context.Context, _ pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	// Calculate the size of the trace in bytes
	traceSize := calculateTraceSize(trace)

	// Use AllowN to check if we can consume 'traceSize' tokens
	// AllowN returns true if the limiter allows the event and false otherwise
	// The limiter automatically handles token bucket refill and thread safety
	if b.limiter.AllowN(time.Now(), int(traceSize)) {
		return samplingpolicy.Sampled, nil
	}

	return samplingpolicy.NotSampled, nil
}

// calculateTraceSize calculates the accurate protobuf marshaled size of a trace in bytes
// using the OpenTelemetry Collector's built-in ProtoMarshaler.TracesSize() method
func calculateTraceSize(trace *samplingpolicy.TraceData) int64 {
	// Use the OpenTelemetry Collector's built-in method for accurate size calculation
	// This gives us the exact protobuf marshaled size
	marshaler := &ptrace.ProtoMarshaler{}
	return int64(marshaler.TracesSize(trace.ReceivedBatches))
}
