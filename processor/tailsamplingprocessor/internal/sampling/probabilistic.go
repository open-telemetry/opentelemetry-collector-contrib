// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"hash/fnv"
	"math"
	"math/big"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

const (
	defaultHashSalt = "default-hash-seed"
)

type probabilisticSampler struct {
	logger    *zap.Logger
	threshold uint64
	hashSalt  string
}

var _ PolicyEvaluator = (*probabilisticSampler)(nil)

// NewProbabilisticSampler creates a policy evaluator that samples a percentage of
// traces.
func NewProbabilisticSampler(settings component.TelemetrySettings, hashSalt string, samplingPercentage float64) PolicyEvaluator {
	if hashSalt == "" {
		hashSalt = defaultHashSalt
	}

	return &probabilisticSampler{
		logger: settings.Logger,
		// calculate threshold once
		threshold: calculateThreshold(samplingPercentage / 100),
		hashSalt:  hashSalt,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (s *probabilisticSampler) Evaluate(ctx context.Context, traceID pcommon.TraceID, _ *TraceData) (Decision, error) {
	s.logger.Debug("Evaluating spans in probabilistic filter")

	if hashTraceID(s.hashSalt, traceID[:]) <= s.threshold {
		return Sampled, nil
	}

	return NotSampled, nil
}

// calculateThreshold converts a ratio into a value between 0 and MaxUint64
func calculateThreshold(ratio float64) uint64 {
	// Use big.Float and big.Int to calculate threshold because directly convert
	// math.MaxUint64 to float64 will cause digits/bits to be cut off if the converted value
	// doesn't fit into bits that are used to store digits for float64 in Golang
	boundary := new(big.Float).SetInt(new(big.Int).SetUint64(math.MaxUint64))
	res, _ := boundary.Mul(boundary, big.NewFloat(ratio)).Uint64()
	return res
}

// hashTraceID creates a hash using the FNV-1a algorithm.
func hashTraceID(salt string, b []byte) uint64 {
	hasher := fnv.New64a()
	// the implementation fnv.Write() never returns an error, see hash/fnv/fnv.go
	_, _ = hasher.Write([]byte(salt))
	_, _ = hasher.Write(b)
	return hasher.Sum64()
}
