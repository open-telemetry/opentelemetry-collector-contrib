// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

const (
	// The constants help translate user friendly percentages to numbers direct used in sampling.
	numHashBuckets        = 0x4000 // Using a power of 2 to avoid division.
	bitMaskHashBuckets    = numHashBuckets - 1
	percentageScaleFactor = numHashBuckets / 100.0
)

type probabilisticSampler struct {
	logger             *zap.Logger
	scaledSamplingRate uint32
	hashSeed           uint32
}

var _ PolicyEvaluator = (*probabilisticSampler)(nil)

// NewProbabilisticSampler creates a policy evaluator that samples a percentage of
// traces.
func NewProbabilisticSampler(logger *zap.Logger, hashSeed uint32, samplingPercentage float32) PolicyEvaluator {
	if samplingPercentage < 0 {
		samplingPercentage = 0
	}

	return &probabilisticSampler{
		logger: logger,
		// Adjust sampling percentage on private so recalculations are avoided.
		scaledSamplingRate: uint32(samplingPercentage * percentageScaleFactor),
		hashSeed:           hashSeed,
	}
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (s *probabilisticSampler) OnLateArrivingSpans(Decision, []*pdata.Span) error {
	s.logger.Debug("Triggering action for late arriving spans in probabilistic filter")
	return nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (s *probabilisticSampler) Evaluate(traceID pdata.TraceID, _ *TraceData) (Decision, error) {
	s.logger.Debug("Evaluating spans in probabilistic filter")

	traceIDBytes := traceID.Bytes()
	if hash(traceIDBytes[:], s.hashSeed)&bitMaskHashBuckets < s.scaledSamplingRate {
		return Sampled, nil
	} else {
		return NotSampled, nil
	}
}

// hash is a murmur3 hash function, see http://en.wikipedia.org/wiki/MurmurHash
func hash(key []byte, seed uint32) (hash uint32) {
	const (
		c1 = 0xcc9e2d51
		c2 = 0x1b873593
		c3 = 0x85ebca6b
		c4 = 0xc2b2ae35
		r1 = 15
		r2 = 13
		m  = 5
		n  = 0xe6546b64
	)

	hash = seed
	iByte := 0
	for ; iByte+4 <= len(key); iByte += 4 {
		k := uint32(key[iByte]) | uint32(key[iByte+1])<<8 | uint32(key[iByte+2])<<16 | uint32(key[iByte+3])<<24
		k *= c1
		k = (k << r1) | (k >> (32 - r1))
		k *= c2
		hash ^= k
		hash = (hash << r2) | (hash >> (32 - r2))
		hash = hash*m + n
	}

	// TraceId and SpanId have lengths that are multiple of 4 so the code below is never expected to
	// be hit when sampling traces. However, it is preserved here to keep it as a correct murmur3 implementation.
	// This is enforced via tests.
	var remainingBytes uint32
	switch len(key) - iByte {
	case 3:
		remainingBytes += uint32(key[iByte+2]) << 16
		fallthrough
	case 2:
		remainingBytes += uint32(key[iByte+1]) << 8
		fallthrough
	case 1:
		remainingBytes += uint32(key[iByte])
		remainingBytes *= c1
		remainingBytes = (remainingBytes << r1) | (remainingBytes >> (32 - r1))
		remainingBytes *= c2
		hash ^= remainingBytes
	}

	hash ^= uint32(len(key))
	hash ^= hash >> 16
	hash *= c3
	hash ^= hash >> 13
	hash *= c4
	hash ^= hash >> 16

	return
}
